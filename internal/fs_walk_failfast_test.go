package internal

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestFailFastSentinel(t *testing.T) {
	// emulate walker callback returning ErrWalkFailFast
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "a.txt"), []byte("x"), 0644)

	gotErr := WalkWithDepth(context.Background(), dir, 0, func(path string, d os.DirEntry, err error) error {
		// trigger only on first file
		if !d.IsDir() {
			return ErrWalkFailFast
		}
		return nil
	})
	if !errors.Is(gotErr, ErrWalkFailFast) {
		t.Fatalf("expected ErrWalkFailFast, got %v", gotErr)
	}
}
