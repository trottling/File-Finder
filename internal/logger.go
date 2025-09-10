package internal

import (
	"os"
	"strings"

	"github.com/sirupsen/logrus"
)

// InitLogger configures logrus once
func InitLogger(logfile, level string) {
	logrus.SetFormatter(&logrus.TextFormatter{
		ForceColors:   true,
		FullTimestamp: true,
		DisableQuote:  true,
		PadLevelText:  true,
	})

	if logfile != "" {
		if f, err := os.OpenFile(logfile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644); err == nil {
			logrus.SetOutput(f)
		} else {
			logrus.Warn("Failed to open log file, fallback to stdout")
		}
	}

	lvl, err := logrus.ParseLevel(strings.ToLower(level))
	if err != nil {
		lvl = logrus.InfoLevel
	}
	logrus.SetLevel(lvl)
}
