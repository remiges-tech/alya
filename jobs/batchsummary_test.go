package jobs

import (
	"os"
	"testing"

	"github.com/remiges-tech/alya/jobs/pg/batchsqlc"
	"github.com/stretchr/testify/require"
)

// TestBatchProcessingLogicalFiles tests the batch processing functionality
// with multiple logical files generated from the blobrows data of individual
// batch rows. It simulates a scenario where a batch job processes financial
// transactions and generates two logical files:
//
//  1. transaction_summary.txt: Contains a summary line for each processed
//     transaction, with details like transaction ID, type, amount, and balance.
//
//  2. error_log.txt: Contains error messages or logs generated during the
//     processing of individual transactions.
func TestBatchProcessingLogicalFiles(t *testing.T) {
	// Prepare test data
	batchRows := []batchsqlc.GetProcessedBatchRowsByBatchIDSortedRow{
		{
			Rowid:    1,
			Line:     1,
			Blobrows: []byte(`{"transaction_summary.txt": "TX001,DEPOSIT,1000.00,5000.00", "error_log.txt": ""}`),
		},
		{
			Rowid:    2,
			Line:     2,
			Blobrows: []byte(`{"transaction_summary.txt": "TX002,WITHDRAWAL,500.00,4500.00"}`),
		},
		{
			Rowid:    3,
			Line:     3,
			Blobrows: []byte(`{"transaction_summary.txt": "TX003,WITHDRAWAL,20000.00,4500.00", "error_log.txt": "Invalid transaction amount"}`),
		},
	}

	// Call the function under test
	tmpFiles, err := createTemporaryFiles(batchRows)
	require.NoError(t, err)
	defer cleanupTemporaryFiles(tmpFiles)

	// Verify that the correct temporary files were created
	require.Len(t, tmpFiles, 2)

	// Check transaction_summary.txt contents
	summaryFile, ok := tmpFiles["transaction_summary.txt"]
	require.True(t, ok)
	appendBlobRowsToFiles(batchRows, tmpFiles)
	summaryContents, err := os.ReadFile(summaryFile.Name())
	require.NoError(t, err)
	expectedSummary := "TX001,DEPOSIT,1000.00,5000.00\nTX002,WITHDRAWAL,500.00,4500.00\nTX003,WITHDRAWAL,20000.00,4500.00\n"
	require.Equal(t, expectedSummary, string(summaryContents))

	// Check error_log.txt contents
	errorLogFile, ok := tmpFiles["error_log.txt"]
	require.True(t, ok)
	errorLogContents, err := os.ReadFile(errorLogFile.Name())
	require.NoError(t, err)
	expectedErrorLog := "Invalid transaction amount\n"
	require.Equal(t, expectedErrorLog, string(errorLogContents))
}
