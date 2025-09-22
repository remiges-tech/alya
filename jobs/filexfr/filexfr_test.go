package filexfr

import (
	"context"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/remiges-tech/alya/jobs"
	"github.com/remiges-tech/alya/jobs/objstore"
	"github.com/remiges-tech/logharbour/logharbour"
	"github.com/stretchr/testify/assert"
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

	assert.NotNil(t, fxs, "NewFileXfrServer should not return nil")
	assert.NotNil(t, fxs.jobManager, "JobManager should be set in FileXfrServer")
	assert.Empty(t, fxs.fileChkMap, "fileChkMap should be empty on initialization")
}


func TestBulkfileinProcessIsObjectID(t *testing.T) {
	// SETUP
	logger := setupTestLogger(t)
	batchctx, _ := jobs.NewJSONstr(`{"test": "context"}`)

	// Track which file checker was called and with what content
	var receivedContent string

	// Helper function to create a spy FileChk that captures content and always fails
	createSpyFileChk := func() FileChk {
		return func(fileContents string, fileName string, batchctx jobs.JSONstr) (bool, jobs.JSONstr, []jobs.BatchInput_t, string, string, string) {
			receivedContent = fileContents
			return false, jobs.JSONstr{}, nil, "", "", ""
		}
	}

	t.Run("isObjectID_false_passes_content_directly", func(t *testing.T) {
		// SETUP
		fileContent := "test,data,123\ntest2,data2,456"
		receivedContent = ""
		spyFileChk := createSpyFileChk()

		mockObjStore := &objstore.ObjectStoreMock{}
		fxs := NewFileXfrServer(nil, mockObjStore, nil, FileXfrConfig{MaxObjectIDLength: 200}, logger)
		err := fxs.RegisterFileChk("test", spyFileChk)
		assert.NoError(t, err)

		// WHEN
		_, err = fxs.BulkfileinProcess(fileContent, "test.csv", "test", batchctx, false)

		// THEN
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "file check failed")
		assert.NotContains(t, err.Error(), "failed to read object contents")
		assert.Equal(t, fileContent, receivedContent)
	})

	t.Run("isObjectID_true_retrieves_from_object_store", func(t *testing.T) {
		// SETUP
		objectID := "test-object-id"
		expectedContent := "content,from,store"
		receivedContent = ""
		spyFileChk := createSpyFileChk()

		mockObjStore := &objstore.ObjectStoreMock{
			GetFunc: func(ctx context.Context, bucket, objectID string) (io.ReadCloser, error) {
				return io.NopCloser(strings.NewReader(expectedContent)), nil
			},
			PutFunc: func(ctx context.Context, bucket, objectID string, reader io.Reader, size int64, contentType string) error {
				return nil
			},
			DeleteFunc: func(ctx context.Context, bucket, objectID string) error {
				return nil
			},
		}

		fxs := NewFileXfrServer(nil, mockObjStore, nil, FileXfrConfig{MaxObjectIDLength: 200}, logger)
		err := fxs.RegisterFileChk("test", spyFileChk)
		assert.NoError(t, err)

		// WHEN
		_, err = fxs.BulkfileinProcess(objectID, "test.csv", "test", batchctx, true)

		// THEN
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "file check failed")
		assert.Equal(t, expectedContent, receivedContent)
	})

	t.Run("isObjectID_true_handles_object_store_error", func(t *testing.T) {
		// SETUP
		objectID := "nonexistent-object"
		receivedContent = ""
		spyFileChk := createSpyFileChk()

		mockObjStore := &objstore.ObjectStoreMock{
			GetFunc: func(ctx context.Context, bucket, objectID string) (io.ReadCloser, error) {
				return nil, assert.AnError
			},
		}

		fxs := NewFileXfrServer(nil, mockObjStore, nil, FileXfrConfig{MaxObjectIDLength: 200}, logger)
		err := fxs.RegisterFileChk("test", spyFileChk)
		assert.NoError(t, err)

		// WHEN
		_, err = fxs.BulkfileinProcess(objectID, "test.csv", "test", batchctx, true)

		// THEN
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to read object contents")
		assert.Empty(t, receivedContent)
	})
}

