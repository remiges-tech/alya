package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
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

// IsAlive implements jobs.InitBlock.
func (ib *InitBlock) IsAlive() (bool, error) {
	return ib.SolrClient.Open() == nil, nil
}

func (ib *InitBlock) Close() error {
	// Clean up resources
	ib.SolrClient.Close()
	return nil
}

func (p *BounceReportProcessor) DoSlowQuery(initBlock jobs.InitBlock, context jobs.JSONstr, input jobs.JSONstr) (status batchsqlc.StatusEnum, result jobs.JSONstr, messages []wscutils.ErrorMessage, outputFiles map[string]string, err error) {
	// Parse the context and input JSON
	var contextData struct {
		UserID int `json:"userId"`
	}
	var inputData struct {
		FromEmail string `json:"fromEmail"`
	}

	err = json.Unmarshal([]byte(context.String()), &contextData)
	if err != nil {
		emptyJson, _ := jobs.NewJSONstr("{}")
		return batchsqlc.StatusEnumFailed, emptyJson, nil, nil, err
	}

	err = json.Unmarshal([]byte(input.String()), &inputData)
	if err != nil {
		emptyJson, _ := jobs.NewJSONstr("{}")
		return batchsqlc.StatusEnumFailed, emptyJson, nil, nil, err
	}

	// assert that initBlock is of type InitBlock
	if _, ok := initBlock.(*InitBlock); !ok {
		emptyJson, _ := jobs.NewJSONstr("{}")
		return batchsqlc.StatusEnumFailed, emptyJson, nil, nil, fmt.Errorf("initBlock is not of type InitBlock")
	}

	ib := initBlock.(*InitBlock)
	report, err := ib.SolrClient.Query("")
	if err != nil {
		emptyJson, _ := jobs.NewJSONstr("{}")
		return batchsqlc.StatusEnumFailed, emptyJson, nil, nil, err
	}
	fmt.Printf("Report: %s", report)

	// Example output
	reportResult := fmt.Sprintf("Report generated for user %d, for from email %s",
		contextData.UserID, inputData.FromEmail)
	res := fmt.Sprintf(`{"report": "%s"}`, reportResult)

	result, _ = jobs.NewJSONstr(res)

	outputFiles = map[string]string{"file1": "somefile.txt"}
	return batchsqlc.StatusEnumSuccess, result, nil, outputFiles, nil
}

func (p *BounceReportProcessor) MarkDone(initBlock jobs.InitBlock, context jobs.JSONstr, details jobs.BatchDetails_t) error {
	fmt.Println("Marking done")
	return nil
}

func main() {
	connString := "postgres://alyatest:alyatest@localhost:5432/alyatest"
	conn, err := examples.InitializeDatabase(connString)
	if err != nil {
		log.Fatalf("Database initialization failed: %v", err)
	}
	defer conn.Close(context.Background())

	pool := getDb()

	queries := batchsqlc.New(pool)

	// insertSampleBatchRecord(queries)

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
	fmt.Println(slowQuery.Queries) // just to make compiler happy while I'm developing slowquery module

	// Create a new Minio client instance with the default credentials
	minioClient, err := minio.New("localhost:9000", &minio.Options{
		Creds:  credentials.NewStaticV4("minioadmin", "minioadmin", ""),
		Secure: false,
	})
	if err != nil {
		log.Fatalf("Error creating Minio client: %v", err)
	}
	// Create the test bucket
	bucketName := "batch-output"
	err = minioClient.MakeBucket(context.Background(), bucketName, minio.MakeBucketOptions{})
	if err != nil {
		// Check if the bucket already exists
		exists, err := minioClient.BucketExists(context.Background(), bucketName)
		if err != nil || !exists {
			log.Fatalf("Error creating test bucket: %v", err)
		}
	}

	lctx := logharbour.NewLoggerContext(logharbour.DefaultPriority)
	logger := logharbour.NewLogger(lctx, "JobManager", os.Stdout)

	// Initialize JobManager
	jm := jobs.NewJobManager(pool, redisClient, minioClient, logger, nil)
	// Register the SlowQueryProcessor for the long-running report
	err = jm.RegisterProcessorSlowQuery("broadside", "bouncerpt", &BounceReportProcessor{})
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
	reqID, err := jm.SlowQuerySubmit("broadside", "bouncerpt", context, input)
	if err != nil {
		fmt.Println("Failed to submit slow query:", err)
		return
	}

	fmt.Println("Slow query submitted. Request ID:", reqID)

	// Start the JobManager in a separate goroutine
	go jm.Run()

	// Poll for the slow query result
	for {
		status, result, messages, outputFiles, err := jm.SlowQueryDone(reqID)
		if err != nil {
			fmt.Println("Error while polling for slow query result:", err)
			return
		}
		fmt.Println("outputfiles:", outputFiles)
		fmt.Println("status:", status)
		fmt.Println("result:", result)
		fmt.Println("messages:", messages)

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
