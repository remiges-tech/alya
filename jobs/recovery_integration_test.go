package jobs

import (
	"context"
	"fmt"
	"log"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/remiges-tech/logharbour/logharbour"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

// TestRecoverAbandonedRows_ResetsRowsInDB verifies the full DB recovery path:
// rows left as 'inprog' by a dead worker are reset to 'queued', and the dead
// worker's Redis keys are cleaned up.
func TestRecoverAbandonedRows_ResetsRowsInDB(t *testing.T) {
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

	// Seed a batch with status 'inprog'
	batchID := uuid.New()
	_, err = pool.Exec(ctx,
		`INSERT INTO batches (id, app, op, context, status, reqat)
		 VALUES ($1, 'testapp', 'testop', '{}', 'inprog', NOW())`,
		batchID)
	require.NoError(t, err)

	// Row 1: already completed (success, doneat set)
	_, err = pool.Exec(ctx,
		`INSERT INTO batchrows (batch, line, input, status, reqat, doneat, blobrows)
		 VALUES ($1, 1, '{}', 'success', NOW(), NOW(), '{}')`,
		batchID)
	require.NoError(t, err)

	// Row 2: abandoned by dead worker (inprog, doneat NULL)
	_, err = pool.Exec(ctx,
		`INSERT INTO batchrows (batch, line, input, status, reqat, blobrows)
		 VALUES ($1, 2, '{}', 'inprog', NOW(), '{}')`,
		batchID)
	require.NoError(t, err)

	// Row 3: abandoned by dead worker (inprog, doneat NULL)
	_, err = pool.Exec(ctx,
		`INSERT INTO batchrows (batch, line, input, status, reqat, blobrows)
		 VALUES ($1, 3, '{}', 'inprog', NOW(), '{}')`,
		batchID)
	require.NoError(t, err)

	// Query back rowids for the two abandoned rows
	var rowID2, rowID3 int64
	err = pool.QueryRow(ctx,
		`SELECT rowid FROM batchrows WHERE batch = $1 AND line = 2`, batchID).Scan(&rowID2)
	require.NoError(t, err)
	err = pool.QueryRow(ctx,
		`SELECT rowid FROM batchrows WHERE batch = $1 AND line = 3`, batchID).Scan(&rowID3)
	require.NoError(t, err)

	// Seed Redis to simulate a dead worker that was processing rows 2 and 3
	deadInstanceID := "dead-worker-abc"
	redisClient.SAdd(ctx, WorkerRegistryKey(), deadInstanceID)
	redisClient.SAdd(ctx, WorkerRowsKey(deadInstanceID),
		fmt.Sprintf("%d", rowID2),
		fmt.Sprintf("%d", rowID3))
	// No heartbeat key -- simulates expired TTL

	// Register the live JobManager so it's in the registry with a heartbeat
	err = jm.RegisterWorker(ctx)
	require.NoError(t, err)
	err = jm.RefreshHeartbeat(ctx)
	require.NoError(t, err)

	// Run recovery
	recovered, err := jm.RecoverAbandonedRows(ctx)
	require.NoError(t, err)
	assert.Equal(t, 2, recovered)

	// Verify DB: abandoned rows reset to 'queued'
	var status2, status3 string
	err = pool.QueryRow(ctx,
		`SELECT status::text FROM batchrows WHERE rowid = $1`, rowID2).Scan(&status2)
	require.NoError(t, err)
	assert.Equal(t, "queued", status2, "row 2 should be reset to queued")

	err = pool.QueryRow(ctx,
		`SELECT status::text FROM batchrows WHERE rowid = $1`, rowID3).Scan(&status3)
	require.NoError(t, err)
	assert.Equal(t, "queued", status3, "row 3 should be reset to queued")

	// Verify DB: completed row untouched
	var status1 string
	err = pool.QueryRow(ctx,
		`SELECT status::text FROM batchrows WHERE batch = $1 AND line = 1`, batchID).Scan(&status1)
	require.NoError(t, err)
	assert.Equal(t, "success", status1, "row 1 should remain success")

	// Verify DB: batch status untouched
	var batchStatus string
	err = pool.QueryRow(ctx,
		`SELECT status::text FROM batches WHERE id = $1`, batchID).Scan(&batchStatus)
	require.NoError(t, err)
	assert.Equal(t, "inprog", batchStatus, "batch should remain inprog")

	// Verify Redis: dead worker's rows SET deleted
	exists, err := redisClient.Exists(ctx, WorkerRowsKey(deadInstanceID)).Result()
	require.NoError(t, err)
	assert.Equal(t, int64(0), exists, "dead worker's rows key should be deleted")

	// Verify Redis: dead worker removed from registry
	isMember, err := redisClient.SIsMember(ctx, WorkerRegistryKey(), deadInstanceID).Result()
	require.NoError(t, err)
	assert.False(t, isMember, "dead worker should be removed from registry")

	// Verify Redis: live worker still in registry
	isMember, err = redisClient.SIsMember(ctx, WorkerRegistryKey(), jm.InstanceID()).Result()
	require.NoError(t, err)
	assert.True(t, isMember, "live worker should remain in registry")
}
