package internal

import (
	"errors"
	"runtime"
)

// ScanOptions - public options from CLI.
type ScanOptions struct {
	Roots                      []string
	PatternFile                string
	Threads                    int
	Whitelist                  []string
	Blacklist                  []string
	Depth                      int
	Archives                   bool
	SaveFull                   bool
	SaveFullFolder             string
	FailFast                   bool
	SaveMatchesFile            string
	SaveMatchesByPatternFolder string

	whMap map[string]struct{}
	blMap map[string]struct{}
}

// Validate checks invariants.
func (o *ScanOptions) Validate() error {
	if o.PatternFile == "" {
		return errors.New("pattern-file is required")
	}
	if o.SaveFull && o.SaveFullFolder == "" {
		return errors.New("save-full-folder must be set when --save-full is used")
	}
	return nil
}

// Prepare builds fast lookup structures and sensible defaults.
func (o *ScanOptions) Prepare() {
	o.whMap = toSet(o.Whitelist)
	o.blMap = toSet(o.Blacklist)
	if o.Threads <= 0 {
		o.Threads = max(32, runtime.GOMAXPROCS(0)*4)
	}
}

func toSet(s []string) map[string]struct{} {
	if len(s) == 0 {
		return nil
	}
	m := make(map[string]struct{}, len(s))
	for _, x := range s {
		m[x] = struct{}{}
	}
	return m
}

func (o *ScanOptions) useWhitelist() bool { return len(o.whMap) > 0 }

func (o *ScanOptions) allowedExt(ext string) bool {
	// O(1) lookups
	if o.useWhitelist() {
		_, ok := o.whMap[ext]
		return ok
	}
	if o.blMap == nil {
		return true
	}
	_, blocked := o.blMap[ext]
	return !blocked
}
