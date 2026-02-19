package jobs

import (
	"context"
	"fmt"
	"io"
	"log"
	"sync"
	"sync/atomic"
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

// noopInitBlock holds no resources. The original test_multi_batch.go used a real
// Redis client here, and Close() called redisClient.Close(). Because all workers
// shared one Redis client, closing it from one goroutine caused "redis: client is
// closed" errors in others -- the source of the ~4% row failure rate. A noop
// InitBlock avoids this entirely and isolates the test to framework behavior.
type noopInitBlock struct{}

func (b *noopInitBlock) Close() error { return nil }

type noopInitializer struct{}

func (i *noopInitializer) Init(app string) (InitBlock, error) {
	return &noopInitBlock{}, nil
}

// trackingBatchProcessor always returns success and tracks peak concurrency.
// A 1ms sleep per row ensures that with 300 rows and 3 workers, invocations
// overlap -- proving multiple workers processed rows simultaneously. Without
// the sleep, a noop processor is so fast that one worker can drain the entire
// queue before others start their first poll cycle.
type trackingBatchProcessor struct {
	active  atomic.Int64
	peakHit atomic.Int64
}

func (p *trackingBatchProcessor) DoBatchJob(
	initBlock InitBlock,
	batchctx JSONstr,
	line int,
	input JSONstr,
) (batchsqlc.StatusEnum, JSONstr, []wscutils.ErrorMessage, map[string]string, error) {
	cur := p.active.Add(1)
	if cur > p.peakHit.Load() {
		p.peakHit.Store(cur)
	}
	time.Sleep(1 * time.Millisecond)
	p.active.Add(-1)

	result, _ := NewJSONstr("{}")
	return batchsqlc.StatusEnumSuccess, result, nil, nil, nil
}

func (p *trackingBatchProcessor) MarkDone(
	initBlock InitBlock,
	batchctx JSONstr,
	details BatchDetails_t,
) error {
	return nil
}

// TestMultiWorkerBatchProcessing verifies that multiple workers can process
// multiple batches concurrently without row failures or stuck rows.
func TestMultiWorkerBatchProcessing(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test requiring Docker")
	}

	// Scale: the original test_multi_batch.go used 10 workers, 10 batches, 100k
	// rows. That takes minutes and is unsuitable for CI. 3x3x100 = 300 rows is
	// enough to exercise concurrent worker contention while finishing in ~2s.
	const (
		numWorkers     = 3
		numBatches     = 3
		rowsPerBatch   = 100
		appName        = "testapp"
		opName         = "testop"
		pollInterval   = 100 * time.Millisecond
		overallTimeout = 30 * time.Second
	)

	ctx := context.Background()

	// Start PostgreSQL testcontainer
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

	// Run migrations
	conn, err := pgx.Connect(ctx, connStr)
	require.NoError(t, err)
	err = MigrateDatabase(conn)
	require.NoError(t, err)
	conn.Close(ctx)

	pool, err := pgxpool.New(ctx, connStr)
	require.NoError(t, err)
	defer pool.Close()

	// miniredis runs in-process -- no container startup cost. The framework uses
	// Redis for batch status caching, not for row processing. An in-process
	// server is sufficient and keeps the test fast (~0ms vs ~2s for a container).
	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()

	redisClient := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer redisClient.Close()

	loggerCtx := &logharbour.LoggerContext{}
	logger := logharbour.NewLogger(loggerCtx, "test", log.Writer())

	// minioClient is nil because we mock the object store directly. The mock Put
	// discards data -- we only care that summarization calls it without error, not
	// that blobs land in real storage.
	jm := NewJobManager(pool, redisClient, nil, logger, nil)
	jm.objStore = &objstore.ObjectStoreMock{
		PutFunc: func(ctx context.Context, bucket, obj string, reader io.Reader, size int64, contentType string) error {
			return nil
		},
	}

	err = jm.RegisterInitializer(appName, &noopInitializer{})
	require.NoError(t, err)
	processor := &trackingBatchProcessor{}
	err = jm.RegisterProcessorBatch(appName, opName, processor)
	require.NoError(t, err)

	// All batches are submitted before workers start. This means all 300 rows are
	// queued upfront, so workers compete for rows from the start -- maximizing the
	// chance of exposing race conditions in row claiming or batch summarization.
	batchContext, err := NewJSONstr("{}")
	require.NoError(t, err)

	batchIDs := make([]string, numBatches)
	for b := 0; b < numBatches; b++ {
		inputs := make([]BatchInput_t, rowsPerBatch)
		for r := 0; r < rowsPerBatch; r++ {
			input, jsonErr := NewJSONstr(fmt.Sprintf(`{"row": %d}`, r+1))
			require.NoError(t, jsonErr)
			inputs[r] = BatchInput_t{Line: r + 1, Input: input}
		}

		batchID, submitErr := jm.BatchSubmit(appName, opName, batchContext, inputs, false)
		require.NoError(t, submitErr)
		batchIDs[b] = batchID
		t.Logf("submitted batch %d: %s", b+1, batchID)
	}

	// RunWithContext blocks until the context is cancelled. Each goroutine polls
	// for queued rows, claims them, and processes. With 3 workers and 3 batches,
	// rows from different batches can be processed concurrently by different
	// workers -- the scenario that triggers advisory lock contention during
	// summarization.
	workerCtx, cancel := context.WithCancel(ctx)
	var wg sync.WaitGroup
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			jm.RunWithContext(workerCtx)
		}()
	}

	// Poll via BatchDone rather than querying the DB directly. This exercises the
	// same code path the API uses. The 100ms interval keeps the test responsive
	// without busy-spinning. The 30s timeout is generous -- 300 noop rows typically
	// finish in under 2s.
	deadline := time.After(overallTimeout)
	allDone := false
	for !allDone {
		select {
		case <-deadline:
			cancel()
			wg.Wait()
			t.Fatal("timed out waiting for batches to complete")
		case <-time.After(pollInterval):
			allDone = true
			for _, id := range batchIDs {
				status, _, _, _, _, _, pollErr := jm.BatchDone(id)
				require.NoError(t, pollErr)
				if status == batchsqlc.StatusEnumQueued || status == batchsqlc.StatusEnumInprog {
					allDone = false
					break
				}
			}
		}
	}

	// Cancel stops the polling loops inside RunWithContext. wg.Wait ensures all
	// goroutines exit before we query the DB for final assertions.
	cancel()
	wg.Wait()

	// With a noop processor that always returns success, any non-zero failure
	// count indicates a framework bug (double-processing, lost rows, etc.).
	for i, id := range batchIDs {
		status, _, _, nsuccess, nfailed, naborted, doneErr := jm.BatchDone(id)
		require.NoError(t, doneErr, "batch %d (%s)", i+1, id)

		assert.Equal(t, batchsqlc.StatusEnumSuccess, status,
			"batch %d (%s): expected status success, got %s", i+1, id, status)
		assert.Equal(t, rowsPerBatch, nsuccess,
			"batch %d (%s): expected %d successful rows", i+1, id, rowsPerBatch)
		assert.Equal(t, 0, nfailed,
			"batch %d (%s): expected 0 failed rows", i+1, id)
		assert.Equal(t, 0, naborted,
			"batch %d (%s): expected 0 aborted rows", i+1, id)
	}

	// Direct DB query catches rows that BatchDone's counts might not reveal --
	// e.g., rows orphaned by a crashed worker or stuck due to a missed status
	// transition.
	var stuckCount int
	err = pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM batchrows WHERE status IN ('queued', 'inprog')`).Scan(&stuckCount)
	require.NoError(t, err)
	assert.Equal(t, 0, stuckCount, "no rows should be stuck in queued or inprog")

	// Peak concurrency > 1 proves multiple workers processed rows simultaneously.
	// Without this, the test could pass with a single worker doing all the work.
	peak := processor.peakHit.Load()
	t.Logf("peak concurrent DoBatchJob invocations: %d", peak)
	assert.Greater(t, peak, int64(1),
		"expected multiple workers processing rows concurrently, got peak=%d", peak)

	// doneat=NULL means summarization never ran for this batch. This catches the
	// bug where advisory lock contention causes summarizeCompletedBatches to skip
	// a batch permanently.
	for i, id := range batchIDs {
		var doneatNotNull bool
		err = pool.QueryRow(ctx,
			`SELECT doneat IS NOT NULL FROM batches WHERE id = $1`, id).Scan(&doneatNotNull)
		require.NoError(t, err)
		assert.True(t, doneatNotNull, "batch %d (%s): doneat should not be NULL", i+1, id)
	}
}
