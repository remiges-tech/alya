package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/tracelog"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/remiges-tech/alya/jobs"
	"github.com/remiges-tech/alya/jobs/filexfr"
	"github.com/remiges-tech/alya/jobs/objstore"
	"github.com/remiges-tech/alya/jobs/pg/batchsqlc"
	"github.com/remiges-tech/logharbour/logharbour"
)

// LogharbourPgxTracerLogger bridges tracelog.Logger and logharbour.Logger.
// This custom logger allows us to capture and log all SQL queries executed by pgx
// using the logharbour logging system.
//
// Additionally, it allows setting the module name for pgx logs using the WithModule method,
// and supports dynamic adjustment of the log level at runtime.
type LogharbourPgxTracerLogger struct {
	logger   *logharbour.Logger
	logLevel *LogLevel // Reference to the log level variable, allowing dynamic control.
}

// LogLevel defines a thread-safe structure to hold the current log level.
// This allows us to adjust the logging verbosity at runtime in a thread-safe manner.
type LogLevel struct {
	mu    sync.RWMutex
	level tracelog.LogLevel
}

// Set sets the log level in a thread-safe manner.
func (l *LogLevel) Set(level tracelog.LogLevel) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.level = level
}

// Get retrieves the current log level in a thread-safe manner.
func (l *LogLevel) Get() tracelog.LogLevel {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.level
}

// Log implements the tracelog.Logger interface required by pgx.
//
// Internals:
// - Whenever pgx performs logging (e.g., executing a query), it calls this Log method.
// - The method receives the context, log level, message, and additional data (like SQL queries, arguments, durations).
// - We first check the current log level to determine if the message should be logged.
// - We use logharbour's WithModule method to set the module name for pgx logging.
// - Map the tracelog.LogLevel to the appropriate logharbour log levels and log the messages along with any additional data.
func (l *LogharbourPgxTracerLogger) Log(ctx context.Context, level tracelog.LogLevel, msg string, data map[string]any) {
	// Check if the message's level is at or above the current log level.
	currentLevel := l.logLevel.Get()
	if level < currentLevel {
		// Do not log messages below the current log level.
		return
	}

	// Use WithModule to set the module name for pgx logging.
	logWithModule := l.logger.WithModule("pgx")

	switch level {
	case tracelog.LogLevelTrace, tracelog.LogLevelDebug:
		// For trace and debug levels, we use logharbour's debug level.
		logWithModule.Debug1().LogActivity(msg, data)
	case tracelog.LogLevelInfo:
		// For info level, we use logharbour's info level.
		logWithModule.Info().LogActivity(msg, data)
	case tracelog.LogLevelWarn:
		// For warn level, we use logharbour's warning level.
		logWithModule.Warn().LogActivity(msg, data)
	case tracelog.LogLevelError:
		// For error level, we use logharbour's error level and include the error message.
		logWithModule.Error(fmt.Errorf(msg)).LogActivity(msg, data)
	default:
		// For any other levels, default to info.
		logWithModule.Info().LogActivity(msg, data)
	}
}

const (
	minioBucketName = "alya-batch"
)

