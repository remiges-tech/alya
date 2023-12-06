package logger

import (
	"fmt"
	"log"
	"os"

	"github.com/remiges-tech/logharbour/logharbour"
)

// Logger is an interface that represents a logger.
type Logger interface {
	Log(message string)
}

// ConsoleLogger logs messages to the console.
type ConsoleLogger struct{}

func (cl *ConsoleLogger) Log(message string) {
	fmt.Println(message)
}

// FileLogger logs messages to a file.
type FileLogger struct {
	FilePath string
}

func (fl *FileLogger) Log(message string) {
	if fl.FilePath == "" {
		log.Fatalln("File path cannot be empty")
	}

	file, err := os.OpenFile(fl.FilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatalf("Error opening log file: %v", err)
	}
	defer file.Close()

	logger := log.New(file, "", log.LstdFlags)
	logger.Println(message)
}

type LogHarbour struct {
	*logharbour.Logger
}

func (lh *LogHarbour) Log(message string) {
	lh.LogActivity("", message)
}
