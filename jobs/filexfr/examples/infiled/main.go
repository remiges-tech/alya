package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"runtime/debug"
	"strings"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/remiges-tech/alya/jobs"
	"github.com/remiges-tech/alya/jobs/filexfr"
	"github.com/remiges-tech/alya/jobs/objstore"
	"github.com/remiges-tech/alya/jobs/pg/batchsqlc"
	"github.com/remiges-tech/logharbour/logharbour"
)

const (
	minioBucketName = "alya-batch"
)

/*
This example program demonstrates how to implement a file transfer server for batch processing using the Alya framework.

1. Setup Infra:
  - Database: Run Postgres server. Run migrations to create tables.
  - Redis Client: Run Redis server.
  - Minio Client: Run Minio server. Create bucket.

2. Initialize Alya components:
  - Logger: Create a logger using Logharbour.
  - JobManager: Initialize the Alya `JobManager`, which manages the processing of batch jobs.
  - ObjectStore: Create an object store instance using Minio for storing files.
  - FileXfrServer: Set up the `FileXfrServer`, which handles file transfers and processing.

3. Register file checker and batch processor:
  - File Checker: Register a file checker function `checkBankTransactionFile` for CSV files. This function validates incoming files and prepares them for batch processing.
  - Batch Processor: Register a batch processor `BankTransactionProcessor` to process the batch jobs submitted by the `FileXfrServer`.

4. Run JobManager:
  - The `JobManager` is started in a separate goroutine to handle batch job processing concurrently.

5. Run infiled daemon:
  - Initialize and run the `Infiled` daemon, which monitors a specified directory for incoming files matching certain patterns.
  - When a file is detected, it uses the `FileXfrServer` to validate the file, store it in Minio, store metadata in Postgres and submit it as a batch job.
*/
func main() {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("Panic occurred: %v\nStack trace:\n%s", r, debug.Stack())
		}
	}()

	log.Println("Starting main function")
	ctx := context.Background()

	// Set up database connection
	log.Println("Setting up database connection")
	dbPool, err := setupDatabase(ctx)
	if err != nil {
		log.Fatalf("Failed to set up database connection: %v", err)
	}
	defer dbPool.Close()

	// Reset database and run migrations
	log.Println("Resetting database")
	if err := resetDatabase(ctx, dbPool); err != nil {
		log.Fatalf("Failed to reset database: %v", err)
	}

	log.Println("Running database migrations")
	if err := runMigrations(ctx, dbPool); err != nil {
		log.Fatalf("Failed to run database migrations: %v", err)
	}

	log.Println("Setting up Redis client")
	redisClient, err := setupRedisClient()
	if err != nil {
		log.Fatalf("Failed to set up Redis client: %v", err)
	}
	defer redisClient.Close()

	log.Println("Setting up Minio client")
	minioClient, err := setupMinioClient()
	if err != nil {
		log.Fatalf("Failed to set up Minio client: %v", err)
	}

	bucketNames := []string{"incoming", "failed"}
	err = setupBuckets(minioClient, bucketNames)
	if err != nil {
		log.Fatalf("Failed to set up buckets: %v", err)
	}

	// Create logger, JobManager, and FileXfrServer
	log.Println("Creating logger")
	logger := logharbour.NewLogger(logharbour.NewLoggerContext(logharbour.DefaultPriority), "InfiledExample", os.Stdout)

	log.Println("Creating JobManager")
	jm := jobs.NewJobManager(dbPool, redisClient, minioClient, logger, nil)

	// Create ObjectStore
	log.Println("Creating ObjectStore")
	objStore := objstore.NewMinioObjectStore(minioClient)

	// Create FileXfrServer
	log.Println("Creating FileXfrServer")
	queries := batchsqlc.New(dbPool)
	fileXfrConfig := filexfr.FileXfrConfig{
		MaxObjectIDLength: 200,
		IncomingBucket:    "incoming",
		FailedBucket:      "failed",
	}
	fxs := filexfr.NewFileXfrServer(jm, objStore, queries, fileXfrConfig)

	// Register file checker and batch processor
	log.Println("Registering file checker")
	err = fxs.RegisterFileChk("csv", checkBankTransactionFile)
	if err != nil {
		log.Fatalf("Failed to register file checker: %v", err)
	}

	log.Println("Registering batch processor")
	err = jm.RegisterProcessorBatch("bankapp", "processtransactions", &BankTransactionProcessor{})
	if err != nil {
		log.Fatalf("Failed to register batch processor: %v", err)
	}

	// Start the JobManager in a separate goroutine
	log.Println("Starting JobManager")
	go jm.Run()

	// Run the Infiled daemon
	log.Println("Running Infiled daemon")
	if err := runInfiled(fxs); err != nil {
		if strings.Contains(err.Error(), "too many open files") {
			log.Fatalf("Fatal error: %v", err)
		}
		log.Printf("Error in Infiled: %v", err)
	}
}

