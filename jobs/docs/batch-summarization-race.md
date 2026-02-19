# Batch Summarization Under Concurrency

Multiple workers can finish rows of the same batch concurrently. When the
last row completes, exactly one worker must summarize the batch -- count
successes and failures, upload output files, mark the batch done. Two
properties make this hard: PostgreSQL Read Committed snapshots can hide
recently committed rows, and only one worker at a time can hold the advisory
lock that guards summarization. This document describes the concurrency
design around these two properties.

## The Problem

A worker that finishes a row enters `summarizeCompletedBatches`. It opens a
transaction, tries to acquire an advisory lock for the batch, then counts
remaining `queued` and `inprog` rows. If both counts are zero, it writes the
summary.

Two workers, s2-1 and s2-4, process the same batch on separate pods. s2-1
finishes its rows first and enters summarization. s2-4 finishes the last row
16ms later.

```
TIME     WORKER  EVENT
-------------------------------------------------------
48.131   s2-1    Acquires advisory lock
48.134   s2-1    Starts inprog count query (snapshot frozen)
48.148   s2-4    Commits last row: inprog -> success
48.153   s2-4    Tries advisory lock -> denied (s2-1 holds it)
48.164   s2-1    Query returns inprogCount=1 (stale snapshot)
```

s2-1 holds the lock but can't see the last row's commit -- its query snapshot
was taken at 48.134, 14ms before the commit at 48.148. s2-4 can see the data
but can't get the lock. Neither summarizes. The batch is stuck.

The core tension: the lock holder can have stale data, and the data holder
can't get the lock. Four design decisions address this.

## 1. Lock Failure Returns a Sentinel Error

`summarizeBatch` returns `ErrBatchLockNotAcquired` when another worker holds
the advisory lock. The caller distinguishes "summarized" from "someone else
is working on it" and rolls back instead of committing an empty transaction.

## 2. Lock Contention Triggers Retry

When a worker gets `ErrBatchLockNotAcquired`, it retries with a 50ms delay --
same behavior as stale-snapshot errors. The lock is transaction-scoped. Once
the holder's transaction ends, the next attempt acquires it. Each retry opens
a fresh transaction with a fresh snapshot, so the retrying worker sees all
rows committed before that point.

## 3. Retry Window

5 attempts, 50ms delay between each. Total window: 250ms. The observed
production race window was 30ms -- the lock holder's query took 30ms to
execute, during which the last row was committed by another worker.

## 4. Periodic Sweep

The retry window is finite. If retries exhaust, the batch has no inline path
to summarization. The polling loop only fetches `queued` rows. Crash recovery
only handles dead workers. Neither finds a batch where all rows reached
terminal status but the batch itself was never summarized.

A background goroutine (`runPeriodicSweep`) wakes every 5-10 minutes
(randomized) and queries for batches in `inprog` with `doneat = NULL` where
no `queued` or `inprog` rows remain (`GetUnsummarizedBatches`). Results feed
into `summarizeCompletedBatches` -- the same code path used during normal
processing. An index on `batches(status)` supports the query.

## Two Layers

Inline retries handle the common case -- lock contention resolves within
250ms. The sweep catches anything retries miss: locks held too long, crashes
during summarization, transient DB errors. A stuck batch is resolved within
10 minutes.

## TODO: Reduce Lock Hold Time

The advisory lock currently spans the entire `summarizeBatch` call -- DB
reads, MinIO uploads, DB write, Redis update, and MarkDone callback. Only the
DB read-then-write needs the lock.

Current lock scope:

```
lock acquired
  DB reads
  MinIO uploads (network I/O, unbounded)
  UpdateBatchSummary (DB write)
  Redis updates
  MarkDone callback (user code, unbounded)
lock released (transaction commit)
```

MinIO uploads can move before the lock. If the transaction rolls back,
orphaned objects are harmless garbage. Redis and MarkDone can move after the
lock. Redis is a cache. MarkDone errors are already swallowed.

Target lock scope:

```
MinIO uploads (no lock)
lock acquired
  DB reads
  UpdateBatchSummary
lock released
Redis updates (no lock)
MarkDone callback (no lock)
```

This drops lock hold time from "DB + network I/O + user code" to "DB reads +
one DB write."
