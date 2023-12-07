package logger_test

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/remiges-tech/alya/logger"
)

func TestNewLogger(t *testing.T) {
	// Create a buffer to capture logger output
	var buf bytes.Buffer

	// Create a new StdLogger with the buffer as output
	logger := logger.NewLogger(&buf)

	// Log a message
	logger.Log("Test message")

	// Check that the message was written to the logger output
	if !strings.Contains(buf.String(), "Test message") {
		t.Errorf("Expected 'Test message', got '%s'", buf.String())
	}
}

func TestNewFileLogger(t *testing.T) {
	// Create a temporary file
	tmpfile, err := os.CreateTemp("", "log")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpfile.Name()) // clean up

	// Create a new FileLogger
	logger := logger.NewFileLogger(tmpfile.Name())

	// Log multiple messages
	logger.Log("Test message 1")
	logger.Log("Test message 2")

	// Read the file contents
	content, err := os.ReadFile(tmpfile.Name())
	if err != nil {
		t.Fatal(err)
	}

	// Check that the file contains both log messages
	lines := strings.Split(string(content), "\n")
	if len(lines) < 2 {
		t.Fatalf("Expected at least 2 lines, got %d", len(lines))
	}
	if !strings.Contains(lines[0], "Test message 1") {
		t.Errorf("Expected 'Test message 1', got '%s'", lines[0])
	}
	if !strings.Contains(lines[1], "Test message 2") {
		t.Errorf("Expected 'Test message 2', got '%s'", lines[1])
	}
}
