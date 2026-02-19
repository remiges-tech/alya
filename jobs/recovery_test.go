package jobs_test

import (
	"context"
	"log"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/go-redis/redis/v8"
	"github.com/remiges-tech/alya/jobs"
	"github.com/remiges-tech/logharbour/logharbour"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRecoverAbandonedRows(t *testing.T) {
	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()

	redisClient := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})
	defer redisClient.Close()

	loggerCtx := &logharbour.LoggerContext{}
	logger := logharbour.NewLogger(loggerCtx, "test", log.Writer())
	ctx := context.Background()

	t.Run("skips alive workers in registry", func(t *testing.T) {
		// Clean up
		redisClient.FlushAll(ctx)

		// Register two workers, both alive
		jm1 := jobs.NewJobManager(nil, redisClient, nil, logger, nil)
		jm1.RegisterWorker(ctx)
		jm1.RefreshHeartbeat(ctx)

		jm2 := jobs.NewJobManager(nil, redisClient, nil, logger, nil)
		jm2.RegisterWorker(ctx)
		jm2.RefreshHeartbeat(ctx)

		// Both are alive, recovery should find nothing
		recovered, err := jm1.RecoverAbandonedRows(ctx)
		require.NoError(t, err)
		assert.Equal(t, 0, recovered)

		// Both should still be in registry
		members, err := redisClient.SMembers(ctx, jobs.WorkerRegistryKey()).Result()
		require.NoError(t, err)
		assert.Len(t, members, 2)
	})
}

func TestShutdown(t *testing.T) {
	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()

	redisClient := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})
	defer redisClient.Close()

	loggerCtx := &logharbour.LoggerContext{}
	logger := logharbour.NewLogger(loggerCtx, "test", log.Writer())
	jm := jobs.NewJobManager(nil, redisClient, nil, logger, nil)
	ctx := context.Background()

	t.Run("shutdown removes heartbeat key and deregisters from registry", func(t *testing.T) {
		// Register and set heartbeat
		err := jm.RegisterWorker(ctx)
		require.NoError(t, err)
		err = jm.RefreshHeartbeat(ctx)
		require.NoError(t, err)

		heartbeatKey := jobs.WorkerHeartbeatKey(jm.InstanceID())
		exists, err := redisClient.Exists(ctx, heartbeatKey).Result()
		require.NoError(t, err)
		assert.Equal(t, int64(1), exists, "heartbeat key should exist before shutdown")

		isMember, err := redisClient.SIsMember(ctx, jobs.WorkerRegistryKey(), jm.InstanceID()).Result()
		require.NoError(t, err)
		assert.True(t, isMember, "worker should be in registry before shutdown")

		// Shutdown
		err = jm.Shutdown(ctx)
		require.NoError(t, err)

		// Verify heartbeat is removed
		exists, err = redisClient.Exists(ctx, heartbeatKey).Result()
		require.NoError(t, err)
		assert.Equal(t, int64(0), exists, "heartbeat key should be removed after shutdown")

		// Verify deregistered from registry
		isMember, err = redisClient.SIsMember(ctx, jobs.WorkerRegistryKey(), jm.InstanceID()).Result()
		require.NoError(t, err)
		assert.False(t, isMember, "worker should be removed from registry after shutdown")
	})

	t.Run("shutdown leaves rows key for recovery", func(t *testing.T) {
		// Track some rows
		err := jm.TrackRowProcessing(ctx, 1)
		require.NoError(t, err)
		err = jm.TrackRowProcessing(ctx, 2)
		require.NoError(t, err)

		rowsKey := jobs.WorkerRowsKey(jm.InstanceID())
		exists, err := redisClient.Exists(ctx, rowsKey).Result()
		require.NoError(t, err)
		assert.Equal(t, int64(1), exists, "rows key should exist before shutdown")

		// Shutdown
		err = jm.Shutdown(ctx)
		require.NoError(t, err)

		// Verify rows key is still there (for recovery by other instances)
		exists, err = redisClient.Exists(ctx, rowsKey).Result()
		require.NoError(t, err)
		assert.Equal(t, int64(1), exists, "rows key should remain after shutdown for recovery")
	})
}

