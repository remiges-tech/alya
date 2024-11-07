package jobs

import (
	"encoding/json"

	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/remiges-tech/alya/jobs/pg/batchsqlc"
	"github.com/remiges-tech/alya/wscutils"
)

type BatchJob_t struct {
	App     string
	Op      string
	Batch   uuid.UUID
	RowID   int
	Context JSONstr
	Line    int
	Input   JSONstr
}

// BatchInput_t represents a single input row for a batch job.
type BatchInput_t struct {
	Line  int     `json:"line"`
	Input JSONstr `json:"input"`
}

// maybe combine initblock and initializer
// InitBlock is used to store and manage resources needed for processing batch jobs and slow queries.
type InitBlock interface {
	Close() error
}

// Initializer is an interface that allows applications to initialize and provide
// any necessary resources or configuration for batch processing or slow queries.
// Implementers of this interface should define a struct that holds the required
// resources, and provide an implementation for the Init method to create and
// initialize an instance of that struct (InitBlock).
//
// The Init method is expected to return an InitBlock that can be used by the
// processing functions (BatchProcessor or SlowQueryProcessor) to access the
// initialized resources.
type Initializer interface {
	Init(app string) (InitBlock, error)
}

type SlowQueryProcessor interface {
	DoSlowQuery(InitBlock InitBlock, context JSONstr, input JSONstr) (status batchsqlc.StatusEnum, result JSONstr, messages []wscutils.ErrorMessage, outputFiles map[string]string, err error)
}

type BatchProcessor interface {
	DoBatchJob(InitBlock InitBlock, context JSONstr, line int, input JSONstr) (status batchsqlc.StatusEnum, result JSONstr, messages []wscutils.ErrorMessage, blobRows map[string]string, err error)
	MarkDone(InitBlock InitBlock, context JSONstr, details BatchDetails_t) error
}

type SlowQuery struct {
	Db          *pgxpool.Pool
	Queries     batchsqlc.Querier
	RedisClient *redis.Client
}

type JSONstr struct {
	value string // value is the string representation of the JSON data
	valid bool   // valid is true if the JSON data is valid, false otherwise.
}

// JSONstr is a custom type that represents a JSON string.
// It provides methods to create a new JSONstr from a string,
// convert it back to a string, and check if it contains valid JSON.
func NewJSONstr(s string) (JSONstr, error) {
	if s == "" {
		return JSONstr{value: "{}", valid: true}, nil
	}
	var js json.RawMessage
	err := json.Unmarshal([]byte(s), &js)
	if err != nil {
		return JSONstr{}, err
	}
	return JSONstr{value: s, valid: true}, nil
}

// String returns the string representation of the JSONstr.
func (j JSONstr) String() string {
	return j.value
}

// IsValid returns true if the JSONstr is valid, false otherwise.
func (j JSONstr) IsValid() bool {
	return j.valid
}

// JobManagerConfig holds the configuration for the job manager.
type JobManagerConfig struct {
	BatchChunkNRows        int    // number of rows to send to the batch processor in each chunk
	BatchStatusCacheDurSec int    // duration in seconds to cache the batch status
	BatchOutputBucket      string // bucket name for batch files
}

// BatchDetails_t struct
type BatchDetails_t struct {
	ID          string
	App         string
	Op          string
	Context     JSONstr
	Status      batchsqlc.StatusEnum
	OutputFiles map[string]string
	NSuccess    int
	NFailed     int
	NAborted    int
}
