package logger_test

import (
	"os"
	"strings"
	"testing"

	"github.com/remiges-tech/alya/logger"
)

func TestConsoleLogger_Log(t *testing.T) {
	logger := &logger.ConsoleLogger{}
	// We can't really check the console output, but we can at least make sure it doesn't panic
	logger.Log("Test message")
}

func TestFileLogger_Log(t *testing.T) {
	tempFile, _ := os.CreateTemp(os.TempDir(), "log-")
	defer os.Remove(tempFile.Name())

	logger := &logger.FileLogger{FilePath: tempFile.Name()}
	logger.Log("Test message")

	bytes, _ := os.ReadFile(tempFile.Name())
	if !strings.Contains(string(bytes), "Test message") {
		t.Errorf("Log message not found in file")
	}
}
