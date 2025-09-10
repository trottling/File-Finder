package internal

import (
	"sync/atomic"
	"time"
)

// AppStats atomic counters for totals
type AppStats struct {
	start          time.Time
	FilesFound     atomic.Int64
	FilesProcessed atomic.Int64
	Matches        atomic.Int64
	Errors         atomic.Int64
}

func (s *AppStats) Start() {
	s.start = time.Now()
}

func (s *AppStats) Elapsed() time.Duration {
	return time.Since(s.start)
}
