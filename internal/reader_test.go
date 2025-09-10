package internal

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

type stubPattern struct {
	sub string
}

func (p stubPattern) Match(s string) bool { return strings.Contains(s, p.sub) }
func (p stubPattern) Desc() string        { return p.sub }

func TestFinalSavePath(t *testing.T) {
	p := finalSavePath("/out", "/var/log/sys.log", "")
	if !strings.Contains(p, "/out/") || !strings.Contains(p, "var_log_sys.log") {
		t.Fatalf("unexpected: %s", p)
	}
	p = finalSavePath("/out", "/x/archive.zip", "inside/dir/file.txt")
	if !strings.HasSuffix(p, "x_archive.zip/inside_dir_file.txt") {
		t.Fatalf("unexpected: %s", p)
	}
}

func TestMatchReader_LinesMode(t *testing.T) {
	data := "hello\nworld\nxHELLOy\n"
	pats := []Pattern{stubPattern{sub: "hello"}}
	var matches int
	var got []string

	on := func(m MatchResult) {
		if m.Matched && m.Line != "" {
			matches++
			got = append(got, strings.TrimSpace(m.Line))
		}
	}

	var matchCnt, errCnt atomic.Int64
	matchReader(bytes.NewBufferString(data), pats, false, false, "", on, "/f.txt", "", &matchCnt, &errCnt)

	if matches != 1 || matchCnt.Load() != 1 {
		t.Fatalf("want 1 match, got %d", matches)
	}
	if got[0] != "hello" {
		t.Fatalf("bad line: %q", got[0])
	}
}

func TestMatchReader_SaveFullMode(t *testing.T) {
	data := "foo\nBAR\nbaz\n"
	pats := []Pattern{stubPattern{sub: "BAR"}}

	dir := t.TempDir()
	var matchCnt, errCnt atomic.Int64
	found := false
	on := func(m MatchResult) {
		if m.Matched {
			found = true
		}
	}

	matchReader(bytes.NewBufferString(data), pats, false, true, dir, on, "/tmp/file.txt", "", &matchCnt, &errCnt)
	if !found {
		t.Fatal("expected match")
	}

	// ensure file appeared
	glob := filepath.Join(dir, "tmp_file.txt")
	_, err := os.Stat(glob)
	if err != nil {
		t.Fatalf("expected saved file at %s: %v", glob, err)
	}
}