func setupDatabase(ctx context.Context) (*pgxpool.Pool, error) {
	connString := "postgres://alyatest:alyatest@localhost:5432/alyatest"
	return pgxpool.New(ctx, connString)
}

func resetDatabase(ctx context.Context, dbPool *pgxpool.Pool) error {
	conn, err := dbPool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("failed to acquire connection: %w", err)
	}
	defer conn.Release()

	_, err = conn.Exec(ctx, `
		DROP TABLE IF EXISTS batch_files;
		DROP TABLE IF EXISTS batchrows;
		DROP TABLE IF EXISTS batches;
		DROP TYPE IF EXISTS status_enum;
		DROP TABLE IF EXISTS schema_version;
	`)
	if err != nil {
		return fmt.Errorf("failed to drop existing tables: %w", err)
	}

	log.Println("All existing tables dropped successfully")
	return nil
}

func runMigrations(ctx context.Context, dbPool *pgxpool.Pool) error {
	conn, err := dbPool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("failed to acquire connection: %w", err)
	}
	defer conn.Release()

	log.Println("Starting database migration")
	err = jobs.MigrateDatabase(conn.Conn())
	if err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}
	log.Println("Database migration completed successfully")

	return nil
}

func setupRedisClient() (*redis.Client, error) {
	return redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	}), nil
}

func setupMinioClient() (*minio.Client, error) {
	return minio.New("localhost:9000", &minio.Options{
		Creds:  credentials.NewStaticV4("minioadmin", "minioadmin", ""),
		Secure: false,
	})
}

func setupBuckets(minioClient *minio.Client, bucketNames []string) error {
	ctx := context.Background()
	for _, bucketName := range bucketNames {
		exists, err := minioClient.BucketExists(ctx, bucketName)
		if err != nil {
			return fmt.Errorf("failed to check if bucket %s exists: %w", bucketName, err)
		}
		if !exists {
			err = minioClient.MakeBucket(ctx, bucketName, minio.MakeBucketOptions{})
			if err != nil {
				return fmt.Errorf("failed to create bucket %s: %w", bucketName, err)
			}
			log.Printf("Created bucket: %s", bucketName)
		} else {
			log.Printf("Bucket already exists: %s", bucketName)
		}
	}
	return nil
}

func runInfiled(fxs *filexfr.FileXfrServer) error {
	config := filexfr.InfiledConfig{
		WatchDirs: []string{"./testdata"},
		FileTypeMap: []filexfr.FileTypeMapping{
			{Path: "*.csv", Type: "csv"},
			{Path: "*.xlsx", Type: "excel"},
			{Path: "TXN*.csv", Type: "transaction"},
		},
		SleepInterval: 5 * time.Second,
		FileAgeSecs:   10,
	}

	infiled := filexfr.NewInfiled(config, fxs)
	fmt.Println("Starting Infiled daemon...")
	return infiled.Run()
}
