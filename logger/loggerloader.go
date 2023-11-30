package logger

import (
	"os"

	"github.com/remiges-tech/logharbour/logharbour"
)

// LoadLogger creates a new logger. By default, it creates a LogHarbour logger.
func LoadLogger(appName string) Logger {
	// Create a new LogHarbour logger with stdout as the default writer
	logger := logharbour.NewLogger(appName, os.Stdout)

	// Wrap the *logharbour.Logger in a LogHarbour
	return &LogHarbour{logger}
}
