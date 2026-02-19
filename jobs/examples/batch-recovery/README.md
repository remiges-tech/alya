# Batch Recovery Example

This example demonstrates the crash recovery mechanism in Alya's batch processing system. When a worker crashes mid-processing, other workers detect the failure and recover the abandoned rows.

## Prerequisites

Start the infrastructure from the `jobs/` directory:

```bash
cd jobs
docker compose up -d
```

## How Recovery Works

1. Each worker instance has a unique ID and maintains a heartbeat in Redis (TTL: 60s)
2. When processing a row, workers track the row ID in a Redis SET
3. When a worker finishes a row, it removes the row ID from the SET
4. If a worker crashes, its heartbeat expires
5. Other workers periodically scan for workers without heartbeats
6. Rows from dead workers are reset to 'queued' and reprocessed

## Verification Steps

### Step 1: Submit a Batch

```bash
go run . --mode=submit
```

Output shows the batch ID and instructions:

```
=================================================
Batch submitted
=================================================
Batch ID: abc123...
Rows:     20
Expected processing time per row: 2s

Next steps:
  1. Start a worker: go run . --mode=worker
  2. After ~5 rows, kill the worker (Ctrl+C or kill)
  3. Check status: go run . --mode=status --batch=abc123...
  4. Start another worker to trigger recovery
```

### Step 2: Start Worker A

In a new terminal:

```bash
go run . --mode=worker
```

Output shows processing progress:

```
=================================================
Worker started
=================================================
Instance ID: hostname-12345-1234567890
PID:         12345

To simulate a crash, kill this process:
  kill 12345

Processing rows...
-------------------------------------------------
[10:30:01] Row 1: START processing (worker: hostname-12345)
[10:30:03] Row 1: DONE processing
[10:30:03] Row 2: START processing (worker: hostname-12345)
...
```

### Step 3: Kill Worker A

After 4-5 rows are processed, kill the worker:

```bash
kill <pid>
# or press Ctrl+C
```

### Step 4: Check Status

```bash
go run . --mode=status --batch=<batch-id>
```

Output shows stuck rows:

```
=================================================
Status Report
=================================================

Batch: abc123...
-------------------------------------------------
Row status counts:
  success:   4
  inprog:    1
  queued:    15

Rows in 'inprog' status: 1 (these may be stuck)

Redis worker tracking keys:
-------------------------------------------------

Active workers (have heartbeat):
  (none - heartbeat expired)

Worker row sets (rows being processed):
  hostname-12345-1234567890 [DEAD (no heartbeat - rows will be recovered)]
    Rows: [5]
```

### Step 5: Start Worker B

```bash
go run . --mode=worker
```

Worker B detects the dead worker and recovers the rows:

```
=================================================
Worker started
=================================================
Instance ID: hostname-67890-9876543210
PID:         67890

Processing rows...
-------------------------------------------------
[10:32:05] Row 5: START processing (worker: hostname-67890)  <-- Recovered row
[10:32:07] Row 5: DONE processing
[10:32:07] Row 6: START processing (worker: hostname-67890)
...
```

### Step 6: Verify Completion

```bash
go run . --mode=status --batch=<batch-id>
```

Output shows all rows completed:

```
=================================================
Status Report
=================================================

Batch: abc123...
-------------------------------------------------
Row status counts:
  success:   20

Redis worker tracking keys:
-------------------------------------------------
  (no worker keys found)
```

## Recovery Timeline

- Heartbeat TTL: 60 seconds
- Heartbeat refresh interval: 30 seconds
- Recovery check interval: 60 seconds

When a worker crashes:

1. **T+0s**: Worker A crashes, heartbeat key still exists in Redis
2. **T+60s**: Heartbeat expires (TTL)
3. **T+60-120s**: Worker B's recovery check finds dead worker
4. **T+60-120s**: Worker B resets stuck rows to 'queued'
5. **T+60-120s**: Worker B processes recovered rows

## Cleanup

To reset the environment:

```bash
# Stop containers
cd jobs
docker compose down -v

# Restart
docker compose up -d
```
