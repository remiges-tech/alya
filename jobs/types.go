package jobs

import (
	"encoding/json"
	"time"

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
	IsAlive() (bool, error)
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
	BatchChunkNRows        int // number of rows to send to the batch processor in each chunk
	BatchStatusCacheDurSec int // duration in seconds to cache the batch status
}

// This struct  response for represents return response for SlowQueryList()
type SlowQueryDetails_t struct {
	Id          string            `json:"id"`
	App         string            `json:"app"`
	Op          string            `json:"op"`
	Inputfile   string            `json:"inputfile"`
	Status      BatchStatus_t     `json:"status"`
	Reqat       time.Time         `json:"requat"`
	Doneat      time.Time         `json:"doneat"`
	Outputfiles map[string]string `json:"outputfiles"`
}

// This struct represents input for list of slow queries and batch queries
type ListInput struct {
	App string  `json:"app" validate:"required,alpha"`
	Op  *string `json:"op,omitempty"`
	Age int32   `json:"age" validate:"required,gt=0"`
}

type BatchDetails_t struct {
	Id          string            `json:"id"`
	App         string            `json:"app"`
	Op          string            `json:"op"`
	Inputfile   string            `json:"inputfile"`
	Status      BatchStatus_t     `json:"status"`
	Reqat       time.Time         `json:"requat"`
	Doneat      time.Time         `json:"doneat"`
	Outputfiles map[string]string `json:"outputfiles"`
	NRows       int32             `json:"nrows"`
	NSuccess    int32             `json:"nsuccess"`
	NFailed     int32             `json:"nfailed"`
	NAborted    int32             `json:"naborted"`
}
