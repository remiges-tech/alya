package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/go-redis/redis"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/remiges-tech/alya/batch"
	"github.com/remiges-tech/alya/batch/pg/batchsqlc"
	"github.com/remiges-tech/alya/wscutils"
)

// create mock solr client with open, close and query functions. use interface
type MockSolrClient interface {
	Open() error
	Close() error
	Query(query string) (string, error)
}

type mockSolrClient struct {
}

func (c *mockSolrClient) Open() error {
	return nil
}

func (c *mockSolrClient) Close() error {
	return nil
}

func (c *mockSolrClient) Query(query string) (string, error) {
	return "mock solr result", nil
}

type BroadsideInitializer struct{}

func (i *BroadsideInitializer) Init(app string) (batch.InitBlock, error) {
	solrClient := mockSolrClient{}
	initBlock := &InitBlock{SolrClient: &solrClient}
	return initBlock, nil
}

// ReportProcessor implements the SlowQueryProcessor interface
type BounceReportProcessor struct {
	SolrClient MockSolrClient
}

type InitBlock struct {
	// Add fields for resources like database connections
	SolrClient MockSolrClient
}

func (ib *InitBlock) Close() error {
	// Clean up resources
	ib.SolrClient.Close()
	return nil
}

type SlowReportProcessor struct {
	SolrClient MockSolrClient
}

func (p *SlowReportProcessor) DoSlowQuery(initBlock batch.InitBlock, context batch.JSONstr, input batch.JSONstr) (status batchsqlc.StatusEnum, result batch.JSONstr, messages []wscutils.ErrorMessage, outputFiles map[string]string, err error) {
	// Simulate a long-running process
	time.Sleep(10 * time.Second)

	// Return success status
	return batchsqlc.StatusEnumSuccess, batch.JSONstr(`{"message": "Slow report processed successfully"}`), nil, nil, nil
}

func main() {
	pool := getDb()

	queries := batchsqlc.New(pool)

	// instantiate redis client
	redisClient := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})

	// Initialize SlowQuery
	slowQuery := batch.SlowQuery{
		Db:          pool,
		Queries:     queries,
		RedisClient: redisClient,
	}
	fmt.Println(slowQuery.Queries)

	// Initialize JobManager
	jm := batch.NewJobManager(pool, redisClient)

	// Register the SlowQueryProcessor for the long-running report
	err := jm.RegisterProcessorSlowQuery("broadside", "slowreport", &SlowReportProcessor{})
	if err != nil {
		fmt.Println("Failed to register SlowQueryProcessor:", err)
		return
	}

	bi := BroadsideInitializer{}

	// Register the initializer for the application
	err = jm.RegisterInitializer("broadside", &bi)
	if err != nil {
		// Handle the error
	}

	// Submit a slow query request
	context := batch.JSONstr(`{"userId": 123}`)
	input := batch.JSONstr(`{"startDate": "2023-01-01", "endDate": "2023-12-31"}`)
	reqID, err := jm.SlowQuerySubmit("broadside", "slowreport", context, input)
	if err != nil {
		fmt.Println("Failed to submit slow query:", err)
		return
	}

	fmt.Println("Slow query submitted. Request ID:", reqID)

	// Start the JobManager in a separate goroutine
	go jm.Run()

	// Wait for a short duration before aborting the slow query
	time.Sleep(5 * time.Second)

	// Abort the slow query
	err = jm.SlowQueryAbort(reqID)
	if err != nil {
		fmt.Println("Failed to abort slow query:", err)
		return
	}

	fmt.Println("Slow query aborted.")

	// Poll for the slow query result
	for {
		status, result, messages, err := jm.SlowQueryDone(reqID)
		if err != nil {
			fmt.Println("Error while polling for slow query result:", err)
			return
		}

		if status == batch.BatchTryLater {
			fmt.Println("Report generation in progress. Trying again in 5 seconds...")
			time.Sleep(5 * time.Second)
			continue
		}

		if status == batch.BatchSuccess {
			fmt.Println("Report generated successfully:")
			fmt.Println("Result:", result)
			break
		}

		if status == batch.BatchFailed {
			fmt.Println("Report generation failed:")
			fmt.Println("Error messages:", messages)
			break
		}

		if status == batch.BatchAborted {
			fmt.Println("Report generation aborted.")
			break
		}
	}
}

func getDb() *pgxpool.Pool {
	dbHost := "localhost"
	dbPort := 5432
	dbUser := "alyatest"
	dbPassword := "alyatest"
	dbName := "alyatest"

	connStr := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable", dbHost, dbPort, dbUser, dbPassword, dbName)

	pool, err := pgxpool.New(context.Background(), connStr)
	if err != nil {
		log.Fatal("error connecting db")
	}
	return pool
}
