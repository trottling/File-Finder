package internal

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	iofs "io/fs"

	"github.com/mholt/archiver/v4"
	"github.com/panjf2000/ants/v2"
	"github.com/schollz/progressbar/v3"
	"github.com/sirupsen/logrus"
)

// FileScanner implements the Scanner interface for searching patterns in files and archives.
type FileScanner struct{}

// NewFileScanner creates a new FileScanner instance.
func NewFileScanner() *FileScanner {
	return &FileScanner{}
}

// Scan performs the search according to the options and calls onMatch for each match.
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

	bar := progressbar.NewOptions64(1,
		progressbar.OptionSetDescription("Scanning..."),
		progressbar.OptionShowDescriptionAtLineEnd(),
		progressbar.OptionFullWidth(),
		progressbar.OptionShowIts(),
		progressbar.OptionClearOnFinish(),
	)

	fileCh := make(chan task, 100)
	var wg sync.WaitGroup
	pool, err := ants.NewPoolWithFunc(opts.Threads, func(i interface{}) {
		defer wg.Done()
		if ctx.Err() != nil {
			return
		}
		t := i.(task)
		atomic.AddInt64(&processedFiles, 1)

		if t.isArchive {
			fs.handleArchiveFile(t.path, t.innerPath, patterns, opts.SaveFull, onMatch, &matchCount, &errorCount)
		} else {
			matchFileWithStats(t.path, patterns, opts.SaveFull, onMatch, &matchCount, &errorCount)
		}
		bar.Add(1)
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
			filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
				if ctx.Err() != nil {
					return ctx.Err()
				}
				if err != nil {
					atomic.AddInt64(&errorCount, 1)
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
					fs.handleArchive(ctx, path, func(t task) {
						select {
						case fileCh <- t:
						case <-ctx.Done():
						}
					}, &foundFiles, bar, opts)
					return nil
				}

				atomic.AddInt64(&foundFiles, 1)
				bar.ChangeMax64(atomic.LoadInt64(&foundFiles))
				select {
				case fileCh <- task{path: path}:
				case <-ctx.Done():
					return ctx.Err()
				}
				return nil
			})
		}
	}()

	progressDone := make(chan struct{})
	go func() {
		defer close(progressDone)
		ticker := time.NewTicker(200 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-walkDone:
				return
			case <-ctx.Done():
				return
			case <-ticker.C:
				processed := atomic.LoadInt64(&processedFiles)
				found := atomic.LoadInt64(&foundFiles)
				matches := atomic.LoadInt64(&matchCount)
				errors := atomic.LoadInt64(&errorCount)

				bar.Describe(fmt.Sprintf("Processed: %d | Found: %d | Matches: %d | Errors: %d", processed, found, matches, errors))

				if processed > found {
					found = processed
				}

				bar.ChangeMax64(found)
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
				logrus.WithError(err).Error("Failed to submit task to pool")
			}
		case <-ctx.Done():
			break loop
		}
	}

	wg.Wait()
	<-walkDone
	<-progressDone
	bar.Finish()

	return nil
}

// Pattern is an interface for pattern matching.
type Pattern interface {
	Match(string) bool
}

type RegexPattern struct{ re *regexp.Regexp }

func (p *RegexPattern) Match(s string) bool { return p.re.MatchString(s) }

type PlainPattern struct{ s string }

func (p *PlainPattern) Match(s string) bool { return strings.Contains(s, p.s) }

func loadPatterns(path string) ([]Pattern, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

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
		} else {
			patterns = append(patterns, &PlainPattern{line})
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading pattern file: %w", err)
	}
	logrus.Debugf("Loaded %d patterns", len(patterns))
	return patterns, nil
}

func matchReader(reader io.Reader, patterns []Pattern, saveFull bool, onMatch func(MatchResult), filePath, innerPath string, matchCount, errorCount *int64) {
	var fullContent []byte
	bufReader := bufio.NewReader(reader)
	if saveFull {
		if data, err := io.ReadAll(bufReader); err == nil {
			fullContent = data
			bufReader = bufio.NewReader(strings.NewReader(string(data)))
		} else {
			atomic.AddInt64(errorCount, 1)
			onMatch(MatchResult{FilePath: filePath, InnerPath: innerPath, Error: err})
			return
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
		for _, p := range patterns {
			if p.Match(line) {
				matched = true
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
				})
				break
			} else {
				onMatch(MatchResult{
					FilePath:   filePath,
					InnerPath:  innerPath,
					LineNumber: lineNum,
					Line:       line,
					Matched:    true,
				})
			}
		}
		if err == io.EOF {
			break
		}
		lineNum++
	}
}

func (fs *FileScanner) handleArchiveFile(archivePath, innerPath string, patterns []Pattern, saveFull bool, onMatch func(MatchResult), matchCount, errorCount *int64) {
	fsys, err := archiver.FileSystem(context.Background(), archivePath, nil)
	if err != nil {
		atomic.AddInt64(errorCount, 1)
		onMatch(MatchResult{FilePath: archivePath, InnerPath: innerPath, Error: err})
		return
	}
	defer func() {
		if closer, ok := fsys.(io.Closer); ok {
			closer.Close()
		}
	}()
	f, err := fsys.Open(innerPath)
	if err != nil {
		atomic.AddInt64(errorCount, 1)
		onMatch(MatchResult{FilePath: archivePath, InnerPath: innerPath, Error: err})
		return
	}
	defer f.Close()
	matchReader(f, patterns, saveFull, onMatch, archivePath, innerPath, matchCount, errorCount)
}

func matchFileWithStats(path string, patterns []Pattern, saveFull bool, onMatch func(MatchResult), matchCount, errorCount *int64) {
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
	defer f.Close()
	matchReader(f, patterns, saveFull, onMatch, path, "", matchCount, errorCount)
}

func (fs *FileScanner) handleArchive(ctx context.Context, path string, sendTask func(t task), foundFiles *int64, bar *progressbar.ProgressBar, opts ScanOptions) {
	fsys, err := archiver.FileSystem(ctx, path, nil)
	if err != nil {
		logrus.WithFields(logrus.Fields{"archive": path, "error": err}).Error("Failed to open archive")
		return
	}
	defer func() {
		if closer, ok := fsys.(io.Closer); ok {
			closer.Close()
		}
	}()

	iofs.WalkDir(fsys, ".", func(innerPath string, d iofs.DirEntry, err error) error {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err != nil || d.IsDir() {
			return nil
		}

		ext := strings.ToLower(filepath.Ext(innerPath))
		if len(opts.Whitelist) > 0 && !containsExt(opts.Whitelist, ext) {
			return nil
		}
		if len(opts.Blacklist) > 0 && containsExt(opts.Blacklist, ext) {
			return nil
		}

		atomic.AddInt64(foundFiles, 1)
		bar.ChangeMax64(atomic.LoadInt64(foundFiles))
		sendTask(task{path: path, innerPath: innerPath, isArchive: true})
		return nil
	})
}

func containsExt(list []string, ext string) bool {
	for _, e := range list {
		if strings.ToLower(e) == ext {
			return true
		}
	}
	return false
}

func isArchive(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	archiveExts := map[string]bool{
		".zip": true,
		".tar": true,
		".gz":  true,
		".bz2": true,
		".xz":  true,
		".rar": true,
	}
	return archiveExts[ext]
}
