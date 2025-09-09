package internal

import (
	"os"

	"github.com/sirupsen/logrus"
)

// InitLogger initializes the logger with optional file output.
func InitLogger(logfile string) {
	logrus.SetFormatter(&logrus.TextFormatter{
		ForceColors:   true,
		FullTimestamp: true,
		DisableQuote:  true,
		PadLevelText:  true,
	})
	if logfile != "" {
		file, err := os.OpenFile(logfile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err == nil {
			logrus.SetOutput(file)
		} else {
			logrus.Warn("Failed to open log file, logging to stdout")
		}
	}
}
