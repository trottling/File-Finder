package internal

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	iofs "io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/mholt/archives"
	"github.com/panjf2000/ants/v2"
	"github.com/sirupsen/logrus"
)

const (
	maxArchiveFiles = 10000 // zip-bomb protect
)

// FileScanner implements the Scanner interface for searching patterns in files and archives.
type FileScanner struct{}

// NewFileScanner creates a new FileScanner instance.
func NewFileScanner() *FileScanner {
	return &FileScanner{}
}

// ScanOptions contains options for the scanning process.
type ScanOptions struct {
	Roots                      []string
	PatternFile                string
	Threads                    int
	Whitelist                  []string
	Blacklist                  []string
	Depth                      int
	Archives                   bool
	SaveFull                   bool
	SaveFullFolder             string
	FailFast                   bool
	SaveMatchesFile            string
	SaveMatchesByPatternFolder string
}

// MatchResult represents a single match found during scanning.
type MatchResult struct {
	FilePath   string
	InnerPath  string
	LineNumber int
	Line       string
	FullFile   []byte
	Matched    bool
	Error      error
	Pattern    string
}

type Task struct {
	path      string
	innerPath string
	isArchive bool
}

// Pattern is an interface for pattern matching.
type Pattern interface {
	Match(string) bool
}

type RegexPattern struct{ re *regexp.Regexp }

func (p *RegexPattern) Match(s string) bool { return p.re.MatchString(s) }

type PlainPattern struct {
	s           string
	insensitive bool
}

func (p *PlainPattern) Match(s string) bool {
	if p.insensitive {
		return strings.Contains(strings.ToLower(s), strings.ToLower(p.s))
	}
	return strings.Contains(s, p.s)
}

// loadPatterns loads patterns from file, supports regex and plain patterns.
func loadPatterns(path string) ([]Pattern, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := f.Close(); err != nil {
			logrus.WithFields(logrus.Fields{"file": path, "err": err}).Warn("Failed to close pattern file")
		}
	}()

	var patterns []Pattern
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "re:") {
			re, err := regexp.Compile(line[3:])
			if err != nil {
				return nil, fmt.Errorf("invalid regex pattern %q: %w", line, err)
			}
			patterns = append(patterns, &RegexPattern{re})
		} else if strings.HasPrefix(line, "plain:i:") {
			patterns = append(patterns, &PlainPattern{line[8:], true})
		} else {
			patterns = append(patterns, &PlainPattern{line, false})
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading pattern file: %w", err)
	}
	logrus.Debugf("Loaded %d patterns", len(patterns))
	return patterns, nil
}

// containsExt checks if the extension is in the list.
func containsExt(list []string, ext string) bool {
	for _, e := range list {
		if e == ext {
			return true
		}
	}
	return false
}

// isArchive checks if the file is an archive by extension.
func isArchive(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	archiveExts := map[string]bool{
		".zip": true, ".tar": true, ".gz": true,
		".bz2": true, ".xz": true, ".rar": true,
		".br": true, ".lz4": true, ".lz": true,
		".mz": true, ".sz": true, ".s2": true,
		".zz": true, ".zst": true, ".7z": true}
	return archiveExts[ext]
}

// walkWithDepth walks the directory tree up to maxDepth.
func walkWithDepth(ctx context.Context, root string, maxDepth int, fileFunc func(path string, info os.FileInfo, err error) error) error {
	return filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if maxDepth > 0 {
			rel, _ := filepath.Rel(root, path)
			if len(strings.Split(rel, string(os.PathSeparator))) > maxDepth {
				return filepath.SkipDir
			}
		}
		return fileFunc(path, info, err)
	})
}

