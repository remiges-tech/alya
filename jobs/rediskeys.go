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
