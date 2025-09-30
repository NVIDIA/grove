package utils

import (
	"os"

	"github.com/sirupsen/logrus"
)

// NewCILogger creates a new logrus logger with the specified verbosity level
func NewCILogger(verbosity logrus.Level) *logrus.Logger {
	logger := logrus.New()
	logger.SetOutput(os.Stdout)
	logger.SetFormatter(&logrus.TextFormatter{
		FullTimestamp: true,
		ForceColors:   true,
	})
	logger.SetLevel(verbosity)

	return logger
}
