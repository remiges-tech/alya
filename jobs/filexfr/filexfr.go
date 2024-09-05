package filexfr

import (
	"fmt"
	"sync"

	"github.com/remiges-tech/alya/jobs"
)

// FileChk is the type for file checking functions
type FileChk func(fileContents string, fileName string) (bool, jobs.JSONstr, []jobs.BatchInput_t, string, string, string)

// FileXfrServer handles file transfer operations
type FileXfrServer struct {
	fileChkMap map[string]FileChk
	jobManager *jobs.JobManager
	mu         sync.RWMutex
}

// NewFileXfrServer creates a new FileXfrServer
func NewFileXfrServer(jobManager *jobs.JobManager) *FileXfrServer {
	return &FileXfrServer{
		fileChkMap: make(map[string]FileChk),
		jobManager: jobManager,
	}
}

// RegisterFileChk allows applications to register a file checking function for a specific file type.
// Each file type can only have one registered file checking function.
// Attempting to register a second function for the same file type will result in an error.
func (fxs *FileXfrServer) RegisterFileChk(fileType string, fileChkFn FileChk) error {
	fxs.mu.Lock()
	defer fxs.mu.Unlock()

	if _, exists := fxs.fileChkMap[fileType]; exists {
		return fmt.Errorf("file check function already registered for file type: %s", fileType)
	}

	fxs.fileChkMap[fileType] = fileChkFn
	return nil
}
