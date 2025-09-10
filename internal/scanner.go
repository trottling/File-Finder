package internal

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/mholt/archives"
	"github.com/panjf2000/ants/v2"
	"github.com/sirupsen/logrus"
)

var ErrWalkFailFast = errors.New("fail-fast: walk error") // sentinel error

// FileScanner implements file + archive scanning.
type FileScanner struct{}

func NewFileScanner() *FileScanner { return &FileScanner{} }

// MatchResult is reported to a callback.
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

// NewResultSink returns a closure writing matches/errs counters + file sinks.
func NewResultSink(opts ScanOptions, stats *AppStats) func(MatchResult) {
	stats.Start()
	var matchesFileMu sync.Mutex
	var patternFilesMu sync.Map

	return func(res MatchResult) {
		if res.Error != nil {
			stats.Errors.Add(1)
			logrus.WithFields(logrus.Fields{"file": res.FilePath, "inner": res.InnerPath, "err": res.Error}).Error("process error")
			return
		}
		if !res.Matched {
			return
		}
		// log basic info
		if res.Line != "" {
			logrus.WithFields(logrus.Fields{"file": res.FilePath, "line": res.LineNumber}).Info("Match found")
		} else {
			logrus.WithFields(logrus.Fields{"file": res.FilePath, "inner": res.InnerPath}).Info("Match found (full file)")
		}
		stats.Matches.Add(1)

		// single sink file
		if opts.SaveMatchesFile != "" && res.Line != "" {
			matchesFileMu.Lock()
			if f, err := os.OpenFile(opts.SaveMatchesFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644); err == nil {
				if !strings.HasSuffix(res.Line, "\n") {
					_, _ = io.WriteString(f, res.Line+"\n")
				} else {
					_, _ = io.WriteString(f, res.Line)
				}
				_ = f.Close()
			}
			matchesFileMu.Unlock()
		}

		// per-pattern files
		if opts.SaveMatchesByPatternFolder != "" && res.Line != "" && res.Pattern != "" {
			_ = os.MkdirAll(opts.SaveMatchesByPatternFolder, 0755)
			name := Sanitize(res.Pattern) + ".txt"
			path := filepath.Join(opts.SaveMatchesByPatternFolder, name)
			muAny, _ := patternFilesMu.LoadOrStore(path, &sync.Mutex{})
			mu := muAny.(*sync.Mutex)
			mu.Lock()
			if f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644); err == nil {
				if !strings.HasSuffix(res.Line, "\n") {
					_, _ = io.WriteString(f, res.Line+"\n")
				} else {
					_, _ = io.WriteString(f, res.Line)
				}
				_ = f.Close()
			}
			mu.Unlock()
		}
	}
}

// Scan is the main pipeline.
func (fs *FileScanner) Scan(ctx context.Context, opts ScanOptions, onMatch func(MatchResult)) error {
	patterns, hasInsensitive, err := LoadPatterns(opts.PatternFile)
	if err != nil {
		return err
	}

	var (
		found     atomic.Int64
		processed atomic.Int64
		errorsC   atomic.Int64
		matches   atomic.Int64
	)

	fileCh := make(chan Task, 2048)
	var wg sync.WaitGroup

	pool, err := ants.NewPoolWithFunc(opts.Threads, func(i interface{}) {
		defer wg.Done()
		if ctx.Err() != nil {
			return
		}
		t := i.(Task)
		processed.Add(1)
		if t.isArchive {
			fs.scanArchiveFile(t.path, t.innerPath, patterns, hasInsensitive, opts, onMatch, &matches, &errorsC)
		} else {
			fs.scanRegularFile(t.path, patterns, hasInsensitive, opts, onMatch, &matches, &errorsC)
		}
	})
	if err != nil {
		return fmt.Errorf("pool: %w", err)
	}
	defer pool.Release()

	// walker
	walkErr := make(chan error, 1)
	go func() {
		defer close(walkErr)
		for _, root := range opts.Roots {
			if ctx.Err() != nil {
				return
			}
			WalkWithDepth(ctx, root, opts.Depth, func(path string, d os.DirEntry, err error) error {
				if ctx.Err() != nil {
					return ctx.Err()
				}
				if err != nil {
					errorsC.Add(1)
					onMatch(MatchResult{FilePath: path, Error: err})
					if opts.FailFast {
						return ErrWalkFailFast
					}
					return nil
				}
				if d.IsDir() {
					return nil
				}
				ext := strings.ToLower(filepath.Ext(d.Name()))
				if !opts.allowedExt(ext) {
					return nil
				}
				if opts.Archives && IsArchive(path) {
					WalkArchive(ctx, path, func(t Task) {
						select {
						case fileCh <- t:
							found.Add(1)
						case <-ctx.Done():
						}
					}, &found, opts)
					return nil
				}
				found.Add(1)
				select {
				case fileCh <- Task{path: path}:
				case <-ctx.Done():
					return ctx.Err()
				}
				return nil
			})
		}
	}()

	// periodic stats
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	closing := false
	for !closing {
		select {
		case t, ok := <-fileCh:
			if !ok {
				closing = true
				break
			}
			wg.Add(1)
			if err := pool.Invoke(t); err != nil {
				wg.Done()
				logrus.WithError(err).Error("submit task")
				if opts.FailFast {
					return err
				}
			}
		case <-ticker.C:
			logrus.Infof("Stats: found=%d processed=%d matches=%d errors=%d",
				found.Load(), processed.Load(), matches.Load(), errorsC.Load())
		case <-ctx.Done():
			return ctx.Err()
		case err := <-walkErr:
			if err != nil {
				// close channel to stop workers before returning
				close(fileCh)
				wg.Wait()
				if errors.Is(err, ErrWalkFailFast) {
					return ErrWalkFailFast
				}
				return err
			}
			// walker done - close input
			close(fileCh)
		}
	}

	wg.Wait()
	return nil
}

func (fs *FileScanner) scanRegularFile(
	path string,
	patterns []Pattern,
	hasInsensitive bool,
	opts ScanOptions,
	onMatch func(MatchResult),
	matchCnt, errCnt *atomic.Int64,
) {
	if IsArchive(path) {
		return
	}
	f, err := os.Open(path)
	if err != nil {
		errCnt.Add(1)
		onMatch(MatchResult{FilePath: path, Error: err})
		return
	}
	defer f.Close()

	matchReader(f, patterns, hasInsensitive, opts.SaveFull, opts.SaveFullFolder, onMatch, path, "", matchCnt, errCnt)
}

func (fs *FileScanner) scanArchiveFile(
	archivePath, innerPath string,
	patterns []Pattern,
	hasInsensitive bool,
	opts ScanOptions,
	onMatch func(MatchResult),
	matchCnt, errCnt *atomic.Int64,
) {
	fsys, err := archives.FileSystem(context.Background(), archivePath, nil)
	if err != nil {
		errCnt.Add(1)
		onMatch(MatchResult{FilePath: archivePath, InnerPath: innerPath, Error: err})
		return
	}
	if closer, ok := fsys.(io.Closer); ok {
		defer closer.Close()
	}
	f, err := fsys.Open(innerPath)
	if err != nil {
		errCnt.Add(1)
		onMatch(MatchResult{FilePath: archivePath, InnerPath: innerPath, Error: err})
		return
	}
	defer f.Close()

	matchReader(f, patterns, hasInsensitive, opts.SaveFull, opts.SaveFullFolder, onMatch, archivePath, innerPath, matchCnt, errCnt)
}
