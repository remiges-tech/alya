# Crash Recovery in Alya Batch Processing

When a worker dies mid-processing, its rows get stuck in `inprog` forever.
No other worker knows about them. The batch never completes.

This document describes how Alya recovers from worker crashes. Three ideas
make it work: heartbeats prove liveness, a registry discovers workers without
SCAN, and row tracking identifies what a dead worker was processing.

## The Problem

A worker fetches a block of rows, marks them `inprog`, and processes them one
by one. If the process crashes (OOM kill, SIGKILL, node failure), those rows
remain `inprog` in PostgreSQL. The batch framework only fetches rows with
status `queued`. The stuck rows are invisible to all workers.

## How Recovery Works

Recovery has three components that run in every worker instance.

### Heartbeat

Each worker writes a Redis key with a 60-second TTL:

```
ALYA_{hostname-1234-1234567890}_HEARTBEAT = "alive"  (TTL: 60s)
```

A background goroutine refreshes this key every 30 seconds. If the worker
crashes, the key expires within 60 seconds. Expiry means death.

`runHeartbeat` does not accept a context parameter. It uses
`context.Background()` internally. During shutdown, the processing context
is cancelled but the worker must finish its current block of rows. If the
heartbeat used the caller's context, it would stop refreshing. After 60
seconds the key expires. Another worker's recovery resets those rows to
`queued` while this worker is still processing them -- double processing.

### Worker Registry

Workers register in a Redis SET:

```
ALYA_WORKER_REGISTRY = { "host-1234-111", "host-5678-222", ... }
```

Recovery iterates this SET to find all workers, then checks each one's
heartbeat key. This avoids `SCAN`, which doesn't work across Redis Cluster
nodes.

The registry is rebuilt on every heartbeat tick. If Redis restarts, SADD
re-adds the worker on the next 30-second tick.

### Row Tracking

Before processing a block, the worker adds each row ID to a Redis SET:

```
ALYA_{hostname-1234-1234567890}_ROWS = { 100, 101, 102, ... }  (TTL: 180s)
```

After each row finishes (success or failure), it's removed from the SET.
`untrackRowProcessing` does not accept a context parameter. It uses
`context.Background()` internally because this SREM must succeed regardless
of shutdown state. If a cancelled context caused the SREM to fail, the row
ID would stay in the SET. Recovery would then reset an already-completed row
back to `queued` -- double processing.

`trackRowProcessing` is different -- it accepts a caller context. If tracking
fails during shutdown, the row ID doesn't make it into the SET, so recovery
won't find it. The worker is still alive and will finish the row normally.

The SET has a 180-second TTL, refreshed by every heartbeat tick and every
SADD. If the worker crashes and no other worker runs recovery within 180
seconds, the SET expires on its own. This prevents Redis memory leaks from
permanently orphaned keys.

## Recovery Sequence

Worker B discovers Worker A is dead and resets its rows:

```
TIME    EVENT
----    -----
T+0s    Worker A crashes while processing rows 100-109
T+0s    Redis: heartbeat key still exists (up to 60s TTL remaining)
T+0s    Redis: rows SET contains {105, 106, 107, 108, 109}
T+0s    Postgres: rows 105-109 have status 'inprog'

T+60s   Heartbeat key expires

T+60s   Worker B's periodic recovery runs (every 60s)
        1. SMEMBERS ALYA_WORKER_REGISTRY -> finds Worker A's instance ID
        2. EXISTS ALYA_{workerA}_HEARTBEAT -> 0 (expired)
        3. SMEMBERS ALYA_{workerA}_ROWS -> {105, 106, 107, 108, 109}
        4. UPDATE batchrows SET status='queued' WHERE rowid IN (105..109) AND status='inprog'
        5. DEL ALYA_{workerA}_ROWS
        6. SREM ALYA_WORKER_REGISTRY workerA

T+60s   Rows 105-109 are now 'queued' again
        Worker B's next FetchBlockOfRows picks them up
```

The worst-case recovery time is 120 seconds: 60s for heartbeat expiry plus
60s for the recovery interval.

## Row Reset

The SQL for resetting rows:

```sql
UPDATE batchrows
SET status = 'queued'
WHERE rowid = ANY($1::bigint[]) AND status = 'inprog'
```

The `AND status = 'inprog'` guard prevents resetting rows that finished
between the crash and recovery (shouldn't happen with a dead worker, but
costs nothing to include).

The parent batch status is not changed. The batch stays `inprog`.
`FetchBlockOfRows` selects rows where `batchrows.status = 'queued' AND
batches.status != 'wait'`, so recovered rows are picked up from batches in
`inprog` state without any batch-level reset.

## Graceful Shutdown

On SIGTERM (normal Kubernetes shutdown):

1. The processing context is cancelled
2. The worker finishes its current block of rows (no mid-block cancellation)
3. `Shutdown()` deletes the heartbeat key and deregisters from the registry
4. The rows SET is left in place -- if any rows remain (shouldn't after step
   2), another worker can still recover them

Step 2 is the key design choice. Once rows are committed as `inprog`, the
worker finishes all of them. If the Kubernetes grace period expires before
completion, SIGKILL terminates the process and crash recovery handles the
rest.

## Timing Constants

| Constant          | Value | Rationale                                      |
|-------------------|-------|-------------------------------------------------|
| heartbeatTTL      | 60s   | Detection window -- how long before a dead worker is noticed |
| heartbeatInterval | 30s   | Refresh rate -- must be less than TTL           |
| recoveryInterval  | 60s   | How often workers scan for dead peers           |
| workerRowsTTL     | 180s  | 3x heartbeat TTL -- must exceed detection + recovery window (120s) |

## Redis Keys

All keys use `{...}` hash tags for Redis Cluster slot co-location.

| Key pattern                       | Type   | TTL  | Purpose                     |
|-----------------------------------|--------|------|-----------------------------|
| `ALYA_{instanceID}_HEARTBEAT`     | String | 60s  | Liveness indicator          |
| `ALYA_{instanceID}_ROWS`          | SET    | 180s | Row IDs being processed     |
| `ALYA_WORKER_REGISTRY`            | SET    | none | All known worker instances  |