func TestRegisterFileChk(t *testing.T) {
	logger := setupTestLogger(t)
	fxs := NewFileXfrServer(&jobs.JobManager{}, nil, nil, FileXfrConfig{MaxObjectIDLength: 200}, logger)

	// Test registering a new file check function
	err := fxs.RegisterFileChk("csv", mockFileChk)
	assert.NoError(t, err, "Should successfully register file check function")
	assert.Len(t, fxs.fileChkMap, 1, "Should have 1 registered file check function")
	assert.Contains(t, fxs.fileChkMap, "csv", "Should contain registered 'csv' file check function")

	// Test registering a duplicate file check function
	err = fxs.RegisterFileChk("csv", mockFileChk)
	assert.Error(t, err, "Should return error when registering duplicate file check function")
	assert.Len(t, fxs.fileChkMap, 1, "Should still have only 1 registered function after duplicate attempt")

	// Test registering a different file type
	err = fxs.RegisterFileChk("json", mockFileChk)
	assert.NoError(t, err, "Should successfully register different file type")
	assert.Len(t, fxs.fileChkMap, 2, "Should have 2 registered file check functions")
	assert.Contains(t, fxs.fileChkMap, "json", "Should contain registered 'json' file check function")
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
		{"a very long filename that exceeds the previous maximum length of 100 characters and is now preserved completely.txt",
			"a_very_long_filename_that_exceeds_the_previous_maximum_length_of_100_characters_and_is_now_preserved_completely.txt"},
	}

	for _, tc := range testCases {
		t.Run(tc.input, func(t *testing.T) {
			result := sanitizeFilename(tc.input)
			assert.Equal(t, tc.expected, result, "sanitizeFilename should replace special characters")
		})
	}
}

// TestGenerateObjectID verifies the behavior of the generateObjectID function.
// The function returns the sanitized filename, truncated if it exceeds MaxObjectIDLength.
func TestGenerateObjectID(t *testing.T) {
	logger := setupTestLogger(t)

	t.Run("Default configuration", func(t *testing.T) {
		// Test with default MaxObjectIDLength (500)
		fxs := NewFileXfrServer(&jobs.JobManager{}, nil, nil, FileXfrConfig{}, logger)

		testCases := []struct {
			name     string
			filename string
			expected string
		}{
			{"Normal filename", "test.txt", "test.txt"},
			{"Filename with spaces", "my file.pdf", "my_file.pdf"},
			{"Filename with special characters", "report_2023!@#.xlsx", "report_2023!@#.xlsx"},
			{"Long filename within default limit", "this_is_a_very_long_filename_that_exceeds_the_previous_100_character_limit_but_is_still_within_the_new_500_character_default_limit_for_object_ids.docx", "this_is_a_very_long_filename_that_exceeds_the_previous_100_character_limit_but_is_still_within_the_new_500_character_default_limit_for_object_ids.docx"},
			{"Filename with non-ASCII characters", "कर्मचारी.txt", "कर्मचारी.txt"},
			{"Empty filename", "", ""},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				objectID := fxs.generateObjectID(tc.filename)

				assert.Equal(t, tc.expected, objectID, "Object ID should match expected sanitized filename")
				assert.LessOrEqual(t, len(objectID), 500, "Object ID should not exceed default max length")

				// Generate another ID with the same filename - should be identical since no randomization
				anotherObjectID := fxs.generateObjectID(tc.filename)
				assert.Equal(t, objectID, anotherObjectID, "Generated object IDs should be identical for same input")
			})
		}
	})

	t.Run("Custom short limit", func(t *testing.T) {
		// Test with custom shorter limit to verify truncation still works
		config := FileXfrConfig{MaxObjectIDLength: 50}
		fxs := NewFileXfrServer(&jobs.JobManager{}, nil, nil, config, logger)

		longFilename := "this_is_a_very_long_filename_that_exceeds_the_custom_limit.docx"
		expectedTruncated := "this_is_a_very_long_filename_that_exceeds_the_cust"

		objectID := fxs.generateObjectID(longFilename)
		assert.Equal(t, expectedTruncated, objectID, "Object ID should be truncated to custom limit")
		assert.Equal(t, 50, len(objectID), "Object ID should be exactly 50 characters")
	})
}