// Scan scans files and archives for patterns.
func (fs *FileScanner) Scan(ctx context.Context, opts ScanOptions, onMatch func(MatchResult)) error {
	patterns, err := loadPatterns(opts.PatternFile)
	if err != nil {
		return err
	}

	var (
		foundFiles     int64
		processedFiles int64
		matchCount     int64
		errorCount     int64
	)

	fileCh := make(chan Task, 1000)
	var wg sync.WaitGroup
	pool, err := ants.NewPoolWithFunc(opts.Threads, func(i interface{}) {
		defer wg.Done()
		if ctx.Err() != nil {
			return
		}
		t := i.(Task)
		atomic.AddInt64(&processedFiles, 1)

		if t.isArchive {
			fs.handleArchiveFile(t.path, t.innerPath, patterns, opts.SaveFull, opts.SaveFullFolder, onMatch, &matchCount, &errorCount)
		} else {
			matchFileWithStats(t.path, patterns, opts.SaveFull, opts.SaveFullFolder, onMatch, &matchCount, &errorCount)
		}
	})

	if err != nil {
		return fmt.Errorf("failed to create worker pool: %w", err)
	}
	defer pool.Release()

	walkDone := make(chan struct{})
	go func() {
		defer close(walkDone)
		for _, root := range opts.Roots {
			if ctx.Err() != nil {
				return
			}
			walkWithDepth(ctx, root, opts.Depth, func(path string, info os.FileInfo, err error) error {
				if ctx.Err() != nil {
					return ctx.Err()
				}
				if err != nil {
					atomic.AddInt64(&errorCount, 1)
					onMatch(MatchResult{FilePath: path, Error: err})
					if opts.FailFast {
						return errors.New("fail-fast: walk error")
					}
					return nil
				}

				if info.IsDir() {
					return nil
				}

				ext := strings.ToLower(filepath.Ext(info.Name()))
				if len(opts.Whitelist) > 0 && !containsExt(opts.Whitelist, ext) {
					return nil
				}
				if len(opts.Blacklist) > 0 && containsExt(opts.Blacklist, ext) {
					return nil
				}

				if opts.Archives && isArchive(path) {
					fs.handleArchive(ctx, path, func(t Task) {
						select {
						case fileCh <- t:
						case <-ctx.Done():
						}
					}, &foundFiles, opts)
					return nil
				}

				atomic.AddInt64(&foundFiles, 1)
				select {
				case fileCh <- Task{path: path}:
				case <-ctx.Done():
					return ctx.Err()
				}
				return nil
			})
		}
	}()

	// Periodic stats logging
	statDone := make(chan struct{})
	go func() {
		defer close(statDone)
		for {
			select {
			case <-ctx.Done():
				return
			case <-time.After(2 * time.Second):
				logrus.Infof("Stats: found=%d, processed=%d, matches=%d, errors=%d", atomic.LoadInt64(&foundFiles), atomic.LoadInt64(&processedFiles), atomic.LoadInt64(&matchCount), atomic.LoadInt64(&errorCount))
			}
		}
	}()

loop:
	for {
		select {
		case t, ok := <-fileCh:
			if !ok {
				break loop
			}
			wg.Add(1)
			if err := pool.Invoke(t); err != nil {
				wg.Done()
				logrus.WithError(err).Error("Failed to submit Task to pool")
				if opts.FailFast {
					return err
				}
			}
		case <-ctx.Done():
			break loop
		}
	}

	wg.Wait()
	<-walkDone
	// Wait for stat logger to finish
	<-statDone

	return nil
}

// handleArchive processes archive files and sends tasks for inner files.
func (fs *FileScanner) handleArchive(ctx context.Context, path string, sendTask func(t Task), foundFiles *int64, opts ScanOptions) {
	fsys, err := archives.FileSystem(ctx, path, nil)
	if err != nil {
		logrus.WithFields(logrus.Fields{"archive": path, "error": err}).Error("Failed to open archive")
		return
	}
	defer func() {
		if closer, ok := fsys.(io.Closer); ok {
			if err := closer.Close(); err != nil {
				logrus.WithFields(logrus.Fields{"archive": path, "err": err}).Warn("Failed to close archive fsys")
			}
		}
	}()

	count := 0
	iofs.WalkDir(fsys, ".", func(innerPath string, d iofs.DirEntry, err error) error {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err != nil || d.IsDir() {
			return nil
		}
		if count > maxArchiveFiles {
			logrus.Warnf("Archive %s skipped: too many files", path)
			return errors.New("archive file limit reached")
		}
		ext := strings.ToLower(filepath.Ext(innerPath))
		if len(opts.Whitelist) > 0 && !containsExt(opts.Whitelist, ext) {
			return nil
		}
		if len(opts.Blacklist) > 0 && containsExt(opts.Blacklist, ext) {
			return nil
		}

		atomic.AddInt64(foundFiles, 1)
		sendTask(Task{path: path, innerPath: innerPath, isArchive: true})
		count++
		return nil
	})
}

// handleArchiveFile scans a file inside an archive for patterns.
func (fs *FileScanner) handleArchiveFile(archivePath, innerPath string, patterns []Pattern, saveFull bool, saveFullFolder string, onMatch func(MatchResult), matchCount, errorCount *int64) {
	fsys, err := archives.FileSystem(context.Background(), archivePath, nil)
	if err != nil {
		atomic.AddInt64(errorCount, 1)
		onMatch(MatchResult{FilePath: archivePath, InnerPath: innerPath, Error: err})
		return
	}
	defer func() {
		if closer, ok := fsys.(io.Closer); ok {
			if err := closer.Close(); err != nil {
				logrus.WithFields(logrus.Fields{"file": innerPath, "err": err}).Warn("Failed to close archive inner file")
			}
		}
	}()
	f, err := fsys.Open(innerPath)
	if err != nil {
		atomic.AddInt64(errorCount, 1)
		onMatch(MatchResult{FilePath: archivePath, InnerPath: innerPath, Error: err})
		return
	}
	defer func() {
		if err := f.Close(); err != nil {
			logrus.WithFields(logrus.Fields{"file": innerPath, "err": err}).Warn("Failed to close archive inner file")
		}
	}()
	matchReader(f, patterns, saveFull, onMatch, archivePath, innerPath, matchCount, errorCount, saveFullFolder)
}

