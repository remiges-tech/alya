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

type Initializer interface {
	Init(app string) (InitBlock, error)
}

type SlowQueryProcessor interface {
	DoSlowQuery(InitBlock InitBlock, context JSONstr, input JSONstr) (status batchsqlc.StatusEnum, result JSONstr, messages []wscutils.ErrorMessage, outputFiles map[string]string, err error)
}

type BatchProcessor interface {
	DoBatchJob(InitBlock InitBlock, context JSONstr, line int, input JSONstr) (status BatchStatus_t, result JSONstr, messages []wscutils.ErrorMessage, blobRows map[string]string, err error)
}

type SlowQuery struct {
	Db          *pgxpool.Pool
	Queries     batchsqlc.Querier
	RedisClient *redis.Client
}

type JSONstr string
