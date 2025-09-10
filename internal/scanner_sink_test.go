package internal

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSanitize(t *testing.T) {
	in := `re:^a.*$|foo/bar:*?"<>|`
	out := Sanitize(in)
	if strings.ContainsAny(out, `\/:*?"<>|`) {
		t.Fatalf("sanitize failed: %q", out)
	}
}

func TestResultSink_WritesFiles(t *testing.T) {
	dir := t.TempDir()
	opts := ScanOptions{
		SaveMatchesFile:            filepath.Join(dir, "all.txt"),
		SaveMatchesByPatternFolder: filepath.Join(dir, "by"),
	}
	var stats AppStats
	sink := NewResultSink(opts, &stats)

	// simulate one match line
	sink(MatchResult{
		FilePath: "/var/log/x.txt",
		Line:     "hello\n",
		Matched:  true,
		Pattern:  "plain:i:hello",
	})

	// check all.txt
	all, err := os.ReadFile(opts.SaveMatchesFile)
	if err != nil {
		t.Fatalf("read all.txt: %v", err)
	}
	if string(all) != "hello\n" {
		t.Fatalf("unexpected all content: %q", string(all))
	}

	// check per-pattern file
	ents, err := os.ReadDir(opts.SaveMatchesByPatternFolder)
	if err != nil || len(ents) != 1 {
		t.Fatalf("expected 1 per-pattern file, err=%v", err)
	}
	b, _ := os.ReadFile(filepath.Join(opts.SaveMatchesByPatternFolder, ents[0].Name()))
	if string(b) != "hello\n" {
		t.Fatalf("unexpected by-pattern content: %q", string(b))
	}
}