// matchFileWithStats scans a regular file for patterns and collects stats.
func matchFileWithStats(path string, patterns []Pattern, saveFull bool, saveFullFolder string, onMatch func(MatchResult), matchCount, errorCount *int64) {
	if isArchive(path) {
		return
	}
	f, err := os.Open(path)
	if err != nil {
		atomic.AddInt64(errorCount, 1)
		onMatch(MatchResult{FilePath: path, Error: err})
		logrus.WithFields(logrus.Fields{"file": path, "err": err}).Error("Error opening file")
		return
	}
	defer func() {
		if err := f.Close(); err != nil {
			logrus.WithFields(logrus.Fields{"file": path, "err": err}).Warn("Failed to close file after scan")
		}
	}()
	matchReader(f, patterns, saveFull, onMatch, path, "", matchCount, errorCount, saveFullFolder)
}

// matchReader reads lines from a file and matches them against patterns.
func matchReader(reader io.Reader, patterns []Pattern, saveFull bool, onMatch func(MatchResult), filePath, innerPath string, matchCount, errorCount *int64, saveFullFolder string) {
	var fullContent []byte
	bufReader := bufio.NewReader(reader)
	shouldCopy := saveFull && saveFullFolder != ""
	var copySrc io.Reader
	if saveFull {
		if shouldCopy {
			pr, pw := io.Pipe()
			tee := io.TeeReader(bufReader, pw)
			copySrc = pr
			go func() {
				defer pw.Close()
				if _, err := io.Copy(io.Discard, tee); err != nil {
					logrus.WithFields(logrus.Fields{"file": filePath, "err": err}).Warn("Failed to discard tee copy")
				}
			}()
			bufReader = bufio.NewReader(pr)
		} else {
			if data, err := io.ReadAll(bufReader); err == nil {
				fullContent = data
				bufReader = bufio.NewReader(strings.NewReader(string(data)))
			} else {
				atomic.AddInt64(errorCount, 1)
				onMatch(MatchResult{FilePath: filePath, InnerPath: innerPath, Error: err})
				logrus.WithFields(logrus.Fields{"file": filePath, "err": err}).Error("Failed to read file content for saveFull")
				return
			}
		}
	}
	lineNum := 0
	for {
		line, err := bufReader.ReadString('\n')
		if err != nil && err != io.EOF {
			atomic.AddInt64(errorCount, 1)
			onMatch(MatchResult{FilePath: filePath, InnerPath: innerPath, Error: err})
			return
		}
		matched := false
		var matchedPattern string
		for _, p := range patterns {
			if p.Match(line) {
				matched = true
				switch pt := p.(type) {
				case *RegexPattern:
					matchedPattern = pt.re.String()
				case *PlainPattern:
					matchedPattern = pt.s
				default:
					matchedPattern = "unknown"
				}
				break
			}
		}
		if matched {
			atomic.AddInt64(matchCount, 1)
			if saveFull {
				onMatch(MatchResult{
					FilePath:  filePath,
					InnerPath: innerPath,
					FullFile:  fullContent,
					Matched:   true,
					Pattern:   matchedPattern,
				})
				if shouldCopy {
					folder := saveFullFolder
					if folder == "" {
						folder = "/found_files"
					}
					if err := os.MkdirAll(folder, 0755); err != nil {
						logrus.WithFields(logrus.Fields{"folder": folder, "err": err}).Error("Failed to create folder for saving file")
						break
					}
					var outPath string
					if innerPath != "" {
						outPath = filepath.Join(folder, strings.ReplaceAll(filePath, string(os.PathSeparator), "_"), strings.ReplaceAll(innerPath, string(os.PathSeparator), "_"))
					} else {
						outPath = filepath.Join(folder, strings.ReplaceAll(filePath, string(os.PathSeparator), "_"))
					}
					out, err := os.Create(outPath)
					if err != nil {
						logrus.WithFields(logrus.Fields{"file": outPath, "err": err}).Error("Failed to create file for saving match")
						break
					}
					if _, err := io.Copy(out, copySrc); err != nil {
						logrus.WithFields(logrus.Fields{"file": outPath, "err": err}).Error("Failed to copy file content to saved file")
					}
					if err := out.Close(); err != nil {
						logrus.WithFields(logrus.Fields{"file": outPath, "err": err}).Warn("Failed to close saved file")
					}
				}
				break
			} else {
				onMatch(MatchResult{
					FilePath:   filePath,
					InnerPath:  innerPath,
					LineNumber: lineNum,
					Line:       line,
					Matched:    true,
					Pattern:    matchedPattern,
				})
			}
		}
		if err == io.EOF {
			break
		}
		lineNum++
	}
}
