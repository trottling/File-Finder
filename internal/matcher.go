package internal

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/sirupsen/logrus"
)

// Pattern - fast interface for line match.
type Pattern interface {
	Match(string) bool
	Desc() string // for logs/files
}

type RegexPattern struct{ re *regexp.Regexp }

func (p *RegexPattern) Match(s string) bool { return p.re.MatchString(s) }
func (p *RegexPattern) Desc() string        { return p.re.String() }

type PlainPattern struct {
	s           string
	insensitive bool
}

func (p *PlainPattern) Match(s string) bool {
	if p.insensitive {
		return strings.Contains(strings.ToLower(s), p.s)
	}
	return strings.Contains(s, p.s)
}

func (p *PlainPattern) Desc() string {
	if p.insensitive {
		return "plain:i:" + p.s
	}
	return p.s
}

// LoadPatterns reads patterns file.
// Lines:
//
//	foo
//	plain:i:bar
//	re:^user=\\w+$
func LoadPatterns(path string) ([]Pattern, bool, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, false, err
	}
	defer f.Close()

	var ps []Pattern
	hasInsensitive := false
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		switch {
		case strings.HasPrefix(line, "re:"):
			re, err := regexp.Compile(line[3:])
			if err != nil {
				return nil, false, fmt.Errorf("invalid regex %q: %w", line, err)
			}
			ps = append(ps, &RegexPattern{re: re})
		case strings.HasPrefix(line, "plain:i:"):
			hasInsensitive = true
			ps = append(ps, &PlainPattern{s: strings.ToLower(line[8:]), insensitive: true})
		default:
			ps = append(ps, &PlainPattern{s: line})
		}
	}
	if err := sc.Err(); err != nil {
		return nil, false, err
	}
	logrus.Debugf("Loaded %d patterns", len(ps))
	return ps, hasInsensitive, nil
}
