package jobs

import "fmt"

// BatchStatusKey returns the Redis key for batch status.
// Uses hash tag {batchID} for Redis Cluster slot co-location.
func BatchStatusKey(batchID string) string {
	return fmt.Sprintf("ALYA_{%s}_STATUS", batchID)
}

// BatchResultKey returns the Redis key for batch result.
// Uses hash tag {batchID} for Redis Cluster slot co-location.
func BatchResultKey(batchID string) string {
	return fmt.Sprintf("ALYA_{%s}_RESULT", batchID)
}

// BatchOutputFilesKey returns the Redis key for batch output files.
// Uses hash tag {batchID} for Redis Cluster slot co-location.
func BatchOutputFilesKey(batchID string) string {
	return fmt.Sprintf("ALYA_{%s}_OUTFILES", batchID)
}

// BatchSummaryKey returns the Redis key for batch summary (status + outputfiles + counters).
// Uses hash tag {batchID} for Redis Cluster slot co-location.
func BatchSummaryKey(batchID string) string {
	return fmt.Sprintf("ALYA_{%s}_SUMMARY", batchID)
}

// workerRegistryKey returns the Redis key for the global worker registry SET.
// All workers register their instance IDs in this SET so recovery can discover
// them without using SCAN (which doesn't work across Redis Cluster nodes).
func workerRegistryKey() string {
	return "ALYA_WORKER_REGISTRY"
}

// workerHeartbeatKey returns the Redis key for a worker's heartbeat.
// Uses hash tag {instanceID} for Redis Cluster slot co-location.
func workerHeartbeatKey(instanceID string) string {
	return fmt.Sprintf("ALYA_{%s}_HEARTBEAT", instanceID)
}

// workerRowsKey returns the Redis key for a worker's active rows SET.
// Uses hash tag {instanceID} for Redis Cluster slot co-location.
func workerRowsKey(instanceID string) string {
	return fmt.Sprintf("ALYA_{%s}_ROWS", instanceID)
}

