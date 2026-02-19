package jobs

import (
	"context"
	"fmt"
	"math/rand"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/remiges-tech/alya/jobs/pg/batchsqlc"
)

const (
	heartbeatTTL      = 60 * time.Second
	heartbeatInterval = 30 * time.Second
	recoveryInterval  = 60 * time.Second
	workerRowsTTL     = 3 * heartbeatTTL // 180s, must exceed recovery detection window
)

// TrackRowProcessing adds a row to this instance's active rows SET in Redis.
// Sets a TTL on the SET so it expires if the worker crashes and recovery
// doesn't run within the TTL window. The TTL is also refreshed by the
// heartbeat loop.
func (jm *JobManager) TrackRowProcessing(ctx context.Context, rowID int64) error {
	if jm.redisClient == nil {
		return nil
	}
	key := WorkerRowsKey(jm.instanceID)
	if err := jm.redisClient.SAdd(ctx, key, rowID).Err(); err != nil {
		return err
	}
	return jm.redisClient.Expire(ctx, key, workerRowsTTL).Err()
}

// UntrackRowProcessing removes a row from this instance's active rows SET in Redis.
// Uses context.Background() internally instead of accepting a caller context.
// During shutdown the processing context is cancelled, but this SREM must still
// succeed -- otherwise the row ID stays in the SET and recovery resets an
// already-completed row back to 'queued', causing double processing.
func (jm *JobManager) UntrackRowProcessing(rowID int64) error {
	if jm.redisClient == nil {
		return nil
	}
	return jm.redisClient.SRem(context.Background(), WorkerRowsKey(jm.instanceID), rowID).Err()
}

// RefreshHeartbeat updates this instance's heartbeat TTL in Redis.
// The heartbeat indicates that this instance is alive and processing.
func (jm *JobManager) RefreshHeartbeat(ctx context.Context) error {
	if jm.redisClient == nil {
		return nil
	}
	return jm.redisClient.Set(ctx, WorkerHeartbeatKey(jm.instanceID), "alive", heartbeatTTL).Err()
}

// RegisterWorker adds this instance's ID to the global worker registry SET.
// This allows recovery to discover all workers without using SCAN
// (which doesn't work across Redis Cluster nodes).
func (jm *JobManager) RegisterWorker(ctx context.Context) error {
	if jm.redisClient == nil {
		return nil
	}
	return jm.redisClient.SAdd(ctx, WorkerRegistryKey(), jm.instanceID).Err()
}

// DeregisterWorker removes this instance's ID from the global worker registry SET.
// Called during graceful shutdown after heartbeat removal.
func (jm *JobManager) DeregisterWorker(ctx context.Context) error {
	if jm.redisClient == nil {
		return nil
	}
	return jm.redisClient.SRem(ctx, WorkerRegistryKey(), jm.instanceID).Err()
}

// RecoverAbandonedRows finds rows from dead worker instances and resets them to queued.
// It reads the worker registry SET to discover all workers, checks if each worker's
// heartbeat has expired, and if so, resets those rows in the database.
// Uses the registry instead of SCAN for Redis Cluster compatibility.
func (jm *JobManager) RecoverAbandonedRows(ctx context.Context) (int, error) {
	if jm.redisClient == nil || jm.db == nil {
		return 0, nil
	}

	// Get all registered worker instance IDs from the registry
	instanceIDs, err := jm.redisClient.SMembers(ctx, WorkerRegistryKey()).Result()
	if err != nil {
		return 0, fmt.Errorf("failed to get worker registry: %w", err)
	}

	var totalRecovered int

	for _, instanceID := range instanceIDs {
		// Skip our own instance
		if instanceID == jm.instanceID {
			continue
		}

		// Check if this instance is alive
		heartbeatKey := WorkerHeartbeatKey(instanceID)
		exists, err := jm.redisClient.Exists(ctx, heartbeatKey).Result()
		if err != nil {
			jm.logger.Error(err).LogActivity("Failed to check heartbeat", map[string]any{
				"instanceID": instanceID,
			})
			continue
		}

		if exists == 1 {
			// Instance is alive, skip
			continue
		}

		// Instance is dead, recover its rows
		recovered, err := jm.recoverRowsFromDeadInstance(ctx, instanceID)
		if err != nil {
			jm.logger.Error(err).LogActivity("Failed to recover rows from dead instance", map[string]any{
				"instanceID": instanceID,
			})
			continue
		}

		totalRecovered += recovered

		// Remove dead worker from registry after recovering its rows, not before.
		// If we crash between recovery and this SREM, the next cycle finds the
		// same dead worker. The rows SET is already deleted above, so
		// recoverRowsFromDeadInstance returns 0 and we reach the SREM again.
		if err := jm.redisClient.SRem(ctx, WorkerRegistryKey(), instanceID).Err(); err != nil {
			jm.logger.Warn().LogActivity("Failed to remove dead worker from registry", map[string]any{
				"instanceID": instanceID,
				"error":      err.Error(),
			})
		}
	}

	return totalRecovered, nil
}

