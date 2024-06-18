package jobs

import (
	"context"
	"fmt"
	"io"
	"testing"
	"time"

	"github.com/go-redis/redismock/v8"
	"github.com/google/uuid"
	"github.com/remiges-tech/alya/jobs/objstore"
	"github.com/remiges-tech/alya/jobs/pg/batchsqlc"
	"github.com/remiges-tech/alya/jobs/pg/batchsqlc/mocks"
	"github.com/stretchr/testify/assert"
)

// Define the test cases
var summarizeBatchTests = []struct {
	name           string
	batchID        uuid.UUID
	batchRows      []batchsqlc.GetBatchRowsByBatchIDSortedRow
	processedRows  []batchsqlc.GetProcessedBatchRowsByBatchIDSortedRow
	expectedStatus batchsqlc.StatusEnum
	expectedCounts struct {
		success int64
		failed  int64
		aborted int64
	}
}{
	{
		name:    "Successful Summary",
		batchID: uuid.New(),
		batchRows: []batchsqlc.GetBatchRowsByBatchIDSortedRow{
			{Status: batchsqlc.StatusEnumSuccess, Blobrows: []byte(`{"transaction_summary.txt": "TX001,DEPOSIT,1000.00,5000.00", "error_log.txt": ""}`)},
			{Status: batchsqlc.StatusEnumSuccess, Blobrows: []byte(`{"transaction_summary.txt": "TX002,WITHDRAWAL,500.00,4500.00", "error_log.txt": ""}`)},
			{Status: batchsqlc.StatusEnumSuccess, Blobrows: []byte(`{"transaction_summary.txt": "TX003,TRANSFER,2000.00,2500.00", "error_log.txt": ""}`)},
		},
		processedRows: []batchsqlc.GetProcessedBatchRowsByBatchIDSortedRow{
			{Status: batchsqlc.StatusEnumSuccess, Blobrows: []byte(`{"transaction_summary.txt": "TX001,DEPOSIT,1000.00,5000.00", "error_log.txt": ""}`)},
			{Status: batchsqlc.StatusEnumSuccess, Blobrows: []byte(`{"transaction_summary.txt": "TX002,WITHDRAWAL,500.00,4500.00", "error_log.txt": ""}`)},
			{Status: batchsqlc.StatusEnumSuccess, Blobrows: []byte(`{"transaction_summary.txt": "TX003,TRANSFER,2000.00,2500.00", "error_log.txt": ""}`)},
		},
		expectedStatus: batchsqlc.StatusEnumSuccess,
		expectedCounts: struct {
			success int64
			failed  int64
			aborted int64
		}{
			success: 3,
			failed:  0,
			aborted: 0,
		},
	},
	{
		name:    "Failed Summary",
		batchID: uuid.New(),
		batchRows: []batchsqlc.GetBatchRowsByBatchIDSortedRow{
			{Status: batchsqlc.StatusEnumSuccess, Blobrows: []byte(`{"transaction_summary.txt": "TX001,DEPOSIT,1000.00,5000.00", "error_log.txt": ""}`)},
			{Status: batchsqlc.StatusEnumFailed, Blobrows: []byte(`{"transaction_summary.txt": "", "error_log.txt": "ERROR: Insufficient funds for TX002"}`)},
			{Status: batchsqlc.StatusEnumSuccess, Blobrows: []byte(`{"transaction_summary.txt": "TX003,TRANSFER,2000.00,2500.00", "error_log.txt": ""}`)},
			{Status: batchsqlc.StatusEnumFailed, Blobrows: []byte(`{"transaction_summary.txt": "", "error_log.txt": "ERROR: Invalid account number for TX004"}`)},
		},
		processedRows: []batchsqlc.GetProcessedBatchRowsByBatchIDSortedRow{
			{Status: batchsqlc.StatusEnumSuccess, Blobrows: []byte(`{"transaction_summary.txt": "TX001,DEPOSIT,1000.00,5000.00", "error_log.txt": ""}`)},
			{Status: batchsqlc.StatusEnumFailed, Blobrows: []byte(`{"transaction_summary.txt": "", "error_log.txt": "ERROR: Insufficient funds for TX002"}`)},
			{Status: batchsqlc.StatusEnumSuccess, Blobrows: []byte(`{"transaction_summary.txt": "TX003,TRANSFER,2000.00,2500.00", "error_log.txt": ""}`)},
			{Status: batchsqlc.StatusEnumFailed, Blobrows: []byte(`{"transaction_summary.txt": "", "error_log.txt": "ERROR: Invalid account number for TX004"}`)},
		},
		expectedStatus: batchsqlc.StatusEnumFailed,
		expectedCounts: struct {
			success int64
			failed  int64
			aborted int64
		}{
			success: 2,
			failed:  2,
			aborted: 0,
		},
	},
	{
		name:    "Aborted Summary",
		batchID: uuid.New(),
		batchRows: []batchsqlc.GetBatchRowsByBatchIDSortedRow{
			{Status: batchsqlc.StatusEnumSuccess, Blobrows: []byte(`{"transaction_summary.txt": "TX001,DEPOSIT,1000.00,5000.00", "error_log.txt": ""}`)},
			{Status: batchsqlc.StatusEnumAborted, Blobrows: []byte(`{"transaction_summary.txt": "", "error_log.txt": "ABORT: User canceled transaction TX002"}`)},
			{Status: batchsqlc.StatusEnumSuccess, Blobrows: []byte(`{"transaction_summary.txt": "TX003,TRANSFER,2000.00,2500.00", "error_log.txt": ""}`)},
			{Status: batchsqlc.StatusEnumAborted, Blobrows: []byte(`{"transaction_summary.txt": "", "error_log.txt": "ABORT: System maintenance, transaction TX004 aborted"}`)},
		},
		processedRows: []batchsqlc.GetProcessedBatchRowsByBatchIDSortedRow{
			{Status: batchsqlc.StatusEnumSuccess, Blobrows: []byte(`{"transaction_summary.txt": "TX001,DEPOSIT,1000.00,5000.00", "error_log.txt": ""}`)},
			{Status: batchsqlc.StatusEnumSuccess, Blobrows: []byte(`{"transaction_summary.txt": "TX003,TRANSFER,2000.00,2500.00", "error_log.txt": ""}`)},
		},
		expectedStatus: batchsqlc.StatusEnumAborted,
		expectedCounts: struct {
			success int64
			failed  int64
			aborted int64
		}{
			success: 2,
			failed:  0,
			aborted: 2,
		},
	},
}

