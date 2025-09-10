package internal

import (
	"context"
	"errors"
	"io"
	iofs "io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"

	"github.com/mholt/archives"
	"github.com/sirupsen/logrus"
)

const maxArchiveFiles = 10000 // zip-bomb protection

// IsArchive by extension. O(1) map lookup
var archiveExt = map[string]struct{}{
	".zip": {}, ".tar": {}, ".gz": {}, ".bz2": {}, ".xz": {},
	".rar": {}, ".br": {}, ".lz4": {}, ".lz": {}, ".mz": {},
	".sz": {}, ".s2": {}, ".zz": {}, ".zst": {}, ".7z": {},
}

// Task describes a unit of work
type Task struct {
	path      string
	innerPath string
	isArchive bool
}

// DetectRoots returns default roots for OS if user didn't provide any.
func DetectRoots(goos string) []string {
	if goos == "windows" {
		var drives []string
		for c := 'C'; c <= 'Z'; c++ {
			p := string(c) + ":\\"
			if st, err := os.Stat(p); err == nil && st.IsDir() {
				drives = append(drives, p)
			}
		}
		return drives
	}
	roots := []string{"/"}
	mounts := []string{"/mnt", "/media", "/run/media", "/Volumes"} // macOS at the end
	for _, m := range mounts {
		if st, err := os.Stat(m); err == nil && st.IsDir() {
			ents, _ := os.ReadDir(m)
			for _, e := range ents {
				roots = append(roots, filepath.Join(m, e.Name()))
			}
		}
	}
	return roots
}

// WalkWithDepth uses WalkDir and cuts branches by depth.
func WalkWithDepth(ctx context.Context, root string, maxDepth int, fn func(path string, d os.DirEntry, err error) error) error {
	return filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err != nil {
			return fn(path, d, err)
		}
		if maxDepth > 0 {
			rel, _ := filepath.Rel(root, path)
			if rel != "." && depthCount(rel) > maxDepth {
				return filepath.SkipDir
			}
		}
		return fn(path, d, nil)
	})
}

// WalkArchive Feed archive entries as tasks.
func WalkArchive(ctx context.Context, path string, send func(Task), found *atomic.Int64, opts ScanOptions) {
	fs, err := archives.FileSystem(ctx, path, nil)
	if err != nil {
		logrus.WithError(err).WithField("archive", path).Error("open archive")
		return
	}
	if closer, ok := fs.(io.Closer); ok {
		defer closer.Close()
	}

	count := 0
	_ = iofs.WalkDir(fs, ".", func(inner string, d iofs.DirEntry, err error) error {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err != nil || d.IsDir() {
			return nil
		}
		if count >= maxArchiveFiles {
			logrus.Warnf("Archive %s skipped: too many files (>= %d)", path, maxArchiveFiles)
			return errors.New("archive file limit reached")
		}
		ext := strings.ToLower(filepath.Ext(inner))
		if !opts.allowedExt(ext) {
			return nil
		}
		found.Add(1)
		send(Task{path: path, innerPath: inner, isArchive: true})
		count++
		return nil
	})
}

func depthCount(rel string) int {
	if rel == "" {
		return 0
	}
	return strings.Count(rel, string(os.PathSeparator)) + 1
}

func Sanitize(s string) string {
	r := strings.NewReplacer(
		string(os.PathSeparator), "_", ":", "_", "*", "_", "?", "_",
		"\"", "_", "<", "_", ">", "_", "|", "_",
	)
	return r.Replace(s)
}

func IsArchive(path string) bool {
	_, ok := archiveExt[strings.ToLower(filepath.Ext(path))]
	return ok
}
