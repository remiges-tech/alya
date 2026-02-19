package jobs

import (
	"context"
	"fmt"
	"io"
	"log"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/remiges-tech/alya/jobs/objstore"
	"github.com/remiges-tech/alya/jobs/pg/batchsqlc"
	"github.com/remiges-tech/logharbour/logharbour"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

func seedBatch(t *testing.T, ctx context.Context, q *batchsqlc.Queries, app, op string, batchStatus batchsqlc.StatusEnum, rowStatuses []batchsqlc.StatusEnum) uuid.UUID {
	t.Helper()
	now := time.Now()
	id := uuid.New()

	_, err := q.InsertIntoBatches(ctx, batchsqlc.InsertIntoBatchesParams{
		ID: id, App: app, Op: op, Context: []byte("{}"),
		Status: batchStatus, Reqat: pgtype.Timestamp{Time: now, Valid: true},
	})
	require.NoError(t, err)

	for i := range rowStatuses {
		err = q.InsertIntoBatchRows(ctx, batchsqlc.InsertIntoBatchRowsParams{
			Batch: id, Line: int32(i + 1),
			Input: []byte(fmt.Sprintf(`{"line":%d}`, i+1)),
			Reqat: pgtype.Timestamp{Time: now, Valid: true},
		})
		require.NoError(t, err)
	}

	rows, err := q.GetBatchRowsByBatchID(ctx, id)
	require.NoError(t, err)
	for i, row := range rows {
		err = q.UpdateBatchRowsBatchJob(ctx, batchsqlc.UpdateBatchRowsBatchJobParams{
			Rowid: row.Rowid, Status: rowStatuses[i],
			Doneat: pgtype.Timestamp{Time: now, Valid: rowStatuses[i] != batchsqlc.StatusEnumInprog},
			Res: []byte("{}"),
		})
		require.NoError(t, err)
	}

	return id
}

// TestSweepUnsummarizedBatches verifies that sweepUnsummarizedBatches finds
// batches stuck in 'inprog' with all rows in terminal status and summarizes
// them, while leaving legitimate in-progress and already-summarized batches
// untouched.
func TestSweepUnsummarizedBatches(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test requiring Docker")
	}

	ctx := context.Background()

	pgContainer, err := postgres.Run(ctx, "postgres:16-alpine",
		postgres.WithDatabase("testdb"),
		postgres.WithUsername("test"),
		postgres.WithPassword("test"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).WithStartupTimeout(30*time.Second),
		),
	)
	require.NoError(t, err)
	defer pgContainer.Terminate(ctx)

	connStr, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)

	conn, err := pgx.Connect(ctx, connStr)
	require.NoError(t, err)
	require.NoError(t, MigrateDatabase(conn))
	conn.Close(ctx)

	pool, err := pgxpool.New(ctx, connStr)
	require.NoError(t, err)
	defer pool.Close()

	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()

	redisClient := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer redisClient.Close()

	logger := logharbour.NewLogger(&logharbour.LoggerContext{}, "test", log.Writer())
	jm := NewJobManager(pool, redisClient, nil, logger, nil)
	jm.objStore = &objstore.ObjectStoreMock{
		PutFunc: func(ctx context.Context, bucket, obj string, reader io.Reader, size int64, contentType string) error {
			return nil
		},
	}
	require.NoError(t, jm.RegisterInitializer("testapp", &noopInitializer{}))
	require.NoError(t, jm.RegisterProcessorBatch("testapp", "testop", &trackingBatchProcessor{}))

	queries := batchsqlc.New(pool)
	ss := batchsqlc.StatusEnumSuccess
	ip := batchsqlc.StatusEnumInprog

	// Batch A: stuck -- inprog, all rows success
	stuckID := seedBatch(t, ctx, queries, "testapp", "testop", ip, []batchsqlc.StatusEnum{ss, ss, ss})

	// Batch B: legitimate in-progress -- inprog, 1 row still inprog
	activeID := seedBatch(t, ctx, queries, "testapp", "testop", ip, []batchsqlc.StatusEnum{ss, ss, ip})

	// Batch C: already summarized -- success, doneat set
	doneID := seedBatch(t, ctx, queries, "testapp", "testop", ss, []batchsqlc.StatusEnum{ss, ss})
	err = queries.UpdateBatchSummary(ctx, batchsqlc.UpdateBatchSummaryParams{
		ID: doneID, Status: ss,
		Doneat:      pgtype.Timestamp{Time: time.Now(), Valid: true},
		Outputfiles: []byte("{}"),
		Nsuccess:    pgtype.Int4{Int32: 2, Valid: true},
		Nfailed:     pgtype.Int4{Int32: 0, Valid: true},
		Naborted:    pgtype.Int4{Int32: 0, Valid: true},
	})
	require.NoError(t, err)

	require.NoError(t, jm.sweepUnsummarizedBatches(ctx))

	batchA, err := queries.GetBatchByID(ctx, stuckID)
	require.NoError(t, err)
	assert.True(t, batchA.Doneat.Valid, "stuck batch: doneat should be set after sweep")
	assert.Equal(t, ss, batchA.Status)

	batchB, err := queries.GetBatchByID(ctx, activeID)
	require.NoError(t, err)
	assert.False(t, batchB.Doneat.Valid, "active batch: doneat should still be NULL")
	assert.Equal(t, ip, batchB.Status)

	batchC, err := queries.GetBatchByID(ctx, doneID)
	require.NoError(t, err)
	assert.True(t, batchC.Doneat.Valid, "done batch: doneat should still be set")
	assert.Equal(t, ss, batchC.Status)
}
