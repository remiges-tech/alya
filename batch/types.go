package batch

import (
	"github.com/go-redis/redis"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/remiges-tech/alya/batch/pg/batchsqlc"
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

// maybe combine initblock and initializer
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
}

type SlowQuery struct {
	Db          *pgxpool.Pool
	Queries     batchsqlc.Querier
	RedisClient *redis.Client
}

type JSONstr string
