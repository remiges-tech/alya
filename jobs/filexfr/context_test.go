package filexfr

import (
	"testing"

	"github.com/remiges-tech/alya/jobs"
	"github.com/stretchr/testify/assert"
)

// testContextFileChk is a FileChk function that checks context passing
func testContextFileChk(fileContents string, fileName string, batchctx jobs.JSONstr) (bool, jobs.JSONstr, []jobs.BatchInput_t, string, string, string) {
	contextStr := batchctx.String()

	result := `{"input_context": ` + contextStr + `, "filename": "` + fileName + `"}`
	resultContext, _ := jobs.NewJSONstr(result)

	// Create batch input
	inputJSON, _ := jobs.NewJSONstr(`{"data": "test"}`)
	batchInput := []jobs.BatchInput_t{
		{Line: 1, Input: inputJSON},
	}

	return true, resultContext, batchInput, "test-app", "test-op", ""
}

func TestFileChkContextPassing(t *testing.T) {
	logger := setupTestLogger(t)
	fxs := NewFileXfrServer(&jobs.JobManager{}, nil, nil, FileXfrConfig{MaxObjectIDLength: 200}, logger)

	// Register test FileChk function
	err := fxs.RegisterFileChk("test-context", testContextFileChk)
	assert.NoError(t, err, "Failed to register test FileChk")

	inputBatchctx, _ := jobs.NewJSONstr(`{"test": "value", "user": "test-user"}`)

	// Test FileChk function directly without full jobManager setup
	fileContents := "test data"
	filename := "test.txt"

	// Get and call the registered FileChk function
	fileChkFn, exists := fxs.fileChkMap["test-context"]
	assert.True(t, exists, "FileChk function should exist after registration")

	success, resultContext, batchInput, app, op, _ := fileChkFn(fileContents, filename, inputBatchctx)

	assert.True(t, success, "FileChk should have returned success")
	assert.Equal(t, "test-app", app, "Expected correct app name")
	assert.Equal(t, "test-op", op, "Expected correct operation")
	assert.Len(t, batchInput, 1, "Expected 1 batch input")

	// Check context in result
	resultStr := resultContext.String()
	assert.Contains(t, resultStr, "test-user", "Result context should contain input context data")

	t.Logf("Input context: %s", inputBatchctx.String())
	t.Logf("Result context: %s", resultContext.String())
}
