package scanner

import (
	"context"
)

// ScanOptions contains options for the scanning process.
type ScanOptions struct {
	Roots       []string
	PatternFile string
	Threads     int
	Whitelist   []string
	Blacklist   []string
	Depth       int
	Archives    bool
	SaveFull    bool
}

// MatchResult represents a single match found during scanning.
type MatchResult struct {
	FilePath   string
	InnerPath  string
	LineNumber int
	Line       string
	FullFile   []byte
	Matched    bool
	Error      error
}

type task struct {
	path      string
	innerPath string
	isArchive bool
}

// Scanner is the interface for file scanners.
type Scanner interface {
	Scan(ctx context.Context, opts ScanOptions, onMatch func(MatchResult)) error
}
