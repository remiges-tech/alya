package filexfr

import (
	"testing"

	"github.com/remiges-tech/alya/jobs"
)

// mockFileChk is a mock file check function for testing
func mockFileChk(fileContents string, fileName string) (bool, jobs.JSONstr, []jobs.BatchInput_t, string, string, string) {
	jsonstr, err := jobs.NewJSONstr("{}")
	if err != nil {
		return false, jobs.JSONstr{}, nil, "", "", ""
	}
	return true, jsonstr, []jobs.BatchInput_t{}, "test-app", "test-op", ""
}

func TestNewFileTransferManager(t *testing.T) {
	fxs := NewFileXfrServer(&jobs.JobManager{})

	if fxs == nil {
		t.Fatal("NewFileXfrServer returned nil")
	}

	if fxs.jobManager == nil {
		t.Error("JobManager not set in FileXfrServer")
	}

	if len(fxs.fileChkMap) != 0 {
		t.Error("fileChkMap should be empty on initialization")
	}
}

func TestRegisterFileChk(t *testing.T) {
	fxs := NewFileXfrServer(&jobs.JobManager{})

	// Test registering a new file check function
	err := fxs.RegisterFileChk("csv", mockFileChk)
	if err != nil {
		t.Errorf("Failed to register file check function: %v", err)
	}

	// Verify that the function was registered
	if len(fxs.fileChkMap) != 1 {
		t.Errorf("Expected 1 registered file check function, got %d", len(fxs.fileChkMap))
	}

	if _, exists := fxs.fileChkMap["csv"]; !exists {
		t.Error("File check function for 'csv' not found in fileChkMap")
	}

	// Test registering a duplicate file check function
	err = fxs.RegisterFileChk("csv", mockFileChk)
	if err == nil {
		t.Error("Expected error when registering duplicate file check function, got nil")
	}

	// Verify that the number of registered functions hasn't changed
	if len(fxs.fileChkMap) != 1 {
		t.Errorf("Expected 1 registered file check function after duplicate registration attempt, got %d", len(fxs.fileChkMap))
	}

	// Test registering a different file type
	err = fxs.RegisterFileChk("json", mockFileChk)
	if err != nil {
		t.Errorf("Failed to register file check function for different file type: %v", err)
	}

	// Verify that the new function was registered
	if len(fxs.fileChkMap) != 2 {
		t.Errorf("Expected 2 registered file check functions, got %d", len(fxs.fileChkMap))
	}

	if _, exists := fxs.fileChkMap["json"]; !exists {
		t.Error("File check function for 'json' not found in fileChkMap")
	}
}