// recoverRowsFromDeadInstance recovers rows from a dead worker instance.
// It gets the row IDs from Redis, resets them in the database, and cleans up Redis.
func (jm *JobManager) recoverRowsFromDeadInstance(ctx context.Context, instanceID string) (int, error) {
	rowsKey := WorkerRowsKey(instanceID)

	// Get all row IDs from the dead instance
	rowIDStrs, err := jm.redisClient.SMembers(ctx, rowsKey).Result()
	if err != nil {
		return 0, fmt.Errorf("failed to get rows for instance %s: %w", instanceID, err)
	}

	if len(rowIDStrs) == 0 {
		// Clean up empty set
		jm.redisClient.Del(ctx, rowsKey)
		return 0, nil
	}

	// Convert to int64
	rowIDs := make([]int64, 0, len(rowIDStrs))
	for _, s := range rowIDStrs {
		id, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			jm.logger.Warn().LogActivity("Invalid row ID in recovery set", map[string]any{
				"instanceID": instanceID,
				"rowIDStr":   s,
			})
			continue
		}
		rowIDs = append(rowIDs, id)
	}

	if len(rowIDs) == 0 {
		jm.redisClient.Del(ctx, rowsKey)
		return 0, nil
	}

	// Reset rows in database, then delete the Redis SET. These two steps are
	// not atomic. If the process crashes between them, the next recovery cycle
	// reads the same row IDs from Redis and calls ResetRowsToQueued again. The
	// SQL guard (AND status = 'inprog') makes this safe -- rows already reset
	// to 'queued' or reprocessed to 'success' are not affected.
	err = jm.resetRowsToQueued(ctx, rowIDs)
	if err != nil {
		return 0, fmt.Errorf("failed to reset rows: %w", err)
	}

	jm.redisClient.Del(ctx, rowsKey)

	// rowCount and the returned count may overcount when multiple workers
	// recover the same dead worker concurrently. The SQL guard
	// (AND status='inprog') makes the DB reset idempotent -- the second
	// worker's UPDATE matches zero rows -- but both report len(rowIDs).
	// We accept this tradeoff to keep recovery lock-free and crash-safe.
	jm.logger.Info().LogActivity("Recovered rows from dead instance", map[string]any{
		"instanceID": instanceID,
		"rowCount":   len(rowIDs),
	})

	return len(rowIDs), nil
}

// resetRowsToQueued resets the given rows to 'queued' status.
// The parent batch status is left as 'inprog'. FetchBlockOfRows selects rows
// where batchrows.status = 'queued' AND batches.status != 'wait', so the
// recovered rows will be picked up without resetting the batch.
func (jm *JobManager) resetRowsToQueued(ctx context.Context, rowIDs []int64) error {
	tx, err := jm.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	txQueries := batchsqlc.New(tx)

	err = txQueries.ResetRowsToQueued(ctx, rowIDs)
	if err != nil {
		return fmt.Errorf("failed to reset rows to queued: %w", err)
	}

	return tx.Commit(ctx)
}

// runHeartbeat runs the heartbeat loop in a background goroutine.
// It runs until the process exits. Does not accept a context parameter
// because the heartbeat must stay alive while the processing loop finishes
// its current block of rows after shutdown is initiated. Uses
// context.Background() for Redis operations so they continue working
// after the caller's context is cancelled.
func (jm *JobManager) runHeartbeat() {
	ctx := context.Background()

	if err := jm.RegisterWorker(ctx); err != nil {
		jm.logger.Error(err).LogActivity("Failed to register worker", nil)
	}

	if err := jm.RefreshHeartbeat(ctx); err != nil {
		jm.logger.Error(err).LogActivity("Failed to send initial heartbeat", nil)
	}

	ticker := time.NewTicker(heartbeatInterval)
	defer ticker.Stop()

	// This loop never exits -- no select on a context or done channel. The
	// goroutine runs until the process dies. This is intentional: the heartbeat
	// must outlive the caller's context to prevent other workers from recovering
	// rows that this worker is still processing after shutdown is initiated.
	for {
		<-ticker.C
		// Re-register on every tick so the registry is rebuilt if Redis restarts.
		// SADD is idempotent -- returns 0 if already a member, no data change.
		if err := jm.RegisterWorker(ctx); err != nil {
			jm.logger.Error(err).LogActivity("Failed to re-register worker", nil)
		}
		if err := jm.RefreshHeartbeat(ctx); err != nil {
			jm.logger.Error(err).LogActivity("Failed to refresh heartbeat", nil)
		}
		// Refresh TTL on the rows SET. EXPIRE on a non-existent key (no rows
		// tracked or all rows untracked) is a no-op.
		jm.redisClient.Expire(ctx, WorkerRowsKey(jm.instanceID), workerRowsTTL)
	}
}

