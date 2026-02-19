package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/remiges-tech/alya/jobs"
	"github.com/remiges-tech/alya/jobs/pg/batchsqlc"
	"github.com/remiges-tech/logharbour/logharbour"
)

const (
	numRows        = 20
	processingTime = 2 * time.Second
)

func main() {
	mode := flag.String("mode", "", "Mode: submit, worker, or status")
	batchID := flag.String("batch", "", "Batch ID (required for status mode)")
	flag.Parse()

	if *mode == "" {
		fmt.Println("Usage: go run . --mode=<submit|worker|status> [--batch=<batch-id>]")
		fmt.Println()
		fmt.Println("Modes:")
		fmt.Println("  submit  - Create a new batch with 20 slow-processing rows")
		fmt.Println("  worker  - Start a JobManager worker instance")
		fmt.Println("  status  - Show batch status and Redis tracking keys")
		os.Exit(1)
	}

	pool := getDb()
	defer pool.Close()

	redisClient := getRedis()
	defer redisClient.Close()

	switch *mode {
	case "submit":
		runSubmit(pool, redisClient)
	case "worker":
		runWorker(pool, redisClient)
	case "status":
		runStatus(pool, redisClient, *batchID)
	default:
		fmt.Printf("Unknown mode: %s\n", *mode)
		os.Exit(1)
	}
}

func runSubmit(pool *pgxpool.Pool, redisClient *redis.Client) {
	minioClient := getMinioClient()
	logger := getLogger()

	jm := jobs.NewJobManager(pool, redisClient, minioClient, logger, nil)

	err := jm.RegisterProcessorBatch("recoverytest", "slowprocess", &SlowProcessor{})
	if err != nil {
		log.Fatal("Failed to register batch processor:", err)
	}

	err = jm.RegisterInitializer("recoverytest", &SlowInitializer{})
	if err != nil {
		log.Fatal("Failed to register initializer:", err)
	}

	batchInput := generateBatchInput(numRows)
	emptyJSON, _ := jobs.NewJSONstr("{}")

	batchID, err := jm.BatchSubmit("recoverytest", "slowprocess", emptyJSON, batchInput, false)
	if err != nil {
		log.Fatal("Failed to submit batch:", err)
	}

	fmt.Println("=================================================")
	fmt.Println("Batch submitted")
	fmt.Println("=================================================")
	fmt.Printf("Batch ID: %s\n", batchID)
	fmt.Printf("Rows:     %d\n", numRows)
	fmt.Printf("Expected processing time per row: %v\n", processingTime)
	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Println("  1. Start a worker: go run . --mode=worker")
	fmt.Println("  2. After ~5 rows, kill the worker (Ctrl+C or kill)")
	fmt.Printf("  3. Check status: go run . --mode=status --batch=%s\n", batchID)
	fmt.Println("  4. Start another worker to trigger recovery")
}

func runWorker(pool *pgxpool.Pool, redisClient *redis.Client) {
	minioClient := getMinioClient()
	logger := getLogger()

	jm := jobs.NewJobManager(pool, redisClient, minioClient, logger, nil)

	err := jm.RegisterProcessorBatch("recoverytest", "slowprocess", &SlowProcessor{})
	if err != nil {
		log.Fatal("Failed to register batch processor:", err)
	}

	err = jm.RegisterInitializer("recoverytest", &SlowInitializer{})
	if err != nil {
		log.Fatal("Failed to register initializer:", err)
	}

	instanceID := jm.InstanceID()
	fmt.Println("=================================================")
	fmt.Println("Worker started")
	fmt.Println("=================================================")
	fmt.Printf("Instance ID: %s\n", instanceID)
	fmt.Printf("PID:         %d\n", os.Getpid())
	fmt.Println()
	fmt.Println("To simulate a crash, kill this process:")
	fmt.Printf("  kill %d\n", os.Getpid())
	fmt.Println()
	fmt.Println("Processing rows...")
	fmt.Println("-------------------------------------------------")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigChan
		fmt.Printf("\n\nReceived signal: %v\n", sig)
		fmt.Println("Simulating abrupt crash (not cleaning up)...")
		os.Exit(1)
	}()

	jm.RunWithContext(ctx)
}

func runStatus(pool *pgxpool.Pool, redisClient *redis.Client, batchID string) {
	ctx := context.Background()

	fmt.Println("=================================================")
	fmt.Println("Status Report")
	fmt.Println("=================================================")

	if batchID != "" {
		printBatchStatus(ctx, pool, batchID)
	} else {
		printRecentBatches(ctx, pool)
	}

	fmt.Println()
	printWorkerKeys(ctx, redisClient)
}

