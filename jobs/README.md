# Alya Batch Package

The Alya Batch Package is a Go library for processing batch jobs and slow queries in a distributed and scalable manner. It provides a framework for registering and executing custom processing functions, managing job statuses, and handling output files.

## Table of Contents
- [Alya Batch Package](#alya-batch-package)
  - [Table of Contents](#table-of-contents)
  - [Prerequisites](#prerequisites)
  - [Installation](#installation)
  - [JobManager](#jobmanager)
  - [Registering Initializers](#registering-initializers)
  - [Registering Processors](#registering-processors)
  - [Submitting Batch Jobs](#submitting-batch-jobs)
  - [Submitting Slow Queries](#submitting-slow-queries)
  - [Checking Job Status](#checking-job-status)
  - [Aborting Jobs](#aborting-jobs)
  - [Example](#example)
  - [Configuration](#configuration)

## Prerequisites
- PostgreSQL 
- Redis 
- Minio 

## Installation

Import the package in your code:

```go
import "github.com/remiges-tech/alya/batch"
```

## JobManager
To start using the Alya Batch Package, you need to initialize a `JobManager` instance. The `JobManager` is responsible for managing the execution of batch jobs and slow queries.

```go
pool := getDb() // Initialize the database connection pool
redisClient := redis.NewClient(&redis.Options{Addr: "localhost:6379"}) // Initialize Redis client
minioClient := createMinioClient() // Initialize Minio client (optional)

jm := batch.NewJobManager(pool, redisClient, minioClient)
```

## Registering Initializers
Initializers are used to set up any necessary resources or configuration for processing batch jobs or slow queries. You need to register an initializer for each application that will use the Alya Batch Package.

```go
err := jm.RegisterInitializer("banking", &BankingInitializer{})
if err != nil {
    log.Fatal("Failed to register initializer:", err)
}
```

## Registering Processors
Processors are custom functions that define how batch jobs or slow queries should be processed. You need to register a processor for each operation type within an application.

```go
err := jm.RegisterProcessorBatch("banking", "process_transactions", &TransactionBatchProcessor{})
if err != nil {
    log.Fatal("Failed to register batch processor:", err)
}

err := jm.RegisterProcessorSlowQuery("banking", "generate_statement", &StatementSlowQueryProcessor{})
if err != nil {
    log.Fatal("Failed to register slow query processor:", err)
}
```

## Submitting Batch Jobs
To submit a batch job, use the `BatchSubmit` method of the `JobManager`. You need to provide the application name, operation type, batch context, batch input data, and a flag indicating whether to wait before processing.

```go
csvFile := "transactions.csv"
batchInput, err := loadBatchInputFromCSV(csvFile)
if err != nil {
    log.Fatal("Failed to load batch input from CSV:", err)
}

batchID, err := jm.BatchSubmit("banking", "process_transactions", batch.JSONstr("{}"), batchInput, false)
if err != nil {
    log.Fatal("Failed to submit batch:", err)
}
```

## Submitting Slow Queries
To submit a slow query, use the `SlowQuerySubmit` method of the `JobManager`. You need to provide the application name, operation type, query context, and query input data.

```go
context := batch.JSONstr(`{"accountID": "1234567890"}`)
input := batch.JSONstr(`{"startDate": "2023-01-01", "endDate": "2023-12-31"}`)

reqID, err := jm.SlowQuerySubmit("banking", "generate_statement", context, input)
if err != nil {
    log.Fatal("Failed to submit slow query:", err)
}
```

## Checking Job Status
To check the status of a batch job or slow query, use the `BatchDone` or `SlowQueryDone` method of the `JobManager`, respectively. These methods return the current status of the job, along with any output files or error messages.

```go
status, batchOutput, outputFiles, nsuccess, nfailed, naborted, err := jm.BatchDone(batchID)
if err != nil {
    log.Fatal("Error while polling for batch status:", err)
}

status, result, messages, err := jm.SlowQueryDone(reqID)
if err != nil {
    log.Fatal("Error while polling for slow query status:", err)
}
```

## Aborting Jobs
To abort a batch job or slow query, use the `BatchAbort` or `SlowQueryAbort` method of the `JobManager`, respectively. These methods will mark the job as aborted and stop any further processing.

```go
err := jm.BatchAbort(batchID)
if err != nil {
    log.Fatal("Failed to abort batch:", err)
}

err := jm.SlowQueryAbort(reqID)
if err != nil {
    log.Fatal("Failed to abort slow query:", err)
}
```

## Example
Here's an example of processing bank transactions from a CSV file:

```go
csvFile := "transactions.csv"
batchInput, err := loadBatchInputFromCSV(csvFile)
if err != nil {
    log.Fatal("Failed to load batch input from CSV:", err)
}

batchID, err := jm.BatchSubmit("banking", "process_transactions", batch.JSONstr("{}"), batchInput, false)
if err != nil {
    log.Fatal("Failed to submit batch:", err)
}

go jm.Run()

status, _, outputFiles, nsuccess, nfailed, naborted, err := jm.BatchDone(batchID)
if err != nil {
    log.Fatal("Error while polling for batch status:", err)
}

fmt.Println("Batch completed with status:", status)
fmt.Println("Output files:", outputFiles)
fmt.Println("Success count:", nsuccess)
fmt.Println("Failed count:", nfailed)
fmt.Println("Aborted count:", naborted)
```

You can find more examples in the `examples` directory of the package.

## Configuration
The Alya Batch Package uses config parameters:

- `ALYA_BATCHCHUNK_NROWS`: The number of rows to fetch in each batch chunk (default: 10).
- `ALYA_BATCHSTATUS_CACHEDUR_SEC`: The duration (in seconds) for which batch status is cached in Redis (default: 100).
```
