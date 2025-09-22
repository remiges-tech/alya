package filexfr

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/remiges-tech/alya/jobs"
	"github.com/remiges-tech/logharbour/logharbour"
)

// testWriter is a custom writer that writes to both test log and os.Stdout
type testWriter struct {
	t *testing.T
}

func (tw *testWriter) Write(p []byte) (n int, err error) {
	tw.t.Log(string(p))
	return os.Stdout.Write(p)
}

// setupTestLogger initializes and returns a test logger
func setupTestLogger(t *testing.T) *logharbour.Logger {
	loggerContext := logharbour.NewLoggerContext(logharbour.Debug2)
	writer := &testWriter{t: t}
	return logharbour.NewLogger(loggerContext, "FileXfrTest", writer)
}

// mockFileChk is a mock file check function for testing
func mockFileChk(fileContents string, fileName string, batchctx jobs.JSONstr) (bool, jobs.JSONstr, []jobs.BatchInput_t, string, string, string) {
	jsonstr, err := jobs.NewJSONstr("{}")
	if err != nil {
		return false, jobs.JSONstr{}, nil, "", "", ""
	}
	return true, jsonstr, []jobs.BatchInput_t{}, "test-app", "test-op", ""
}

func TestNewFileTransferManager(t *testing.T) {
	logger := setupTestLogger(t)
	fxs := NewFileXfrServer(&jobs.JobManager{}, nil, nil, FileXfrConfig{MaxObjectIDLength: 200}, logger)

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
	logger := setupTestLogger(t)
	fxs := NewFileXfrServer(&jobs.JobManager{}, nil, nil, FileXfrConfig{MaxObjectIDLength: 200}, logger)

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

func TestSanitizeFilename(t *testing.T) {
	testCases := []struct {
		input    string
		expected string
	}{
		{"normal_filename.txt", "normal_filename.txt"},
		{"file with spaces.csv", "file_with_spaces.csv"},
		{"file/with/slashes.json", "file_with_slashes.json"},
		{"file\\with\\backslashes.xml", "file_with_backslashes.xml"},
		{"file:with:colons.txt", "file_with_colons.txt"},
		{"file*with*asterisks.csv", "file_with_asterisks.csv"},
		{"file?with?questions.json", "file_with_questions.json"},
		{"file\"with\"quotes.xml", "file_with_quotes.xml"},
		{"file<with>brackets.txt", "file_with_brackets.txt"},
		{"file|with|pipes.csv", "file_with_pipes.csv"},
		{"a very long filename that exceeds the maximum length of 100 characters and should be truncated.txt",
			"a_very_long_filename_that_exceeds_the_maximum_length_of_100_characters_and_should_be_truncated.txt"},
	}

	for _, tc := range testCases {
		t.Run(tc.input, func(t *testing.T) {
			result := sanitizeFilename(tc.input)
			if result != tc.expected {
				t.Errorf("sanitizeFilename(%q) = %q; want %q", tc.input, result, tc.expected)
			}
			if len(result) > 100 {
				t.Errorf("sanitizeFilename(%q) returned a string longer than 100 characters: %d", tc.input, len(result))
			}
		})
	}
}

// TestGenerateObjectID verifies the behavior of the generateObjectID function.
// The test creates a FileXfrServer instance with a specified maximum object ID length,
// then generates and validates object IDs for each test case. It ensures that each
// generated ID meets all the above criteria and that consecutive calls with the same
// input filename produce different IDs due to the unique timestamp and UUID components.
func TestGenerateObjectID(t *testing.T) {
	logger := setupTestLogger(t)
	config := FileXfrConfig{MaxObjectIDLength: 200}
	fxs := NewFileXfrServer(&jobs.JobManager{}, nil, nil, config, logger)

	validateObjectID := func(t *testing.T, objectID, filename string) {
		t.Helper()

		parts := strings.Split(objectID, "_")
		if len(parts) < 3 {
			t.Errorf("Expected at least 3 parts in object ID, got %d: %s", len(parts), objectID)
			return
		}

		sanitizedFilename := sanitizeFilename(filename)
		if !strings.HasPrefix(objectID, sanitizedFilename) {
			t.Errorf("Object ID doesn't start with sanitized filename. Got: %s, Expected prefix: %s", objectID, sanitizedFilename)
		}

		// The timestamp should be the second-to-last part
		timestamp := parts[len(parts)-2]
		if _, err := time.Parse("20060102-150405", timestamp); err != nil {
			t.Errorf("Invalid timestamp format in object ID: %s", timestamp)
		}

		// The UUID should be the last part
		uniqueID := parts[len(parts)-1]
		if _, err := uuid.Parse(uniqueID); err != nil {
			t.Errorf("Invalid UUID in object ID: %s", uniqueID)
		}

		if len(objectID) > config.MaxObjectIDLength {
			t.Errorf("Generated object ID is too long (max %d characters): %s", config.MaxObjectIDLength, objectID)
		}
	}

	testCases := []struct {
		name     string
		filename string
	}{
		{"Normal filename", "test.txt"},
		{"Filename with spaces", "my file.pdf"},
		{"Filename with special characters", "report_2023!@#.xlsx"},
		{"Very long filename", "this_is_a_very_long_filename_that_exceeds_the_usual_length_limits_for_filenames_in_most_systems.docx"},
		{"Filename with non-ASCII characters", "कर्मचारी.txt"},
		{"Empty filename", ""},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			objectID := fxs.generateObjectID(tc.filename)
			validateObjectID(t, objectID, tc.filename)

			// Generate another ID with the same filename
			anotherObjectID := fxs.generateObjectID(tc.filename)

			// Verify that the IDs are different
			if objectID == anotherObjectID {
				t.Errorf("Generated object IDs should be unique even for the same filename, but got duplicate: %s", objectID)
			}

			// Verify that both IDs start with the same sanitized filename
			sanitizedFilename := sanitizeFilename(tc.filename)
			if !strings.HasPrefix(objectID, sanitizedFilename) || !strings.HasPrefix(anotherObjectID, sanitizedFilename) {
				t.Errorf("Both object IDs should start with the sanitized filename: %s", sanitizedFilename)
			}
		})
	}
}
