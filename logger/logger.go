package logger

import (
	"io"
	"log"
	"os"

	"github.com/remiges-tech/logharbour/logharbour"
)

// Logger is an interface that represents a logger.
type Logger interface {
	Log(message string)
	LogDebug(message string)
}

// StdLogger logs messages to an io.Writer.
type StdLogger struct {
	logger *log.Logger
}

// NewLogger creates a new StdLogger.
func NewLogger(output io.Writer) *StdLogger {
	logger := log.New(output, "", log.LstdFlags)
	return &StdLogger{logger: logger}
}

func (sl *StdLogger) Log(message string) {
	sl.logger.Println(message)
}

// FileLogger logs messages to a file.
type FileLogger struct {
	FilePath string
	file     *os.File
	logger   *log.Logger
}

// NewFileLogger creates a new FileLogger with the given file path.
func NewFileLogger(filePath string) *FileLogger {
	file, err := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatalf("Error opening log file: %v", err)
	}
	logger := log.New(file, "", log.LstdFlags)
	return &FileLogger{FilePath: filePath, file: file, logger: logger}
}

func (fl *FileLogger) Log(message string) {
	fl.logger.Println(message)
}

func (fl *FileLogger) LogDebug(message string) {
	if fl.FilePath == "" {
		log.Fatalln("File path cannot be empty")
	}

	fl.logger.Println(message)
}

// Close closes the file.
// It is the caller's responsibility to call Close when finished with the logger.
func (fl *FileLogger) Close() error {
	if fl.file != nil {
		return fl.file.Close()
	}
	return nil
}

type LogHarbour struct {
	*logharbour.Logger
}

func (lh *LogHarbour) Log(message string) {
	lh.LogActivity("", message)
}

func (lh *LogHarbour) LogDebug(message string) {
	fileName, lineNumber, functionName, stackTrace := logharbour.GetDebugInfo(1)
	debugInfo := logharbour.DebugInfo{
		FileName:     fileName,
		LineNumber:   lineNumber,
		FunctionName: functionName,
		StackTrace:   stackTrace,
	}
	lh.Logger.LogDebug(message, debugInfo)
}
