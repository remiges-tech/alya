package main

import (
	"context"
	"fmt"
	"log"
	"os"
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
	minioBucketName = "incoming" // The name of the bucket we want to use
)

// BankAppInitializer is the initializer for the "bankapp" application
type BankAppInitializer struct{}

func (i BankAppInitializer) Init(appName string) (jobs.InitBlock, error) {
	// You can initialize any resources needed for the bank app here
	fmt.Printf("Initializing app: %s\n", appName)
	return nil, nil
}

func (i BankAppInitializer) Close(ib jobs.InitBlock) error {
	// Clean up any resources if needed
	fmt.Println("Closing BankAppInitializer")
	return nil
}

func main() {
	ctx := context.Background()

	// Set up database connection
	dbPool, err := setupDatabase(ctx)
	if err != nil {
		log.Fatalf("Failed to set up database connection: %v", err)
	}
	defer dbPool.Close()

	// Run database migrations
	err = runMigrations(ctx, dbPool)
	if err != nil {
		log.Fatalf("Failed to run database migrations: %v", err)
	}

	// Set up Redis client
	redisClient, err := setupRedisClient(ctx)
	if err != nil {
		log.Fatalf("Failed to set up Redis client: %v", err)
	}
	defer redisClient.Close()

	// Set up Minio client
	minioClient, err := setupMinioClient(ctx)
	if err != nil {
		log.Fatalf("Failed to set up Minio client: %v", err)
	}

	// Ensure the required bucket exists
	err = ensureMinIOBucketExists(ctx, minioClient, minioBucketName)
	if err != nil {
		log.Fatalf("Failed to ensure MinIO bucket exists: %v", err)
	}

	// Create ObjectStore
	objStore := objstore.NewMinioObjectStore(minioClient)

	// Create a new logger
	logger := logharbour.NewLogger(logharbour.NewLoggerContext(logharbour.DefaultPriority), "BankTransactionExample", os.Stdout)

	// Create JobManager
	jm := jobs.NewJobManager(dbPool, redisClient, minioClient, logger, nil)

	// Register the initializer for "bankapp"
	err = jm.RegisterInitializer("bankapp", &BankAppInitializer{})
	if err != nil {
		log.Fatalf("Failed to register initializer: %v", err)
	}

	// Create FileXfrServer
	queries := batchsqlc.New(dbPool)
	fxs := filexfr.NewFileXfrServer(jm, objStore, queries, filexfr.FileXfrConfig{MaxObjectIDLength: 200}, logger)

	// Register file checker
	err = fxs.RegisterFileChk("csv", checkBankTransactionFile)
	if err != nil {
		log.Fatalf("Failed to register file checker: %v", err)
	}

	// Register batch processor
	err = jm.RegisterProcessorBatch("bankapp", "processtransactions", &BankTransactionProcessor{})
	if err != nil {
		log.Fatalf("Failed to register batch processor: %v", err)
	}

	// Process the sample file
	err = processSampleFile(fxs)
	if err != nil {
		log.Fatalf("Failed to process sample file: %v", err)
	}

	// Start the JobManager
	go jm.Run()

	// Wait for batch processing to complete
	time.Sleep(10 * time.Second)

	fmt.Println("Bank transaction batch processing example completed successfully.")
}

func setupDatabase(ctx context.Context) (*pgxpool.Pool, error) {
	connString := "postgres://alyatest:alyatest@localhost:5432/alyatest"
	return pgxpool.New(ctx, connString)
}

func setupRedisClient(ctx context.Context) (*redis.Client, error) {
	return redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	}), nil
}

func setupMinioClient(ctx context.Context) (*minio.Client, error) {
	return minio.New("localhost:9000", &minio.Options{
		Creds:  credentials.NewStaticV4("minioadmin", "minioadmin", ""),
		Secure: false,
	})
}

func runMigrations(ctx context.Context, dbPool *pgxpool.Pool) error {
	conn, err := dbPool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("failed to acquire connection: %v", err)
	}
	defer conn.Release()

	err = jobs.MigrateDatabase(conn.Conn())
	if err != nil {
		return fmt.Errorf("failed to run migrations: %v", err)
	}

	return nil
}

func processSampleFile(fxs *filexfr.FileXfrServer) error {
	fileContents, err := os.ReadFile("testdata/transactions.csv")
	if err != nil {
		return fmt.Errorf("failed to read sample file: %v", err)
	}

	err = fxs.BulkfileinProcess(string(fileContents), "transactions.csv", "csv")
	if err != nil {
		return fmt.Errorf("failed to process sample file: %v", err)
	}

	return nil
}

func ensureMinIOBucketExists(ctx context.Context, minioClient *minio.Client, bucketName string) error {
	exists, err := minioClient.BucketExists(ctx, bucketName)
	if err != nil {
		return fmt.Errorf("failed to check if bucket exists: %v", err)
	}

	if !exists {
		err = minioClient.MakeBucket(ctx, bucketName, minio.MakeBucketOptions{})
		if err != nil {
			return fmt.Errorf("failed to create bucket: %v", err)
		}
		fmt.Printf("Created MinIO bucket: %s\n", bucketName)
	} else {
		fmt.Printf("MinIO bucket already exists: %s\n", bucketName)
	}

	return nil
}
