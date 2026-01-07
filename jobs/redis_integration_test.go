package jobs

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
	"github.com/remiges-tech/alya/jobs/pg/batchsqlc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Prerequisites:
//   - Redis must be running: docker compose up redis
//   - Set environment variable: REDIS_TEST=1
//
// Run with: go test -v -run TestRedis.*_Integration ./jobs/

func getRedisTestClient() *redis.Client {
	addr := os.Getenv("REDIS_ADDR")
	if addr == "" {
		addr = "localhost:6379"
	}
	return redis.NewClient(&redis.Options{
		Addr: addr,
	})
}

func TestUpdateStatusInRedis_Integration(t *testing.T) {
	if os.Getenv("REDIS_TEST") != "1" {
		t.Skip("Skipping Redis integration test. Set REDIS_TEST=1 to run.")
	}

	client := getRedisTestClient()
	defer client.Close()

	ctx := context.Background()

	err := client.Ping(ctx).Err()
	require.NoError(t, err, "Failed to connect to Redis")

	t.Run("set_status_with_ttl", func(t *testing.T) {
		batchID := uuid.New()
		status := batchsqlc.StatusEnumSuccess
		expirySec := 60

		err := updateStatusInRedis(client, batchID, status, expirySec)
		require.NoError(t, err)

		key := BatchStatusKey(batchID.String())

		val, err := client.Get(ctx, key).Result()
		require.NoError(t, err)
		assert.Equal(t, string(status), val)

		ttl, err := client.TTL(ctx, key).Result()
		require.NoError(t, err)
		assert.True(t, ttl > 0 && ttl <= time.Duration(expirySec)*time.Second,
			"TTL should be between 0 and %d seconds, got %v", expirySec, ttl)

		client.Del(ctx, key)
	})

	t.Run("overwrite_existing_status", func(t *testing.T) {
		batchID := uuid.New()
		expirySec := 60

		err := updateStatusInRedis(client, batchID, batchsqlc.StatusEnumQueued, expirySec)
		require.NoError(t, err)

		err = updateStatusInRedis(client, batchID, batchsqlc.StatusEnumSuccess, expirySec)
		require.NoError(t, err)

		key := BatchStatusKey(batchID.String())
		val, err := client.Get(ctx, key).Result()
		require.NoError(t, err)
		assert.Equal(t, string(batchsqlc.StatusEnumSuccess), val)

		client.Del(ctx, key)
	})

	t.Run("different_status_values", func(t *testing.T) {
		statuses := []batchsqlc.StatusEnum{
			batchsqlc.StatusEnumQueued,
			batchsqlc.StatusEnumInprog,
			batchsqlc.StatusEnumSuccess,
			batchsqlc.StatusEnumFailed,
			batchsqlc.StatusEnumAborted,
		}

		for _, status := range statuses {
			batchID := uuid.New()
			err := updateStatusInRedis(client, batchID, status, 60)
			require.NoError(t, err, "Failed for status %s", status)

			key := BatchStatusKey(batchID.String())
			val, err := client.Get(ctx, key).Result()
			require.NoError(t, err)
			assert.Equal(t, string(status), val)

			client.Del(ctx, key)
		}
	})
}

func TestUpdateStatusAndOutputFilesDataInRedis_Integration(t *testing.T) {
	if os.Getenv("REDIS_TEST") != "1" {
		t.Skip("Skipping Redis integration test. Set REDIS_TEST=1 to run.")
	}

	client := getRedisTestClient()
	defer client.Close()

	ctx := context.Background()

	err := client.Ping(ctx).Err()
	require.NoError(t, err, "Failed to connect to Redis")

	t.Run("set_all_three_keys_with_ttl", func(t *testing.T) {
		batchID := uuid.New()
		status := batchsqlc.StatusEnumSuccess
		outputFiles := map[string]string{
			"output.txt": "file-id-123",
			"error.log":  "file-id-456",
		}
		result := `{"data": "test result"}`
		expirySec := 60

		err := updateStatusAndOutputFilesDataInRedis(client, batchID, status, outputFiles, result, expirySec)
		require.NoError(t, err)

		statusKey := BatchStatusKey(batchID.String())
		resultKey := BatchResultKey(batchID.String())
		outputFilesKey := BatchOutputFilesKey(batchID.String())

		statusVal, err := client.Get(ctx, statusKey).Result()
		require.NoError(t, err)
		assert.Equal(t, string(status), statusVal)

		resultVal, err := client.Get(ctx, resultKey).Result()
		require.NoError(t, err)
		assert.Equal(t, result, resultVal)

		outputFilesVal, err := client.Get(ctx, outputFilesKey).Result()
		require.NoError(t, err)
		assert.Contains(t, outputFilesVal, "output.txt")
		assert.Contains(t, outputFilesVal, "file-id-123")

		for _, key := range []string{statusKey, resultKey, outputFilesKey} {
			ttl, err := client.TTL(ctx, key).Result()
			require.NoError(t, err)
			assert.True(t, ttl > 0 && ttl <= time.Duration(expirySec)*time.Second,
				"TTL for %s should be between 0 and %d seconds, got %v", key, expirySec, ttl)
		}

		client.Del(ctx, statusKey, resultKey, outputFilesKey)
	})

	t.Run("overwrite_existing_values", func(t *testing.T) {
		batchID := uuid.New()
		expirySec := 60

		err := updateStatusAndOutputFilesDataInRedis(client, batchID,
			batchsqlc.StatusEnumInprog,
			map[string]string{"old.txt": "old-id"},
			`{"old": "data"}`,
			expirySec)
		require.NoError(t, err)

		err = updateStatusAndOutputFilesDataInRedis(client, batchID,
			batchsqlc.StatusEnumSuccess,
			map[string]string{"new.txt": "new-id"},
			`{"new": "data"}`,
			expirySec)
		require.NoError(t, err)

		statusKey := BatchStatusKey(batchID.String())
		resultKey := BatchResultKey(batchID.String())
		outputFilesKey := BatchOutputFilesKey(batchID.String())

		statusVal, err := client.Get(ctx, statusKey).Result()
		require.NoError(t, err)
		assert.Equal(t, string(batchsqlc.StatusEnumSuccess), statusVal)

		resultVal, err := client.Get(ctx, resultKey).Result()
		require.NoError(t, err)
		assert.Equal(t, `{"new": "data"}`, resultVal)

		outputFilesVal, err := client.Get(ctx, outputFilesKey).Result()
		require.NoError(t, err)
		assert.Contains(t, outputFilesVal, "new.txt")
		assert.NotContains(t, outputFilesVal, "old.txt")

		client.Del(ctx, statusKey, resultKey, outputFilesKey)
	})

	t.Run("empty_output_files", func(t *testing.T) {
		batchID := uuid.New()
		expirySec := 60

		err := updateStatusAndOutputFilesDataInRedis(client, batchID,
			batchsqlc.StatusEnumSuccess,
			map[string]string{},
			`{"result": "no files"}`,
			expirySec)
		require.NoError(t, err)

		statusKey := BatchStatusKey(batchID.String())
		resultKey := BatchResultKey(batchID.String())
		outputFilesKey := BatchOutputFilesKey(batchID.String())

		outputFilesVal, err := client.Get(ctx, outputFilesKey).Result()
		require.NoError(t, err)
		assert.Equal(t, "{}", outputFilesVal)

		client.Del(ctx, statusKey, resultKey, outputFilesKey)
	})
}
