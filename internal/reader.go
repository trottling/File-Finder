package internal

import (
	"bufio"
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
)

// matchReader streams file lines and reports matches.
// If saveFull && folder != "", content is written to a temp file via Tee.
// On first match the temp file is moved to final destination; otherwise it's removed.
// This keeps memory low and avoids double-reading.
func matchReader(
	reader io.Reader,
	patterns []Pattern,
	hasInsensitive bool,
	saveFull bool,
	saveFullFolder string,
	onMatch func(MatchResult),
	filePath, innerPath string,
	matchCount, errorCount *atomic.Int64,
) {
	var (
		tee     = reader
		tmpPath string
		tmpFile *os.File
		err     error
	)

	// Prepare tee into temp file only if we might save full content
	if saveFull && saveFullFolder != "" {
		if err = os.MkdirAll(saveFullFolder, 0755); err == nil {
			tmpFile, err = os.CreateTemp(saveFullFolder, "ff-*")
			if err == nil {
				tee = io.TeeReader(reader, tmpFile)
			}
		}
		// if any error, silently fall back to no-full-save
		if err != nil {
			saveFull = false
		}
	}

	br := bufio.NewReaderSize(tee, 64*1024)
	lineNum := 0
	lowerBuf := new(bytes.Buffer) // reuse for lowercasing lines

	var matchedPattern string
	found := false

	for {
		b, err := br.ReadBytes('\n')
		if len(b) > 0 {
			line := string(b)
			// Lowercase once per line if we have insensitive patterns

			var lineForCheck string
			if hasInsensitive {
				lowerBuf.Reset()
				lowerBuf.WriteString(strings.ToLower(line))
				lineForCheck = lowerBuf.String()
			} else {
				lineForCheck = line
			}

			for _, p := range patterns {

				if p.Match(lineForCheck) {
					matchedPattern = p.Desc()
					found = true

					if saveFull {
						// flush temp and move it
						if tmpFile != nil {

							tmpFile.Sync()
							tmpPath = finalSavePath(saveFullFolder, filePath, innerPath)

							// ensure parent dir exists: <saveFullFolder>/<base>/
							if err := os.MkdirAll(filepath.Dir(tmpPath), 0755); err != nil {
								errorCount.Add(1)
								onMatch(MatchResult{FilePath: filePath, InnerPath: innerPath, Error: err})
								return
							}

							_ = tmpFile.Close()
							// atomic rename
							if err := os.Rename(tmpFile.Name(), tmpPath); err != nil {
								errorCount.Add(1)
								onMatch(MatchResult{FilePath: filePath, InnerPath: innerPath, Error: err})
								return
							}
						}
						onMatch(MatchResult{FilePath: filePath, InnerPath: innerPath, Matched: true, Pattern: matchedPattern})
					}
					matchCount.Add(1)
					// do not break reading: we still need to drain if tee is active for archives;
					// but we can stop sending more matches for that file.
					// So we only break the pattern loop.
					break
				}
			}
			lineNum++
		}
		if err != nil {
			if err != io.EOF {
				errorCount.Add(1)
				onMatch(MatchResult{FilePath: filePath, InnerPath: innerPath, Error: err})
			}
			break
		}
	}

	// cleanup temp if no match
	if saveFull && tmpFile != nil {
		info, _ := os.Stat(tmpFile.Name())
		_ = tmpFile.Close()
		if !found {
			if info != nil {
				_ = os.Remove(tmpFile.Name())
			}
		}
	}
}

func finalSavePath(folder, filePath, innerPath string) string {
	base := strings.ReplaceAll(filePath, string(os.PathSeparator), "_")
	base = strings.ReplaceAll(base, "/", "_")
	base = strings.ReplaceAll(base, "\\", "_")
	base = strings.ReplaceAll(base, ":", "_")

	if innerPath != "" {
		inner := strings.ReplaceAll(innerPath, string(os.PathSeparator), "_")
		inner = strings.ReplaceAll(inner, "/", "_")
		inner = strings.ReplaceAll(inner, "\\", "_")
		return filepath.Join(folder, base, inner)
	}
	return filepath.Join(folder, base)
}
