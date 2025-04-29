package jobs

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/remiges-tech/alya/jobs/pg/batchsqlc"
	"github.com/remiges-tech/alya/wscutils"
	"github.com/remiges-tech/logharbour/logharbour"
	"github.com/stretchr/testify/assert"
)

/**
 * TestProcessorNotFoundErrorHandling is an integration test that validates the error handling mechanism
 * for missing processor scenarios. The test:
 *
 * 1. Sets up a JobManager with only an initializer (but no processor) for a test app
 * 2. Creates a batch with multiple rows, all requiring an unregistered processor
 * 3. Uses the helper method singleRunIteration() to process the batch once
 * 4. Verifies that all rows in the batch are updated to 'failed' status
 * 5. Checks that the batch status is updated to 'failed'
 * 6. Confirms that proper error messages are stored in the database with correct codes
 *
 * This test verifies the error handling logic by directly using the internal helper
 * method that executes a single iteration of the processing loop.
 */

func TestProcessorNotFoundErrorHandling(t *testing.T) {
	// Setup test database
	db := getDb()
	defer db.Close()

	// Ensure migrations are applied
	conn, err := db.Acquire(context.Background())
	assert.NoError(t, err)
	defer conn.Release()
	err = MigrateDatabase(conn.Conn())
	assert.NoError(t, err)

	// Create Redis client
	redisClient := getRedisClient()
	defer redisClient.Close()

	// Create a real logger for testing
	logger := logharbour.NewLogger(&logharbour.LoggerContext{}, "jobmanager-test", os.Stdout)

	// Create JobManager with configuration for testing
	jm := NewJobManager(db, redisClient, nil, logger, &JobManagerConfig{
		BatchChunkNRows: 5,
	})

	// Register initializer only (no processor)
	err = jm.RegisterInitializer("testapp", &MockInitializer{})
	assert.NoError(t, err)

	// Create test data
	// 1. Batch with multiple rows, all using the same unregistered processor
	appName := "testapp"
	opName := "unregistered_op"
	batchContext, err := NewJSONstr("{}")
	assert.NoError(t, err)

	// Create multiple rows in the batch
	batchInputs := []BatchInput_t{}
	for i := 1; i <= 3; i++ {
		input, err := NewJSONstr(fmt.Sprintf(`{"data": "test-%d"}`, i))
		assert.NoError(t, err)
		batchInputs = append(batchInputs, BatchInput_t{Line: i, Input: input})
	}

	// Submit batch
	batchID, err := jm.BatchSubmit(appName, opName, batchContext, batchInputs, false)
	assert.NoError(t, err)

	// Convert string batchID to UUID
	batchUUID, err := uuid.Parse(batchID)
	assert.NoError(t, err)

	// Run a single iteration of the job processing loop
	jm.RunOneIteration()

	// Small delay to ensure database operations are complete
	time.Sleep(100 * time.Millisecond)

	// Verify results:
	// 1. All rows in the batch should be marked as failed
	dbRows, err := db.Query(context.Background(),
		"SELECT status FROM batchrows WHERE batch = $1", batchUUID)
	assert.NoError(t, err)
	defer dbRows.Close()

	rowCount := 0
	for dbRows.Next() {
		var status string
		err := dbRows.Scan(&status)
		assert.NoError(t, err)
		assert.Equal(t, string(batchsqlc.StatusEnumFailed), status)
		rowCount++
	}
	assert.Equal(t, 3, rowCount) // All three rows should be found and marked failed

	// 2. The batch status should be failed
	var batchStatus string
	err = db.QueryRow(context.Background(),
		"SELECT status FROM batches WHERE id = $1", batchUUID).Scan(&batchStatus)
	assert.NoError(t, err)
	assert.Equal(t, string(batchsqlc.StatusEnumFailed), batchStatus)

	// 3. Verify error messages have been set correctly
	var messages []byte
	err = db.QueryRow(context.Background(),
		"SELECT messages FROM batchrows WHERE batch = $1 LIMIT 1", batchUUID).Scan(&messages)
	assert.NoError(t, err)

	var errorMessages []wscutils.ErrorMessage
	err = json.Unmarshal(messages, &errorMessages)
	assert.NoError(t, err)
	assert.Len(t, errorMessages, 1)
	// Verify error code and message ID match our new constants
	assert.Equal(t, ErrCodeConfiguration, errorMessages[0].ErrCode, "Error code should be ErrCodeConfiguration")
	assert.Equal(t, MsgIDProcessorNotFound, errorMessages[0].MsgID, "Message ID should be MsgIDProcessorNotFound")

	// Check that the error message contains the expected text
	var errMsgJSON string
	err = db.QueryRow(context.Background(),
		"SELECT messages::text FROM batchrows WHERE batch = $1 LIMIT 1", batchUUID).Scan(&errMsgJSON)
	assert.NoError(t, err)
	assert.Contains(t, errMsgJSON, "no BatchProcessor registered")
}

