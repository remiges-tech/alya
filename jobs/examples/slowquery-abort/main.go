package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/remiges-tech/alya/jobs"
	"github.com/remiges-tech/alya/jobs/examples"
	"github.com/remiges-tech/alya/jobs/pg/batchsqlc"
	"github.com/remiges-tech/alya/wscutils"
	"github.com/remiges-tech/logharbour/logharbour"
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

func (i *BroadsideInitializer) Init(app string) (jobs.InitBlock, error) {
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

func (ib *InitBlock) IsAlive() (bool, error) {
	// Check if SolrClient connection is working or not
	return ib.SolrClient.Open() == nil, nil
}

type SlowReportProcessor struct {
	SolrClient MockSolrClient
}

func (p *SlowReportProcessor) DoSlowQuery(initBlock jobs.InitBlock, context jobs.JSONstr, input jobs.JSONstr) (status batchsqlc.StatusEnum, result jobs.JSONstr, messages []wscutils.ErrorMessage, outputFiles map[string]string, err error) {
	// Simulate a long-running process
	time.Sleep(10 * time.Second)

	// Return success status
	result, _ = jobs.NewJSONstr(`{"message": "Slow report processed successfully"}`)
	return batchsqlc.StatusEnumSuccess, result, nil, nil, nil
}

func (p *SlowReportProcessor) MarkDone(initBlock jobs.InitBlock, context jobs.JSONstr, details jobs.BatchDetails_t) error {
	fmt.Println("Marking done")
	return nil
}

func main() {
	pool := getDb()

	queries := batchsqlc.New(pool)

	// instantiate redis client
	redisClient := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})

	// Initialize SlowQuery
	slowQuery := jobs.SlowQuery{
		Db:          pool,
		Queries:     queries,
		RedisClient: redisClient,
	}
	fmt.Println(slowQuery.Queries)

	store := examples.CreateMinioClient()

	lctx := logharbour.NewLoggerContext(logharbour.DefaultPriority)
	logger := logharbour.NewLogger(lctx, "JobManager", os.Stdout)

	// Initialize JobManager
	jm := jobs.NewJobManager(pool, redisClient, store, logger, nil)

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
	context, _ := jobs.NewJSONstr(`{"userId": 123}`)
	input, _ := jobs.NewJSONstr(`{"startDate": "2023-01-01", "endDate": "2023-12-31"}`)
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
		status, result, messages, _, err := jm.SlowQueryDone(reqID)
		if err != nil {
			fmt.Println("Error while polling for slow query result:", err)
			return
		}

		if status == jobs.BatchTryLater {
			fmt.Println("Report generation in progress. Trying again in 5 seconds...")
			time.Sleep(5 * time.Second)
			continue
		}

		if status == jobs.BatchSuccess {
			fmt.Println("Report generated successfully:")
			fmt.Println("Result:", result)
			break
		}

		if status == jobs.BatchFailed {
			fmt.Println("Report generation failed:")
			fmt.Println("Error messages:", messages)
			break
		}

		if status == jobs.BatchAborted {
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