// runPeriodicRecovery runs the recovery loop in a background goroutine.
// It checks for abandoned rows every recoveryInterval.
func (jm *JobManager) runPeriodicRecovery(ctx context.Context) {
	// Immediate first recovery check
	if recovered, err := jm.RecoverAbandonedRows(ctx); err != nil {
		jm.logger.Error(err).LogActivity("Initial recovery failed", nil)
	} else if recovered > 0 {
		jm.logger.Info().LogActivity("Initial recovery completed", map[string]any{
			"count": recovered,
		})
	}

	ticker := time.NewTicker(recoveryInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if recovered, err := jm.RecoverAbandonedRows(ctx); err != nil {
				jm.logger.Error(err).LogActivity("Periodic recovery failed", nil)
			} else if recovered > 0 {
				jm.logger.Info().LogActivity("Periodic recovery completed", map[string]any{
					"count": recovered,
				})
			}
		}
	}
}

const (
	sweepMinInterval = 5 * time.Minute
	sweepMaxInterval = 10 * time.Minute
)

// sweepUnsummarizedBatches finds batches stuck in 'inprog' with all rows in
// terminal status and feeds them into summarizeCompletedBatches. This catches
// batches that were missed due to race conditions or exhausted retries during
// normal summarization.
func (jm *JobManager) sweepUnsummarizedBatches(ctx context.Context) error {
	batchIDs, err := jm.queries.GetUnsummarizedBatches(ctx)
	if err != nil {
		return fmt.Errorf("failed to query unsummarized batches: %w", err)
	}

	if len(batchIDs) == 0 {
		return nil
	}

	jm.logger.Info().LogActivity("Sweep found unsummarized batches", map[string]any{
		"count": len(batchIDs),
	})

	batchSet := make(map[uuid.UUID]bool, len(batchIDs))
	for _, id := range batchIDs {
		batchSet[id] = true
	}

	return jm.summarizeCompletedBatches(ctx, batchSet)
}

// runPeriodicSweep runs the sweep loop in a background goroutine.
// It queries for stuck batches at random intervals between sweepMinInterval
// and sweepMaxInterval.
func (jm *JobManager) runPeriodicSweep(ctx context.Context) {
	for {
		interval := sweepMinInterval + time.Duration(rand.Int63n(int64(sweepMaxInterval-sweepMinInterval)))
		select {
		case <-ctx.Done():
			return
		case <-time.After(interval):
			if err := jm.sweepUnsummarizedBatches(ctx); err != nil {
				jm.logger.Error(err).LogActivity("Periodic sweep failed", nil)
			}
		}
	}
}

// Shutdown cleans up this instance's Redis keys on graceful shutdown.
// It removes the heartbeat key. The rows key is intentionally left in place
// so that if this instance has active rows, they can be recovered by other instances.
func (jm *JobManager) Shutdown(ctx context.Context) error {
	if jm.redisClient == nil {
		return nil
	}

	// Remove heartbeat - this signals to other instances that we're shutting down
	if err := jm.redisClient.Del(ctx, WorkerHeartbeatKey(jm.instanceID)).Err(); err != nil {
		return fmt.Errorf("failed to remove heartbeat: %w", err)
	}

	// Deregister from the worker registry
	if err := jm.DeregisterWorker(ctx); err != nil {
		jm.logger.Warn().LogActivity("Failed to deregister worker from registry", map[string]any{
			"instanceID": jm.instanceID,
			"error":      err.Error(),
		})
	}

	jm.logger.Info().LogActivity("JobManager shutdown complete", map[string]any{
		"instanceID": jm.instanceID,
	})

	return nil
}