/**
 * TestJobManagerWithMissingProcessor is an integration test that simulates the full operation
 * of the JobManager when it encounters a missing processor. The test:
 *
 * 1. Sets up a JobManager with only an initializer for a test app (no processor)
 * 2. Creates a batch with a single row requiring this unregistered processor
 * 3. Runs the JobManager in a controlled way by calling RunOneIteration()
 * 4. Uses a monitoring goroutine to detect when the batch is marked as failed
 * 5. Terminates the test once the batch is properly handled or a timeout occurs
 * 6. Verifies that the batch and row statuses are correctly set to 'failed'
 *
 * This test validates the end-to-end behavior, ensuring that when JobManager encounters
 * a missing processor in its normal operation cycle, it properly handles the failure case
 * by updating the batch and row statuses appropriately.
 */
func TestJobManagerWithMissingProcessor(t *testing.T) {
	// Only run this test if integration tests are enabled
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Setup test database
	db := getDb()
	defer db.Close()

	// Create Redis client
	redisClient := getRedisClient()
	defer redisClient.Close()

	// Create JobManager
	jm := NewJobManager(db, redisClient, nil, nil, nil)

	// Register initializer only (no processor)
	err := jm.RegisterInitializer("testapp", &MockInitializer{})
	assert.NoError(t, err)

	// Create test data with an unregistered processor
	appName := "testapp"
	opName := "unregistered_op"
	batchContext, err := NewJSONstr("{}")
	assert.NoError(t, err)

	input, err := NewJSONstr(`{"data": "test"}`)
	assert.NoError(t, err)
	batchInputs := []BatchInput_t{
		{Line: 1, Input: input},
	}

	// Submit batch
	batchID, err := jm.BatchSubmit(appName, opName, batchContext, batchInputs, false)
	assert.NoError(t, err)

	// Run JobManager in a goroutine with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan struct{})
	go func() {
		defer close(done)

		// Create a monitoring goroutine
		go func() {
			ticker := time.NewTicker(100 * time.Millisecond)
			defer ticker.Stop()

			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					// Check if our batch has been processed
					var status string
					err := db.QueryRow(context.Background(),
						"SELECT status FROM batches WHERE id = $1", batchID).Scan(&status)

					if err == nil && status == string(batchsqlc.StatusEnumFailed) {
						// Batch has been properly marked as failed, we're done
						cancel()
						return
					}
				}
			}
		}()

		// Run JobManager with context until canceled
		go func() {
			// Use the new context-aware RunWithContext method
			jm.RunWithContext(ctx)
		}()

		<-ctx.Done()
	}()

	// Wait for completion or timeout
	select {
	case <-done:
		// Success - completed within timeout
	case <-time.After(5 * time.Second):
		t.Fatal("Test timed out")
	}

	// Verify the batch and its rows are marked as failed
	var batchStatus string
	err = db.QueryRow(context.Background(),
		"SELECT status FROM batches WHERE id = $1", batchID).Scan(&batchStatus)
	assert.NoError(t, err)
	assert.Equal(t, string(batchsqlc.StatusEnumFailed), batchStatus)

	// Check row status
	var rowStatus string
	err = db.QueryRow(context.Background(),
		"SELECT status FROM batchrows WHERE batch = $1", batchID).Scan(&rowStatus)
	assert.NoError(t, err)
	assert.Equal(t, string(batchsqlc.StatusEnumFailed), rowStatus)
}
