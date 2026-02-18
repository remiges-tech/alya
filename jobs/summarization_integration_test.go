package jobs

import (
	"context"
	"io"
	"log"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/remiges-tech/alya/jobs/objstore"
	"github.com/remiges-tech/logharbour/logharbour"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

// TestSummarizeCompletedBatches_RetriesOnLockNotAcquired reproduces the race
// condition where one worker holds the advisory lock while another gives up
// immediately on ErrBatchLockNotAcquired. Without retry, the batch stays
// unsummarized permanently.
//
// Setup:
//   - PostgreSQL testcontainer with migrations applied
//   - Batch with 3 rows, all status=success, doneat=NOW()
//   - Advisory lock held from a separate connection for 100ms
//
// Expected behavior:
//   - Before fix: summarizeCompletedBatches breaks on lock contention, batch
//     stays inprog with doneat=NULL
//   - After fix: retries with 50ms delay, acquires lock after ~100ms, summarizes
func TestSummarizeCompletedBatches_RetriesOnLockNotAcquired(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test requiring Docker")
	}

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
	err = jm.RegisterInitializer("testapp", &mockInitializer{})
	require.NoError(t, err)

	batchID := uuid.New()
	_, err = pool.Exec(ctx,
		`INSERT INTO batches (id, app, op, context, status, reqat)
		 VALUES ($1, 'testapp', 'testop', '{}', 'inprog', NOW())`,
		batchID)
	require.NoError(t, err)

	for i := 1; i <= 3; i++ {
		_, err = pool.Exec(ctx,
			`INSERT INTO batchrows (batch, line, input, status, reqat, doneat, blobrows)
			 VALUES ($1, $2, '{}', 'success', NOW(), NOW(), '{}')`,
			batchID, i)
		require.NoError(t, err)
	}

	lockAcquired := make(chan struct{})
	lockDone := make(chan struct{})

	go func() {
		defer close(lockDone)

		lockConn, connErr := pgx.Connect(ctx, connStr)
		if connErr != nil {
			t.Errorf("lock goroutine: connect failed: %v", connErr)
			return
		}
		defer lockConn.Close(ctx)

		tx, txErr := lockConn.Begin(ctx)
		if txErr != nil {
			t.Errorf("lock goroutine: begin failed: %v", txErr)
			return
		}

		var locked bool
		txErr = tx.QueryRow(ctx,
			`SELECT pg_try_advisory_xact_lock(
				('x' || substr(md5($1::text), 1, 16))::bit(64)::bigint
			)`,
			batchID.String()).Scan(&locked)
		if txErr != nil {
			t.Errorf("lock goroutine: lock query failed: %v", txErr)
			tx.Rollback(ctx)
			return
		}
		if !locked {
			t.Errorf("lock goroutine: failed to acquire advisory lock")
			tx.Rollback(ctx)
			return
		}

		close(lockAcquired)
		time.Sleep(100 * time.Millisecond)
		tx.Commit(ctx)
	}()

	<-lockAcquired
	time.Sleep(10 * time.Millisecond)

	batchSet := map[uuid.UUID]bool{batchID: true}
	err = jm.summarizeCompletedBatches(ctx, batchSet)
	require.NoError(t, err)

	<-lockDone

	var status string
	var doneatNotNull bool
	var nsuccess int32
	err = pool.QueryRow(ctx,
		`SELECT status::text, doneat IS NOT NULL, COALESCE(nsuccess, 0)
		 FROM batches WHERE id = $1`,
		batchID).Scan(&status, &doneatNotNull, &nsuccess)
	require.NoError(t, err)

	assert.True(t, doneatNotNull, "doneat should not be NULL -- batch should be summarized")
	assert.Equal(t, "success", status)
	assert.Equal(t, int32(3), nsuccess)
}
