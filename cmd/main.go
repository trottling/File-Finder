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

	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
)

func main() {
	app := &cli.App{
		Name:  "FileFinder",
		Usage: "Search patterns in files and archives across disks",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "pattern-file",
				Usage:    "Path to text file with patterns: plain lines, 'plain:i:' for case-insensitive, or 're:<regex>'",
				Required: true,
			},
			&cli.StringSliceFlag{
				Name:  "whitelist",
				Usage: "Only scan these extensions (comma separated, e.g. txt,log,json). Use without dot.",
			},
			&cli.StringSliceFlag{
				Name:  "blacklist",
				Usage: "Skip these extensions (comma separated). If whitelist is set, blacklist is ignored.",
			},
			&cli.StringFlag{
				Name:  "logfile",
				Usage: "Write logs into file instead of stdout",
			},
			&cli.IntFlag{
				Name:  "threads",
				Usage: "Max concurrent file workers (default scales with CPU)",
				Value: 0,
			},
			&cli.BoolFlag{
				Name:  "save-full",
				Usage: "On first match, save the whole file (not only the matching line)",
			},
			&cli.StringFlag{
				Name:  "save-full-folder",
				Usage: "Destination folder for saved matched files (required with --save-full)",
				Value: "/found_files",
			},
			&cli.BoolFlag{
				Name:  "archives",
				Usage: "Also scan archives (.zip,.tar,.gz,.bz2,.xz,.rar,.7z,...)",
			},
			&cli.IntFlag{
				Name:  "depth",
				Usage: "Max directory depth (0 - unlimited)",
				Value: 0,
			},
			&cli.DurationFlag{
				Name:  "timeout",
				Usage: "Global timeout for scan (e.g. 10m, 1h)",
			},
			&cli.BoolFlag{
				Name:  "fail-fast",
				Usage: "Stop immediately on any error",
			},
			&cli.StringFlag{
				Name:  "save-matches-file",
				Usage: "Append all matched lines into a single file",
			},
			&cli.StringFlag{
				Name:  "save-matches-folder",
				Usage: "Create per-pattern files with matched lines inside this folder",
			},
			&cli.StringFlag{
				Name:  "log-level",
				Usage: "Log level: debug, info, warn, error",
				Value: "info",
			},
		},
		Action: func(c *cli.Context) error {
			internal.InitLogger(c.String("logfile"), c.String("log-level"))
			logrus.Info("FileFinder started")

			// ctx with timeout + OS signals
			base := context.Background()

			var cancel context.CancelFunc
			if t := c.Duration("timeout"); t > 0 {
				base, cancel = context.WithTimeout(base, t)
			} else {
				base, cancel = context.WithCancel(base)
			}
			defer cancel()

			ctx, stop := signal.NotifyContext(base, syscall.SIGINT, syscall.SIGTERM)
			defer stop()

			// roots
			roots := c.Args().Slice()
			var validRoots []string
			if len(roots) == 0 {
				validRoots = internal.DetectRoots(runtime.GOOS)
				logrus.Infof("No search paths provided, using auto roots: %v", validRoots)
			} else {
				for _, r := range roots {
					if st, err := os.Stat(r); err == nil && st.IsDir() {
						validRoots = append(validRoots, r)
					} else {
						logrus.Warnf("Skip: not a dir or inaccessible: %s", r)
					}
				}
				if len(validRoots) == 0 {
					return cli.Exit("No valid search paths", 1)
				}
			}

			// normalize ext slices to ".ext"
			norm := func(s []string) []string {
				out := make([]string, 0, len(s))
				for _, ext := range s {
					for _, v := range strings.Split(ext, ",") {
						v = strings.TrimSpace(v)
						if v == "" {
							continue
						}
						v = strings.TrimPrefix(v, ".")
						out = append(out, "."+strings.ToLower(v))
					}
				}
				return out
			}

			wh := norm(c.StringSlice("whitelist"))
			bl := norm(c.StringSlice("blacklist"))

			opts := internal.ScanOptions{
				Roots:                      validRoots,
				PatternFile:                c.String("pattern-file"),
				Depth:                      c.Int("depth"),
				Archives:                   c.Bool("archives"),
				Whitelist:                  wh,
				Blacklist:                  bl,
				Threads:                    c.Int("threads"),
				SaveFull:                   c.Bool("save-full"),
				SaveFullFolder:             c.String("save-full-folder"),
				FailFast:                   c.Bool("fail-fast"),
				SaveMatchesFile:            c.String("save-matches-file"),
				SaveMatchesByPatternFolder: c.String("save-matches-folder"),
			}
			if err := opts.Validate(); err != nil {
				return cli.Exit(err.Error(), 1)
			}
			opts.Prepare() // build fast lookup maps, set thread defaults

			var stats internal.AppStats
			finder := internal.NewFileScanner()

			if err := finder.Scan(ctx, opts, internal.NewResultSink(opts, &stats)); err != nil {
				if ctx.Err() != nil {
					logrus.Warn("Scan cancelled")
				} else {
					logrus.WithError(err).Error("Scan failed")
				}
			}

			fmt.Printf(
				"\n======= Scan finished in %s =======\nTotal files scanned: %d\nTotal matches found: %d\nErrors: %d\n",
				stats.Elapsed(), stats.FilesProcessed.Load(), stats.Matches.Load(), stats.Errors.Load(),
			)
			return nil
		},
	}

	if err := app.Run(os.Args); err != nil {
		logrus.Fatal(err)
	}
}
