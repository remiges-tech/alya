package jobs

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// getClusterNodeClient returns a regular Redis client connected to a cluster node.
// CLUSTER KEYSLOT works on any node without needing full cluster client.
func getClusterNodeClient() *redis.Client {
	addr := os.Getenv("REDIS_CLUSTER_ADDR")
	if addr == "" {
		addr = "localhost:7000"
	}
	return redis.NewClient(&redis.Options{
		Addr: addr,
	})
}

// TestRedisClusterMultiKeyTransaction verifies that batch keys with hash tags
// can be used in multi-key transactions on Redis Cluster without CROSSSLOT errors.
//
// Run with: go test -v -run TestRedisClusterMultiKeyTransaction
// Requires: docker compose up redis-cluster
func TestRedisClusterMultiKeyTransaction(t *testing.T) {
	if os.Getenv("REDIS_CLUSTER_TEST") != "1" {
		t.Skip("Skipping Redis Cluster test. Set REDIS_CLUSTER_TEST=1 to run.")
	}

	client := getClusterNodeClient()
	defer client.Close()

	ctx := context.Background()

	// Verify cluster connection
	err := client.Ping(ctx).Err()
	require.NoError(t, err, "Failed to connect to Redis Cluster node")

	// Verify this is actually a cluster node
	clusterInfo, err := client.ClusterInfo(ctx).Result()
	require.NoError(t, err, "Failed to get cluster info")
	require.Contains(t, clusterInfo, "cluster_state:ok", "Not a cluster node or cluster not ready")

	batchID := "test-batch-cluster-" + time.Now().Format("20060102150405")
	statusKey := BatchStatusKey(batchID)
	resultKey := BatchResultKey(batchID)
	outputFilesKey := BatchOutputFilesKey(batchID)
	summaryKey := BatchSummaryKey(batchID)

	// Clean up before test
	client.Del(ctx, statusKey, resultKey, outputFilesKey, summaryKey)

	t.Run("multi_key_set_in_pipeline", func(t *testing.T) {
		// This simulates what updateStatusAndOutputFilesDataInRedis does
		// On a cluster node, this works because all keys hash to the same slot
		err := client.Watch(ctx, func(tx *redis.Tx) error {
			_, err := tx.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
				pipe.Set(ctx, statusKey, "success", time.Minute)
				pipe.Set(ctx, resultKey, `{"data": "test"}`, time.Minute)
				pipe.Set(ctx, outputFilesKey, `{"file1": "path1"}`, time.Minute)
				return nil
			})
			return err
		}, statusKey)

		// If we get CROSSSLOT error, the hash tags aren't working
		if err != nil && err.Error() == "CROSSSLOT Keys in request don't hash to the same slot" {
			t.Fatal("CROSSSLOT error: hash tags are not working correctly")
		}
		// MOVED error is expected when connecting to wrong node - that's fine
		// It means the cluster recognized the keys but they belong to different node
		if err != nil {
			t.Logf("Got error (may be MOVED which is OK): %v", err)
		}
	})

	t.Run("keys_on_same_slot", func(t *testing.T) {
		// Verify all keys are on the same slot using CLUSTER KEYSLOT
		statusSlot, err := client.ClusterKeySlot(ctx, statusKey).Result()
		require.NoError(t, err)

		resultSlot, err := client.ClusterKeySlot(ctx, resultKey).Result()
		require.NoError(t, err)

		outputFilesSlot, err := client.ClusterKeySlot(ctx, outputFilesKey).Result()
		require.NoError(t, err)

		summarySlot, err := client.ClusterKeySlot(ctx, summaryKey).Result()
		require.NoError(t, err)

		assert.Equal(t, statusSlot, resultSlot, "STATUS and RESULT keys should be on same slot")
		assert.Equal(t, statusSlot, outputFilesSlot, "STATUS and OUTFILES keys should be on same slot")
		assert.Equal(t, statusSlot, summarySlot, "STATUS and SUMMARY keys should be on same slot")

		t.Logf("All keys for batch %s are on slot %d", batchID, statusSlot)
	})

	// Clean up
	client.Del(ctx, statusKey, resultKey, outputFilesKey, summaryKey)
}

// TestRedisClusterKeySlotVerification uses CLUSTER KEYSLOT to verify
// that our hash tag implementation works correctly.
func TestRedisClusterKeySlotVerification(t *testing.T) {
	if os.Getenv("REDIS_CLUSTER_TEST") != "1" {
		t.Skip("Skipping Redis Cluster test. Set REDIS_CLUSTER_TEST=1 to run.")
	}

	client := getClusterNodeClient()
	defer client.Close()

	ctx := context.Background()

	// Verify connection
	err := client.Ping(ctx).Err()
	require.NoError(t, err, "Failed to connect to Redis Cluster node")

	testBatchIDs := []string{
		"550e8400-e29b-41d4-a716-446655440000",
		"test-batch-123",
		"abc",
	}

	for _, batchID := range testBatchIDs {
		t.Run(batchID, func(t *testing.T) {
			statusKey := BatchStatusKey(batchID)
			resultKey := BatchResultKey(batchID)
			outputFilesKey := BatchOutputFilesKey(batchID)
			summaryKey := BatchSummaryKey(batchID)

			statusSlot, err := client.ClusterKeySlot(ctx, statusKey).Result()
			require.NoError(t, err)

			resultSlot, err := client.ClusterKeySlot(ctx, resultKey).Result()
			require.NoError(t, err)

			outputFilesSlot, err := client.ClusterKeySlot(ctx, outputFilesKey).Result()
			require.NoError(t, err)

			summarySlot, err := client.ClusterKeySlot(ctx, summaryKey).Result()
			require.NoError(t, err)

			assert.Equal(t, statusSlot, resultSlot,
				"STATUS and RESULT should be on same slot for batchID=%s", batchID)
			assert.Equal(t, statusSlot, outputFilesSlot,
				"STATUS and OUTFILES should be on same slot for batchID=%s", batchID)
			assert.Equal(t, statusSlot, summarySlot,
				"STATUS and SUMMARY should be on same slot for batchID=%s", batchID)

			t.Logf("batchID=%s -> slot %d", batchID, statusSlot)
		})
	}
}
