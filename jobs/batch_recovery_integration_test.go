package jobs

import (
	"context"
	"fmt"
	"io"
	"log"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/go-redis/redis/v8"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/remiges-tech/alya/jobs/objstore"
	"github.com/remiges-tech/alya/jobs/pg/batchsqlc"
	"github.com/remiges-tech/alya/wscutils"
	"github.com/remiges-tech/logharbour/logharbour"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

type recoveryTestInitBlock struct{}

func (b *recoveryTestInitBlock) Close() error { return nil }

type recoveryTestInitializer struct{}

func (i *recoveryTestInitializer) Init(app string) (InitBlock, error) {
	return &recoveryTestInitBlock{}, nil
}

// recoveryTestProcessor sleeps 50ms per row and returns success.
type recoveryTestProcessor struct{}

func (p *recoveryTestProcessor) DoBatchJob(
	initBlock InitBlock,
	batchctx JSONstr,
	line int,
	input JSONstr,
) (batchsqlc.StatusEnum, JSONstr, []wscutils.ErrorMessage, map[string]string, error) {
	time.Sleep(50 * time.Millisecond)
	result, _ := NewJSONstr("{}")
	return batchsqlc.StatusEnumSuccess, result, nil, nil, nil
}

func (p *recoveryTestProcessor) MarkDone(
	initBlock InitBlock,
	batchctx JSONstr,
	details BatchDetails_t,
) error {
	return nil
}

// TestBatchRecovery_SubmitCrashRecover verifies the end-to-end recovery flow:
// submit a batch, simulate a crashed worker that left rows in 'inprog', start a
// live worker that recovers those rows and processes the full batch to completion.
func TestBatchRecovery_SubmitCrashRecover(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test requiring Docker")
	}

	const (
		numRows      = 10
		appName      = "recoveryapp"
		opName       = "recoverop"
		pollInterval = 100 * time.Millisecond
		timeout      = 30 * time.Second
	)

	ctx := context.Background()

	pgContainer, err := postgres.Run(ctx,
		"postgres:16-alpine",
		postgres.WithDatabase("testdb"),
		postgres.WithUsername("test"),
		postgres.WithPassword("test"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(30*time.Second),
		),
	)
	require.NoError(t, err)
	defer pgContainer.Terminate(ctx)

	connStr, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)

	conn, err := pgx.Connect(ctx, connStr)
	require.NoError(t, err)
	err = MigrateDatabase(conn)
	require.NoError(t, err)
	conn.Close(ctx)

	pool, err := pgxpool.New(ctx, connStr)
	require.NoError(t, err)
	defer pool.Close()

	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()

	redisClient := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer redisClient.Close()

	loggerCtx := &logharbour.LoggerContext{}
	logger := logharbour.NewLogger(loggerCtx, "test", log.Writer())

	jm := NewJobManager(pool, redisClient, nil, logger, nil)
	jm.objStore = &objstore.ObjectStoreMock{
		PutFunc: func(ctx context.Context, bucket, obj string, reader io.Reader, size int64, contentType string) error {
			return nil
		},
	}

	err = jm.RegisterInitializer(appName, &recoveryTestInitializer{})
	require.NoError(t, err)
	err = jm.RegisterProcessorBatch(appName, opName, &recoveryTestProcessor{})
	require.NoError(t, err)

	// Submit a batch of 10 rows
	batchContext, err := NewJSONstr("{}")
	require.NoError(t, err)

	inputs := make([]BatchInput_t, numRows)
	for i := range numRows {
		input, jsonErr := NewJSONstr(fmt.Sprintf(`{"row": %d}`, i+1))
		require.NoError(t, jsonErr)
		inputs[i] = BatchInput_t{Line: i + 1, Input: input}
	}

	batchID, err := jm.BatchSubmit(appName, opName, batchContext, inputs, false)
	require.NoError(t, err)
	t.Logf("submitted batch: %s", batchID)

	// Simulate a crashed worker: mark 3 rows as 'inprog' directly in DB,
	// register a fake dead worker in Redis with those row IDs, and do NOT
	// set a heartbeat key (simulates expired TTL).
	deadWorkerID := "dead-worker-crash-sim"

	// Pick 3 rows and flip them to 'inprog'
	rows, err := pool.Query(ctx,
		`SELECT rowid FROM batchrows WHERE batch = $1 ORDER BY line LIMIT 3`, batchID)
	require.NoError(t, err)

	var crashedRowIDs []int64
	for rows.Next() {
		var rowID int64
		require.NoError(t, rows.Scan(&rowID))
		crashedRowIDs = append(crashedRowIDs, rowID)
	}
	rows.Close()
	require.Len(t, crashedRowIDs, 3)

	_, err = pool.Exec(ctx,
		`UPDATE batchrows SET status = 'inprog' WHERE rowid = ANY($1)`, crashedRowIDs)
	require.NoError(t, err)

	// Also set the batch to 'inprog' since some rows are now being processed
	_, err = pool.Exec(ctx,
		`UPDATE batches SET status = 'inprog' WHERE id = $1`, batchID)
	require.NoError(t, err)

	// Seed Redis: dead worker in registry with row IDs, no heartbeat
	redisClient.SAdd(ctx, workerRegistryKey(), deadWorkerID)
	rowIDArgs := make([]any, len(crashedRowIDs))
	for i, id := range crashedRowIDs {
		rowIDArgs[i] = fmt.Sprintf("%d", id)
	}
	redisClient.SAdd(ctx, workerRowsKey(deadWorkerID), rowIDArgs...)

	t.Logf("simulated dead worker %s with %d stuck rows", deadWorkerID, len(crashedRowIDs))

	// Start a live worker via RunWithContext. runPeriodicRecovery does an
	// immediate recovery on startup, which detects the dead worker and resets
	// the 3 rows to 'queued'. The main loop then processes all 10 rows.
	workerCtx, cancel := context.WithCancel(ctx)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		jm.RunWithContext(workerCtx)
	}()

	// Poll until batch completes
	deadline := time.After(timeout)
	done := false
	for !done {
		select {
		case <-deadline:
			cancel()
			wg.Wait()
			t.Fatal("timed out waiting for batch to complete")
		case <-time.After(pollInterval):
			status, _, _, _, _, _, pollErr := jm.BatchDone(batchID)
			require.NoError(t, pollErr)
			if status != batchsqlc.StatusEnumQueued && status != batchsqlc.StatusEnumInprog {
				done = true
			}
		}
	}

	cancel()
	wg.Wait()

	// Verify batch completed with all rows successful
	status, _, _, nsuccess, nfailed, naborted, err := jm.BatchDone(batchID)
	require.NoError(t, err)
	assert.Equal(t, batchsqlc.StatusEnumSuccess, status, "batch should complete as success")
	assert.Equal(t, numRows, nsuccess, "all rows should succeed")
	assert.Equal(t, 0, nfailed, "no rows should fail")
	assert.Equal(t, 0, naborted, "no rows should be aborted")

	// No stuck rows in DB
	var stuckCount int
	err = pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM batchrows WHERE status IN ('queued', 'inprog')`).Scan(&stuckCount)
	require.NoError(t, err)
	assert.Equal(t, 0, stuckCount, "no rows should be stuck in queued or inprog")

	// Dead worker removed from Redis registry
	isMember, err := redisClient.SIsMember(ctx, workerRegistryKey(), deadWorkerID).Result()
	require.NoError(t, err)
	assert.False(t, isMember, "dead worker should be removed from registry")

	// Dead worker's rows SET deleted from Redis
	exists, err := redisClient.Exists(ctx, workerRowsKey(deadWorkerID)).Result()
	require.NoError(t, err)
	assert.Equal(t, int64(0), exists, "dead worker's rows key should be deleted")

	// Live worker still in registry
	isMember, err = redisClient.SIsMember(ctx, workerRegistryKey(), jm.instanceID).Result()
	require.NoError(t, err)
	assert.True(t, isMember, "live worker should remain in registry")
}
