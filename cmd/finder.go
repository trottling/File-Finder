package main

import (
	"FileFinder/internal"
	"context"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
)

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
				Usage: "Whitelist of file extensions (comma separated)",
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
				Usage: "Save the entire file with a match, not just the matching lines",
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
		},
		Action: func(c *cli.Context) error {
			start := time.Now()
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

			opts := internal.ScanOptions{
				Roots:       validRoots,
				Whitelist:   c.StringSlice("whitelist"),
				Blacklist:   c.StringSlice("blacklist"),
				Depth:       c.Int("depth"),
				Archives:    c.Bool("archives"),
				PatternFile: c.String("pattern-file"),
				Threads:     c.Int("threads"),
				SaveFull:    c.Bool("save-full"),
			}

			finder := internal.NewFileScanner()
			err := finder.Scan(sigCtx, opts, func(res internal.MatchResult) {
				if res.Error != nil {
					logrus.WithFields(logrus.Fields{"file": res.FilePath, "err": res.Error}).Error("Error while processing file")
					return
				}
				if res.Matched {
					logrus.WithFields(logrus.Fields{"file": res.FilePath, "line": res.LineNumber}).Info("Match found")
				}
			})
			if err != nil {
				logrus.WithError(err).Fatal("Scan failed")
			}

			logrus.Infof("FileFinder finished in %s", time.Since(start))
			return nil
		},
	}

	// Logger setup
	logrus.SetFormatter(&logrus.TextFormatter{
		ForceColors:   true,
		FullTimestamp: true,
		DisableQuote:  true,
		PadLevelText:  true,
	})

	if logfile := os.Getenv("LOGFILE"); logfile != "" {
		file, err := os.OpenFile(logfile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err == nil {
			logrus.SetOutput(file)
		} else {
			logrus.Warn("Failed to open log file, logging to stdout")
		}
	}

	if err := app.Run(os.Args); err != nil {
		logrus.Fatal(err)
	}
}

func detectRoots() []string {
	osType := runtime.GOOS
	if osType == "windows" {
		// On Windows, scan all available drives
		var drives []string
		for c := 'C'; c <= 'Z'; c++ {
			path := string(c) + ":\\"
			if _, err := os.Stat(path); err == nil {
				drives = append(drives, path)
			}
		}
		return drives
	}
	// On Unix-like, scan from root
	return []string{"/"}
}