func printBatchStatus(ctx context.Context, pool *pgxpool.Pool, batchID string) {
	queries := batchsqlc.New(pool)

	fmt.Printf("\nBatch: %s\n", batchID)
	fmt.Println("-------------------------------------------------")

	batchUUID, err := uuid.Parse(batchID)
	if err != nil {
		fmt.Printf("Error parsing batch ID: %v\n", err)
		return
	}

	rows, err := queries.GetBatchRowsByBatchID(ctx, batchUUID)
	if err != nil {
		fmt.Printf("Error getting batch rows: %v\n", err)
		return
	}

	statusCounts := make(map[string]int)
	for _, row := range rows {
		statusCounts[string(row.Status)]++
	}

	fmt.Println("Row status counts:")
	for status, count := range statusCounts {
		fmt.Printf("  %-10s %d\n", status+":", count)
	}

	inprogCount := statusCounts["inprog"]
	if inprogCount > 0 {
		fmt.Printf("\nRows in 'inprog' status: %d (these may be stuck)\n", inprogCount)
	}
}

func printRecentBatches(ctx context.Context, pool *pgxpool.Pool) {
	query := `SELECT id, app, op, status FROM batches ORDER BY reqat DESC LIMIT 5`
	rows, err := pool.Query(ctx, query)
	if err != nil {
		fmt.Printf("Error getting recent batches: %v\n", err)
		return
	}
	defer rows.Close()

	fmt.Println("\nRecent batches:")
	fmt.Println("-------------------------------------------------")

	found := false
	for rows.Next() {
		found = true
		var id uuid.UUID
		var app, op, status string
		if err := rows.Scan(&id, &app, &op, &status); err != nil {
			fmt.Printf("Error scanning row: %v\n", err)
			continue
		}
		fmt.Printf("  ID: %s\n", id)
		fmt.Printf("    App:    %s\n", app)
		fmt.Printf("    Op:     %s\n", op)
		fmt.Printf("    Status: %s\n", status)
		fmt.Println()
	}

	if !found {
		fmt.Println("  (no batches found)")
	}
}

func printWorkerKeys(ctx context.Context, redisClient *redis.Client) {
	fmt.Println("Redis worker tracking keys:")
	fmt.Println("-------------------------------------------------")

	// Get all registered workers from the registry
	registryKey := jobs.WorkerRegistryKey()
	instanceIDs, _ := redisClient.SMembers(ctx, registryKey).Result()

	if len(instanceIDs) == 0 {
		fmt.Println("  (no workers registered)")
		return
	}

	fmt.Printf("\nRegistered workers (%d):\n", len(instanceIDs))
	for _, instanceID := range instanceIDs {
		heartbeatKey := jobs.WorkerHeartbeatKey(instanceID)
		exists, _ := redisClient.Exists(ctx, heartbeatKey).Result()

		if exists == 1 {
			ttl, _ := redisClient.TTL(ctx, heartbeatKey).Result()
			fmt.Printf("  %s [ALIVE] (heartbeat TTL: %v)\n", instanceID, ttl)
		} else {
			fmt.Printf("  %s [DEAD (no heartbeat - rows will be recovered)]\n", instanceID)
		}

		// Show active rows if any
		rowsKey := jobs.WorkerRowsKey(instanceID)
		members, _ := redisClient.SMembers(ctx, rowsKey).Result()
		if len(members) > 0 {
			fmt.Printf("    Rows: %v\n", members)
		}
	}
}

func generateBatchInput(numRows int) []jobs.BatchInput_t {
	var inputs []jobs.BatchInput_t

	for i := 1; i <= numRows; i++ {
		input, _ := jobs.NewJSONstr(fmt.Sprintf(`{"row_number": %d}`, i))
		inputs = append(inputs, jobs.BatchInput_t{
			Line:  i,
			Input: input,
		})
	}

	return inputs
}

func getDb() *pgxpool.Pool {
	connStr := "host=localhost port=5432 user=alyatest password=alyatest dbname=alyatest sslmode=disable"
	pool, err := pgxpool.New(context.Background(), connStr)
	if err != nil {
		log.Fatal("Error connecting to database:", err)
	}
	return pool
}

func getRedis() *redis.Client {
	return redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})
}

func getMinioClient() *minio.Client {
	client, err := minio.New("localhost:9000", &minio.Options{
		Creds:  credentials.NewStaticV4("minioadmin", "minioadmin", ""),
		Secure: false,
	})
	if err != nil {
		log.Fatalf("Error creating MinIO client: %v", err)
	}

	bucketName := "batch-output"
	err = client.MakeBucket(context.Background(), bucketName, minio.MakeBucketOptions{})
	if err != nil {
		exists, errExists := client.BucketExists(context.Background(), bucketName)
		if errExists != nil || !exists {
			log.Fatalf("Error creating bucket: %v", err)
		}
	}

	return client
}

func getLogger() *logharbour.Logger {
	lctx := logharbour.NewLoggerContext(logharbour.DefaultPriority)
	return logharbour.NewLogger(lctx, "batch-recovery", os.Stdout)
}
