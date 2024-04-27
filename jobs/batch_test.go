package jobs_test

import (
	"context"
	"errors"
	"fmt"
	"log"
	"testing"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/remiges-tech/alya/jobs"
	"github.com/remiges-tech/alya/jobs/pg/batchsqlc"
	"github.com/remiges-tech/alya/wscutils"
	"github.com/stretchr/testify/assert"
)

type mockBatchProcessor struct{}

func (m *mockBatchProcessor) DoBatchJob(initBlock jobs.InitBlock, context jobs.JSONstr, line int, input jobs.JSONstr) (batchsqlc.StatusEnum, jobs.JSONstr, []wscutils.ErrorMessage, map[string]string, error) {
	result, _ := jobs.NewJSONstr("")
	return batchsqlc.StatusEnumSuccess, result, nil, nil, nil
}
func TestRegisterBatchProcessor(t *testing.T) {
	jm := jobs.NewJobManager(nil, nil, nil, nil)

	// Test registering a new processor
	err := jm.RegisterProcessorBatch("app1", "op1", &mockBatchProcessor{})
	assert.NoError(t, err)

	// Test registering a duplicate processor
	err = jm.RegisterProcessorBatch("app1", "op1", &mockBatchProcessor{})
	assert.Error(t, err)
	assert.True(t, errors.Is(err, jobs.ErrProcessorAlreadyRegistered))

	// Test registering a different processor for the same app but different op
	err = jm.RegisterProcessorBatch("app1", "op2", &mockBatchProcessor{})
	assert.NoError(t, err)

	// Test registering a different processor for a different app
	err = jm.RegisterProcessorBatch("app2", "op1", &mockBatchProcessor{})
	assert.NoError(t, err)
}

func TestBatchDone(t *testing.T) {
	// Create a PostgreSQL connection pool
	db := getDb()
	defer db.Close()

	// Acquire a connection from the pool
	conn, err := db.Acquire(context.Background())
	assert.NoError(t, err)
	defer conn.Release()
	// Run database migrations
	err = jobs.MigrateDatabase(conn.Conn())
	assert.NoError(t, err)

	// Create a Redis client
	redisClient := getRedisClient()
	defer redisClient.Close()

	// Create a JobManager instance with the database and Redis dependencies
	jm := &jobs.JobManager{
		Queries:     batchsqlc.New(db),
		RedisClient: redisClient,
	}

	// Generate a random batch ID
	batchID := uuid.New().String()

	// Insert test data into the database
	err = insertTestData(db, batchID)
	assert.NoError(t, err)

	// Case 1: Status not in Redis
	status, batchOutput, outputFiles, nsuccess, nfailed, naborted, err := jm.BatchDone(batchID)
	assert.NoError(t, err)
	assert.Equal(t, batchsqlc.StatusEnumSuccess, status)
	assert.Len(t, batchOutput, 2)
	assert.Equal(t, map[string]string{"file1": "object1", "file2": "object2"}, outputFiles)
	assert.Equal(t, 2, nsuccess)
	assert.Equal(t, 0, nfailed)
	assert.Equal(t, 0, naborted)

	// Case 2: Status present in Redis
	err = redisClient.Set(context.Background(), jobs.GetBatchStatusRedisKey(batchID), string(batchsqlc.StatusEnumSuccess), time.Hour).Err()
	assert.NoError(t, err)

	status, batchOutput, outputFiles, nsuccess, nfailed, naborted, err = jm.BatchDone(batchID)
	assert.NoError(t, err)
	assert.Equal(t, batchsqlc.StatusEnumSuccess, status)
	assert.Len(t, batchOutput, 2)
	assert.Equal(t, map[string]string{"file1": "object1", "file2": "object2"}, outputFiles)
	assert.Equal(t, 2, nsuccess)
	assert.Equal(t, 0, nfailed)
	assert.Equal(t, 0, naborted)
}

func insertTestData(db *pgxpool.Pool, batchID string) error {
	// Insert test data into the batches table
	_, err := db.Exec(context.Background(), `
		INSERT INTO batches (id, app, op, context, status, reqat, outputfiles, nsuccess, nfailed, naborted)
		VALUES ($1, 'testapp', 'testop', '{}', 'success', NOW(), '{"file1": "object1", "file2": "object2"}'::jsonb, 2, 0, 0)
	`, batchID)
	if err != nil {
		return err
	}

	// Insert test data into the batchrows table
	_, err = db.Exec(context.Background(), `
		INSERT INTO batchrows (batch, line, input, status, reqat)
		VALUES
			($1, 1, '{"input": "input1"}'::jsonb, 'success', NOW()),
			($1, 2, '{"input": "input2"}'::jsonb, 'success', NOW())
	`, batchID)
	if err != nil {
		return err
	}

	return nil
}

func getDb() *pgxpool.Pool {
	dbHost := "localhost"
	dbPort := 5432
	dbUser := "alyatest"
	dbPassword := "alyatest"
	dbName := "alyatest"

	connStr := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
		dbHost, dbPort, dbUser, dbPassword, dbName)

	pool, err := pgxpool.New(context.Background(), connStr)
	if err != nil {
		log.Fatal("Error connecting to the database:", err)
	}
	return pool
}

func getRedisClient() *redis.Client {
	redisHost := "localhost"
	redisPort := 6379

	redisClient := redis.NewClient(&redis.Options{
		Addr: fmt.Sprintf("%s:%d", redisHost, redisPort),
	})
	return redisClient
}
