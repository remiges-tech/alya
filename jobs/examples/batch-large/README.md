# Batch Processing Smoke Test

Run this after code changes to verify the batch processing framework works with concurrent workers and multiple batches.

## Programs

**generate_txn/main.go** - Generates 100,000 random transactions (DEPOSIT/WITHDRAWAL) to CSV file.

**test_multi_batch.go** - Processes transactions with concurrent workers and multiple batches. Tests the batch framework.

**calc_balance/main.go** - Calculates expected balance from the CSV file. Use to verify transaction processing.

## Run the Test

```bash
# 1. Generate test data
cd generate_txn && go run main.go && cd ..

# 2. Start infrastructure
docker-compose up -d && sleep 10

# 3. Run test
go run test_multi_batch.go
```

Test processes 100,000 transactions across 10 batches with 10 workers. Completes in 30-40 seconds.

## Expected Output

```
=== All Batches Completed ===
Total Success: 100,000
Total Failed: 0
Throughput: 300-400 transactions/second
```

## Verify Batch Framework

Run this query to verify batch summarization:

```bash
docker exec -it alyabatch-pg psql -U alyatest -d alyatest -c "
WITH batch_summary AS (
    SELECT id, nsuccess, nfailed FROM batches
),
actual_counts AS (
    SELECT batch,
           COUNT(CASE WHEN status = 'success' THEN 1 END) as actual_success,
           COUNT(CASE WHEN status = 'failed' THEN 1 END) as actual_failed
    FROM batchrows
    GROUP BY batch
)
SELECT
    bs.id,
    bs.nsuccess,
    ac.actual_success,
    CASE WHEN bs.nsuccess = ac.actual_success THEN '✓' ELSE '✗' END
FROM batch_summary bs
JOIN actual_counts ac ON bs.id = ac.batch;
"
```

All 10 batches show ✓.

## Verify Transaction Processing

Calculate expected balance:

```bash
cd calc_balance && go run main.go
```

Output shows expected balance from CSV:

```
Final balance: 123456.78
```

Get actual Redis balance:

```bash
docker exec -it alyabatch-redis redis-cli GET "batch:transactions.csv:balance"
```

Both values should match.

## Troubleshooting

Services not running?

```bash
docker-compose ps
```

Both PostgreSQL and Redis show "Up".

Reset and rerun:

```bash
docker-compose down -v
docker-compose up -d && sleep 10
go run test_multi_batch.go
```

## When Failures Occur

Expect 100,000 success, 0 failed.

Failures indicate:
- Redis connection issues
- PostgreSQL connection pool exhaustion
- Worker errors (check logs)

Database verification mismatches indicate:
- Batch summarization bug
- Race condition in concurrent updates

Balance mismatch indicates:
- Transaction processing bug
- Lost updates from race condition
