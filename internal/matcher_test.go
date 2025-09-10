package internal

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadPatterns_BasicAndRegexAndInsensitive(t *testing.T) {
	dir := t.TempDir()
	fp := filepath.Join(dir, "patterns.txt")
	content := `
# comment
plain:i:HeLLo
world
re:^id=\d{3}$
`
	if err := os.WriteFile(fp, []byte(content), 0644); err != nil {
		t.Fatalf("write patterns: %v", err)
	}

	ps, hasInsensitive, err := LoadPatterns(fp)
	if err != nil {
		t.Fatalf("LoadPatterns error: %v", err)
	}
	if !hasInsensitive {
		t.Errorf("expected hasInsensitive=true")
	}
	if len(ps) != 3 {
		t.Fatalf("expected 3 patterns, got %d", len(ps))
	}

	// check matching
	if !ps[0].Match("hello there") {
		t.Errorf("plain:i should match lowercased")
	}
	if !ps[1].Match("say world!") {
		t.Errorf("plain should match as substring")
	}
	if ps[1].Match("w0rld") {
		t.Errorf("plain should not match wrong substring")
	}
	if !ps[2].Match("id=123") || ps[2].Match("id=12x") {
		t.Errorf("regex match failed")
	}

	// desc sanity
	if ps[0].Desc() != "plain:i:hello" {
		t.Errorf("unexpected desc: %q", ps[0].Desc())
	}
}

func TestLoadPatterns_InvalidRegex(t *testing.T) {
	dir := t.TempDir()
	fp := filepath.Join(dir, "bad.txt")
	_ = os.WriteFile(fp, []byte("re:[\n"), 0644)
	_, _, err := LoadPatterns(fp)
	if err == nil {
		t.Fatal("expected regex compile error")
	}
}
