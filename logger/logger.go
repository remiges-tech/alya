package logger

import (
	"fmt"
	"log"
	"os"

	"github.com/remiges-tech/logharbour/logharbour"
)

// Logger is an interface that represents a logger.
type Logger interface {
	Log(message string) error
}

// ConsoleLogger logs messages to the console.
type ConsoleLogger struct{}

func (cl *ConsoleLogger) Log(message string) error {
	fmt.Println(message)
	return nil
}

// FileLogger logs messages to a file.
type FileLogger struct {
	FilePath string
}

func (fl *FileLogger) Log(message string) error {
	if fl.FilePath == "" {
		return fmt.Errorf("FilePath cannot be empty")
	}

	file, err := os.OpenFile(fl.FilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	logger := log.New(file, "", log.LstdFlags)
	logger.Println(message)

	return nil
}

type LogHarbour struct {
	*logharbour.Logger
}

func (lh *LogHarbour) Log(message string) error {
	lh.LogActivity("", message)
	return nil
}
