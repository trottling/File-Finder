package internal

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestIsArchive(t *testing.T) {
	exts := []string{".zip", ".tar", ".gz", ".bz2", ".xz", ".rar", ".7z", ".zst"}
	for _, e := range exts {
		if !IsArchive("x" + e) {
			t.Errorf("expected archive for %s", e)
		}
	}
	if IsArchive("file.txt") {
		t.Errorf("txt is not archive")
	}
}

func TestDepthCount(t *testing.T) {
	if depthCount("") != 0 {
		t.Fatal("empty rel should be 0")
	}
	if depthCount("a") != 1 || depthCount(filepath.Join("a", "b")) != 2 {
		t.Fatal("depthCount wrong")
	}
}

func TestWalkWithDepth(t *testing.T) {
	dir := t.TempDir()
	// a/, a/b/, a/b/c.txt
	if err := os.MkdirAll(filepath.Join(dir, "a", "b"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "a", "b", "c.txt"), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}

	var seen []string
	err := WalkWithDepth(context.Background(), dir, 1, func(path string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			seen = append(seen, path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk: %v", err)
	}

	// with depth=1 we should not see c.txt under a/b
	for _, p := range seen {
		if filepath.Base(p) == "c.txt" {
			t.Fatalf("should not visit deep file with depth=1")
		}
	}

	// depth=0 unlimited should see c.txt
	seen = nil
	err = WalkWithDepth(context.Background(), dir, 0, func(path string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			seen = append(seen, path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk: %v", err)
	}
	found := false
	for _, p := range seen {
		if filepath.Base(p) == "c.txt" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected to see c.txt with depth=0")
	}

	// DetectRoots smoke (non-strict)
	_ = DetectRoots(runtime.GOOS)
}
