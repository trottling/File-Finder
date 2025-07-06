package internal

import (
	"archive/zip"
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestMatchReader_PlainAndRegexp(t *testing.T) {
	patterns := []Pattern{
		&PlainPattern{"foo"},
		&RegexPattern{re: mustCompile(t, `bar\d+`)},
	}
	input := "foo\nbar1\nnope\n"
	var results []MatchResult
	matchReader(strings.NewReader(input), patterns, false, func(res MatchResult) {
		results = append(results, res)
	}, "file.txt", "", new(int64), new(int64))
	if len(results) != 2 {
		t.Fatalf("expected 2 matches, got %d", len(results))
	}
	if !results[0].Matched || !results[1].Matched {
		t.Error("expected matches to be true")
	}
}

func TestMatchReader_SaveFull(t *testing.T) {
	patterns := []Pattern{&PlainPattern{"foo"}}
	input := "foo\nbar\n"
	var results []MatchResult
	matchReader(strings.NewReader(input), patterns, true, func(res MatchResult) {
		results = append(results, res)
	}, "file.txt", "", new(int64), new(int64))
	if len(results) != 1 {
		t.Fatalf("expected 1 match, got %d", len(results))
	}
	if !bytes.Contains(results[0].FullFile, []byte("foo")) {
		t.Error("expected full file content to contain 'foo'")
	}
}

func TestMatchReader_Error(t *testing.T) {
	patterns := []Pattern{&PlainPattern{"foo"}}
	broken := &errorReader{}
	var results []MatchResult
	matchReader(broken, patterns, false, func(res MatchResult) {
		results = append(results, res)
	}, "file.txt", "", new(int64), new(int64))
	if len(results) != 1 || results[0].Error == nil {
		t.Error("expected error result")
	}
}

type errorReader struct{}

func (e *errorReader) Read(p []byte) (int, error) { return 0, os.ErrInvalid }

func mustCompile(t *testing.T, expr string) *regexp.Regexp {
	t.Helper()
	re, err := regexp.Compile(expr)
	if err != nil {
		t.Fatalf("failed to compile regexp: %v", err)
	}
	return re
}

func TestFileScanner_Scan_Integration(t *testing.T) {
	dir := t.TempDir()
	file1 := dir + "/a.txt"
	file2 := dir + "/b.txt"
	os.WriteFile(file1, []byte("foo\nbar1\n"), 0644)
	os.WriteFile(file2, []byte("nope\nbar2\n"), 0644)
	patterns := dir + "/patterns.txt"
	os.WriteFile(patterns, []byte("foo\nre:bar\\d+\n"), 0644)

	scanner := NewFileScanner()
	var matches []MatchResult
	err := scanner.Scan(context.Background(), ScanOptions{
		Roots:       []string{dir},
		PatternFile: patterns,
		Threads:     2,
	}, func(res MatchResult) {
		if res.Matched {
			matches = append(matches, res)
		}
	})
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}
	if len(matches) != 3 {
		t.Errorf("expected 3 matches, got %d", len(matches))
	}
}

func TestFileScanner_Scan_ArchiveIntegration(t *testing.T) {
	dir := t.TempDir()
	zipPath := filepath.Join(dir, "test.zip")
	f, err := os.Create(zipPath)
	if err != nil {
		t.Fatalf("failed to create zip: %v", err)
	}
	zw := zip.NewWriter(f)
	w1, _ := zw.Create("a.txt")
	io.WriteString(w1, "foo\nbar1\n")
	w2, _ := zw.Create("b.txt")
	io.WriteString(w2, "nope\nbar2\n")
	zw.Close()
	f.Close()

	patterns := filepath.Join(dir, "patterns.txt")
	os.WriteFile(patterns, []byte("foo\nre:bar\\d+\n"), 0644)

	scanner := NewFileScanner()
	var matches []MatchResult
	err = scanner.Scan(context.Background(), ScanOptions{
		Roots:       []string{dir},
		PatternFile: patterns,
		Threads:     2,
		Archives:    true,
	}, func(res MatchResult) {
		if res.Matched {
			matches = append(matches, res)
		}
	})
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}
	if len(matches) != 3 {
		t.Errorf("expected 3 matches in archive, got %d", len(matches))
	}
}