/*
This example program demonstrates how to implement a file transfer server for batch processing using the Alya framework.

Key steps:
1. Setup Infrastructure:
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
  - When a file is detected, it uses the `FileXfrServer` to validate the file, store it in Minio, store metadata in Postgres, and submit it as a batch job.
*/
func main() {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("Panic occurred: %v\nStack trace:\n%s", r, debug.Stack())
		}
	}()

	log.Println("Starting main function")
	ctx := context.Background()

	// Step 1: Create a logger using Logharbour.
	// This logger will be used throughout the application for consistent logging.
	loggerContext := logharbour.NewLoggerContext(logharbour.DefaultPriority)
	logger := logharbour.NewLogger(loggerContext, "InfiledExample", os.Stdout)

	// Create a LogLevel instance to control pgx logging level at runtime.
	logLevel := &LogLevel{}
	// Set initial log level (e.g., LogLevelInfo).
	logLevel.Set(tracelog.LogLevelInfo)

	// Step 2: Set up the database connection with the custom tracer logger.
	// This ensures that all SQL queries executed via pgx will be captured and logged by logharbour.
	// We pass the logLevel variable to enable dynamic log level adjustment.
	log.Println("Setting up database connection")
	dbPool, err := setupDatabase(ctx, logger, logLevel)
	if err != nil {
		log.Fatalf("Failed to set up database connection: %v", err)
	}
	defer dbPool.Close()

	// Reset database and run migrations for a clean state.
	log.Println("Resetting database")
	if err := resetDatabase(ctx, dbPool); err != nil {
		log.Fatalf("Failed to reset database: %v", err)
	}

	log.Println("Running database migrations")
	if err := runMigrations(ctx, dbPool); err != nil {
		log.Fatalf("Failed to run database migrations: %v", err)
	}

	// Set up Redis client.
	log.Println("Setting up Redis client")
	redisClient, err := setupRedisClient()
	if err != nil {
		log.Fatalf("Failed to set up Redis client: %v", err)
	}
	defer redisClient.Close()

	// Set up Minio client for object storage.
	log.Println("Setting up Minio client")
	minioClient, err := setupMinioClient()
	if err != nil {
		log.Fatalf("Failed to set up Minio client: %v", err)
	}

	// Ensure necessary buckets exist in Minio.
	bucketNames := []string{"incoming", "failed"}
	err = setupBuckets(minioClient, bucketNames)
	if err != nil {
		log.Fatalf("Failed to set up buckets: %v", err)
	}

	// Create JobManager.
	log.Println("Creating JobManager")
	jm := jobs.NewJobManager(dbPool, redisClient, minioClient, logger, nil)

	// Create ObjectStore.
	log.Println("Creating ObjectStore")
	objStore := objstore.NewMinioObjectStore(minioClient)

	// Create FileXfrServer.
	log.Println("Creating FileXfrServer")
	queries := batchsqlc.New(dbPool)
	fileXfrConfig := filexfr.FileXfrConfig{
		MaxObjectIDLength: 200,
		IncomingBucket:    "incoming",
		FailedBucket:      "failed",
	}
	fxs := filexfr.NewFileXfrServer(jm, objStore, queries, fileXfrConfig, logger)

	// Register file checker and batch processor.
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

	// Start the JobManager in a separate goroutine.
	log.Println("Starting JobManager")
	go jm.Run()

	// Run the Infiled daemon.
	log.Println("Running Infiled daemon")
	if err := runInfiled(fxs); err != nil {
		if strings.Contains(err.Error(), "too many open files") {
			log.Fatalf("Fatal error: %v", err)
		}
		log.Printf("Error in Infiled: %v", err)
	}

	// Example of changing the log level at runtime.
	// This could be triggered by a configuration reload or an API call.
	go func() {
		time.Sleep(30 * time.Second) // Wait for 30 seconds before changing the log level.
		log.Println("Changing pgx log level to Debug")
		logLevel.Set(tracelog.LogLevelDebug)
	}()
}

// setupDatabase initializes the database connection and configures
// it to use our custom tracer logger for logging SQL queries.
//
// We pass the logLevel variable to be able to control the logging level at runtime.
func setupDatabase(ctx context.Context, logger *logharbour.Logger, logLevel *LogLevel) (*pgxpool.Pool, error) {
	connString := "postgres://alyatest:alyatest@localhost:5432/alyatest"

	// Parse the connection string to get pgxpool.Config.
	config, err := pgxpool.ParseConfig(connString)
	if err != nil {
		return nil, fmt.Errorf("unable to parse DATABASE_URL: %w", err)
	}

	// Create an instance of our custom tracer logger, passing the log level.
	tracerLogger := &LogharbourPgxTracerLogger{
		logger:   logger,
		logLevel: logLevel,
	}

	// Configure the connection to use the tracer.
	// We set the LogLevel to LogLevelTrace to ensure all messages are sent to our logger,
	// and our logger decides whether to log them based on the dynamic log level.
	config.ConnConfig.Tracer = &tracelog.TraceLog{
		Logger:   tracerLogger,           // Our custom logger that bridges to logharbour.
		LogLevel: tracelog.LogLevelTrace, // Set to the lowest level to capture all logs.
	}

	// Create the pgx connection pool (dbPool) with the configured settings.
	pool, err := pgxpool.NewWithConfig(context.Background(), config)
	if err != nil {
		return nil, fmt.Errorf("unable to create pgx pool: %w", err)
	}

	return pool, nil
}

// resetDatabase drops existing tables to start with a clean slate.
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

// runMigrations runs database migrations to set up the required schema.
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

// setupRedisClient initializes the Redis client.
func setupRedisClient() (*redis.Client, error) {
	return redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	}), nil
}

// setupMinioClient initializes the Minio client for object storage.
func setupMinioClient() (*minio.Client, error) {
	return minio.New("localhost:9000", &minio.Options{
		Creds:  credentials.NewStaticV4("minioadmin", "minioadmin", ""),
		Secure: false,
	})
}

// setupBuckets ensures that the required buckets exist in Minio.
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

// runInfiled sets up and runs the Infiled daemon, which monitors directories
// for incoming files and processes them using the FileXfrServer.
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

	// Create a logger for Infiled using Logharbour.
	loggerContext := logharbour.NewLoggerContext(logharbour.DefaultPriority)
	logger := logharbour.NewLogger(loggerContext, "Infiled", os.Stdout)

	// Initialize Infiled with the FileXfrServer and logger.
	infiled, err := filexfr.NewInfiled(config, fxs, logger)
	if err != nil {
		return fmt.Errorf("failed to create infiled: %w", err)
	}
	fmt.Println("Starting Infiled daemon...")
	return infiled.Run()
}
