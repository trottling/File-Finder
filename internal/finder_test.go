package internal

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

// ==== Pattern tests ====

func TestPlainPattern_Match_CaseSensitive(t *testing.T) {
	p := PlainPattern{"abc", false}
	if !p.Match("123abc456") {
		t.Error("plain pattern (sensitive) failed to match substring")
	}
	if p.Match("ABC") {
		t.Error("plain pattern (sensitive) incorrectly matched different case")
	}
}

func TestPlainPattern_Match_CaseInsensitive(t *testing.T) {
	p := PlainPattern{"abc", true}
	if !p.Match("XXXaBcYYY") {
		t.Error("plain pattern (insensitive) failed to match substring with different case")
	}
}

func TestRegexPattern_Match(t *testing.T) {
	r, _ := compileRegexp("foo.*bar")
	p := RegexPattern{r}
	if !p.Match("xxxfooqwertybarzzz") {
		t.Error("regex pattern failed to match")
	}
	if p.Match("foobaz") {
		t.Error("regex pattern incorrectly matched")
	}
}

func compileRegexp(s string) (*regexp.Regexp, error) {
	return regexp.Compile(s)
}

// ==== Tests loading patterns from a file ====

func TestLoadPatterns(t *testing.T) {
	content := `re:abc[0-9]+
plain:foo
plain:i:BAR
`

	tmp, err := os.CreateTemp("", "patterns")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmp.Name())
	tmp.WriteString(content)
	tmp.Close()

	patterns, err := loadPatterns(tmp.Name())
	if err != nil {
		t.Fatalf("loadPatterns returned error: %v", err)
	}
	if len(patterns) != 3 {
		t.Errorf("expected 3 patterns, got %d", len(patterns))
	}

	// Checking that it works and is case-insensitive
	matched := false
	for _, p := range patterns {
		if p.Match("something BAR here") {
			matched = true
		}
	}
	if !matched {
		t.Error("plain:i: pattern did not match")
	}
}

// ==== File processing test ====

func TestMatchReader_MatchLines(t *testing.T) {
	// 2 паттерна: plain и regex
	patterns := []Pattern{
		&PlainPattern{"secret", false},
		&RegexPattern{regexp.MustCompile(`token-\d+`)},
	}
	input := "hello world\nthis is a secret line\ntoken-12345 here\nanother line"
	r := strings.NewReader(input)

	var results []MatchResult
	matchReader(r, patterns, false, func(res MatchResult) {
		if res.Matched {
			results = append(results, res)
		}
	}, "testfile.txt", "", nil, nil, "")

	if len(results) != 2 {
		t.Fatalf("expected 2 matches, got %d", len(results))
	}
	if !strings.Contains(results[0].Line, "secret") {
		t.Error("first match is not the secret line")
	}
	if !strings.Contains(results[1].Line, "token-12345") {
		t.Error("second match is not the token line")
	}
}

// ==== Test error when loading a non-existent file ====

func TestLoadPatterns_FileNotExist(t *testing.T) {
	_, err := loadPatterns("doesnotexist_12345.txt")
	if err == nil {
		t.Error("expected error for missing file, got nil")
	}
}