func TestSummarizeBatch(t *testing.T) {
	for _, tt := range summarizeBatchTests {
		t.Run(tt.name, func(t *testing.T) {
			// generated using moq
			// $ cd batch/pg/batchsqlc
			// $ moq -out mocks/querier_mock.go -pkg mocks . Querier
			mockQuerier := &mocks.QuerierMock{}
			mockQuerier.GetBatchByIDFunc = func(ctx context.Context, id uuid.UUID) (batchsqlc.GetBatchByIDRow, error) {
				return batchsqlc.GetBatchByIDRow{
					ID:     tt.batchID,
					Status: batchsqlc.StatusEnumQueued,
				}, nil
			}
			mockQuerier.CountBatchRowsByBatchIDAndStatusFunc = func(ctx context.Context, arg batchsqlc.CountBatchRowsByBatchIDAndStatusParams) (int64, error) {
				return int64(0), nil
			}
			mockQuerier.GetBatchRowsByBatchIDSortedFunc = func(ctx context.Context, batch uuid.UUID) ([]batchsqlc.GetBatchRowsByBatchIDSortedRow, error) {
				return tt.batchRows, nil
			}
			mockQuerier.GetProcessedBatchRowsByBatchIDSortedFunc = func(ctx context.Context, batch uuid.UUID) ([]batchsqlc.GetProcessedBatchRowsByBatchIDSortedRow, error) {
				return tt.processedRows, nil
			}
			mockQuerier.UpdateBatchSummaryFunc = func(ctx context.Context, arg batchsqlc.UpdateBatchSummaryParams) error {
				assert.Equal(t, tt.batchID, arg.ID)
				assert.Equal(t, tt.expectedStatus, arg.Status)
				assert.Equal(t, int32(tt.expectedCounts.success), arg.Nsuccess.Int32)
				assert.Equal(t, int32(tt.expectedCounts.failed), arg.Nfailed.Int32)
				assert.Equal(t, int32(tt.expectedCounts.aborted), arg.Naborted.Int32)
				return nil
			}

			mockObjStore := &objstore.ObjectStoreMock{}
			mockObjStore.PutFunc = func(ctx context.Context, bucket, obj string, reader io.Reader, size int64, contentType string) error {
				return nil
			}

			redisClient, redisMock := redismock.NewClientMock()
			redisMock.ExpectSet(fmt.Sprintf("ALYA_BATCHSTATUS_%s", tt.batchID.String()), string(tt.expectedStatus), time.Duration(ALYA_BATCHSTATUS_CACHEDUR_SEC*100)*time.Second).SetVal("OK")

			jm := JobManager{
				Queries:     mockQuerier,
				ObjStore:    mockObjStore,
				RedisClient: redisClient,
				// Initialize other required fields for the test
			}

			err := jm.summarizeBatch(mockQuerier, tt.batchID)
			assert.NoError(t, err)
		})
	}
}
