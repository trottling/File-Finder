package main

import (
	"FileFinder/internal"
	"context"
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
)

// InitLogger initializes the logger with optional file output.
func InitLogger(logfile string) {
	logrus.SetFormatter(&logrus.TextFormatter{
		ForceColors:   true,
		FullTimestamp: true,
		DisableQuote:  true,
		PadLevelText:  true,
	})
	if logfile != "" {
		file, err := os.OpenFile(logfile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err == nil {
			logrus.SetOutput(file)
		} else {
			logrus.Warn("Failed to open log file, logging to stdout")
		}
	}
}

func main() {
	app := &cli.App{
		Name:  "FileFinder",
		Usage: "Search for matches in files across all disks",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "pattern-file",
				Usage:    "Path to the file with search patterns (required)",
				Required: true,
			},
			&cli.StringSliceFlag{
				Name:  "whitelist",
				Usage: "Whitelist of file extensions (comma separated, e.g. txt,log)",
			},
			&cli.StringSliceFlag{
				Name:  "blacklist",
				Usage: "Blacklist of file extensions (comma separated)",
			},
			&cli.StringFlag{
				Name:  "logfile",
				Usage: "Path to the log file",
			},
			&cli.IntFlag{
				Name:  "threads",
				Usage: "Number of threads",
				Value: 100,
			},
			&cli.BoolFlag{
				Name:  "save-full",
				Usage: "Save the entire file with a match, not just the matching lines (use --save-full-folder to specify folder)",
			},
			&cli.StringFlag{
				Name:  "save-full-folder",
				Usage: "Folder to save found files (default: /found_files)",
				Value: "/found_files",
			},
			&cli.BoolFlag{
				Name:  "archives",
				Usage: "Process archives (zip, tar, gz, bz2, xz, rar)",
			},
			&cli.IntFlag{
				Name:  "depth",
				Usage: "Search depth (0 - unlimited)",
				Value: 0,
			},
			&cli.DurationFlag{
				Name:  "timeout",
				Usage: "Timeout for the scan (e.g. 10m, 1h)",
			},
			&cli.BoolFlag{
				Name:  "fail-fast",
				Usage: "Stop on first error (fail-fast mode)",
			},
		},
		Action: func(c *cli.Context) error {
			start := time.Now()
			InitLogger(c.String("logfile"))
			logrus.Info("FileFinder started")

			// Setup context with cancel and signal handling
			timeout := c.Duration("timeout")
			ctx := context.Background()
			var cancel context.CancelFunc
			if timeout > 0 {
				ctx, cancel = context.WithTimeout(ctx, timeout)
			} else {
				ctx, cancel = context.WithCancel(ctx)
			}
			sigCtx, stop := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
			defer stop()
			defer cancel()

			roots := c.Args().Slice()
			var validRoots []string
			if len(roots) == 0 {
				// Autodetect roots for each OS
				validRoots = detectRoots()
				logrus.Infof("No search paths provided, using all available roots: %v", validRoots)
			} else {
				for _, r := range roots {
					if _, err := os.Stat(r); err == nil {
						validRoots = append(validRoots, r)
					} else {
						logrus.Warnf("Path does not exist or is not accessible: %s", r)
					}
				}
				if len(validRoots) == 0 {
					logrus.Error("No valid search paths provided. Exiting.")
					return nil
				}
			}

			// Bring extensions to unified format (.ext)
			whitelist := normalizeExtSlice(c.StringSlice("whitelist"))
			blacklist := normalizeExtSlice(c.StringSlice("blacklist"))

			opts := internal.ScanOptions{
				Roots:          validRoots,
				Whitelist:      whitelist,
				Blacklist:      blacklist,
				Depth:          c.Int("depth"),
				Archives:       c.Bool("archives"),
				PatternFile:    c.String("pattern-file"),
				Threads:        c.Int("threads"),
				SaveFull:       c.Bool("save-full"),
				SaveFullFolder: c.String("save-full-folder"),
				FailFast:       c.Bool("fail-fast"),
			}

			finder := internal.NewFileScanner()
			var (
				matchCount int64
				errorCount int64
				fileCount  int64
			)
			err := finder.Scan(sigCtx, opts, func(res internal.MatchResult) {
				if res.Error != nil {
					errorCount++
					logrus.WithFields(logrus.Fields{"file": res.FilePath, "err": res.Error}).Error("Error while processing file")
					if opts.FailFast {
						cancel()
					}
					return
				}
				if res.Matched {
					matchCount++
					logrus.WithFields(logrus.Fields{"file": res.FilePath, "line": res.LineNumber}).Info("Match found")
				}
				if res.FilePath != "" && !res.Matched {
					fileCount++
				}
			})
			if err != nil {
				logrus.WithError(err).Fatal("Scan failed")
			}

			// Total info
			fmt.Printf("\n======= Scan finished in %s =======\n", time.Since(start))
			fmt.Printf("Total files scanned: %d\n", fileCount)
			fmt.Printf("Total matches found: %d\n", matchCount)
			fmt.Printf("Errors: %d\n", errorCount)

			return nil
		},
	}

	if err := app.Run(os.Args); err != nil {
		logrus.Fatal(err)
	}
}

// normalizeExtSlice normalizes file extension slices to ".ext" format.
func normalizeExtSlice(s []string) []string {
	out := make([]string, 0, len(s))
	for _, ext := range s {
		for _, val := range strings.Split(ext, ",") {
			val = strings.TrimSpace(val)
			if val == "" {
				continue
			}
			val = strings.TrimPrefix(val, ".")
			val = "." + strings.ToLower(val)
			out = append(out, val)
		}
	}
	return out
}

// detectRoots detects all root directories depending on the OS.
func detectRoots() []string {
	osType := runtime.GOOS
	if osType == "windows" {
		var drives []string
		for c := 'C'; c <= 'Z'; c++ {
			path := string(c) + ":\\"
			if _, err := os.Stat(path); err == nil {
				drives = append(drives, path)
			}
		}
		return drives
	}

	// On Linux/macOS: search root is /
	roots := []string{"/"}
	// Check default mount points
	folders := []string{"/mnt", "/media", "/run/media", "/Volumes"} // Last — for macOS

	for _, mount := range folders {
		if info, err := os.Stat(mount); err == nil && info.IsDir() {
			entries, err := os.ReadDir(mount)
			if err == nil {
				for _, entry := range entries {
					// Add all found folder
					roots = append(roots, mount+"/"+entry.Name())
				}
			}
		}
	}
	return roots
}
