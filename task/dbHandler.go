package task

import (
	"context"
	"database/sql"
	"encoding/json"
	"github.com/sqlc-dev/pqtype"
	db "go-framework/task/pg/sqlc-gen"
	"time"
)

type DatabaseHandler interface {
	CreateTaskEntry(ctx context.Context, task Task) error
	GetTaskEntry(ctx context.Context, taskId int64) (Task, error) // updated this line
}

type SQLDatabase struct {
	db *sql.DB
}

type Queries struct {
	*sql.DB
}

type TaskEntry struct {
	ID          int64     `json:"id"`
	Type        string    `json:"type"`
	Status      string    `json:"status"`
	SubmittedAt time.Time `json:"submitted_at"`
	StartedAt   time.Time `json:"started_at"`
	CompletedAt time.Time `json:"completed_at"`
	ResultPath  string    `json:"result_path"`
	Details     string    `json:"details"`
}

func NewSQLDatabase(db *sql.DB) *SQLDatabase {
	return &SQLDatabase{
		db: db,
	}
}

func (s *SQLDatabase) CreateTaskEntry(ctx context.Context, task Task) error {
	q := db.New(s.db)

	// Convert task.Details into JSON
	detailsBytes, err := json.Marshal(task.Details)
	if err != nil {
		return err
	}

	err = q.CreateTaskEntry(ctx, db.CreateTaskEntryParams{
		Type:        task.Type,
		Status:      task.Status,
		SubmittedAt: time.Now(),
		StartedAt:   sql.NullTime{Time: task.StartedAt, Valid: task.StartedAt.IsZero()},
		CompletedAt: sql.NullTime{Time: task.CompletedAt, Valid: task.CompletedAt.IsZero()},
		ResultPath:  sql.NullString{String: task.ResultPath, Valid: task.ResultPath != ""}, // Now this is type NullString
		Details:     pqtype.NullRawMessage{RawMessage: detailsBytes},
	})

	return err
}

// GetTaskEntry fetches task details based on taskId
func (s *SQLDatabase) GetTaskEntry(ctx context.Context, taskId int64) (Task, error) {
	q := db.New(s.db)

	taskEntry, err := q.GetTaskEntry(ctx, int32(taskId))
	if err != nil {
		return Task{}, err
	}

	var details interface{}
	if taskEntry.Details.Valid {
		if err := json.Unmarshal(taskEntry.Details.RawMessage, &details); err != nil {
			return Task{}, err
		}
	}

	task := Task{
		ID:          string(taskEntry.ID),
		Type:        taskEntry.Type,
		Status:      taskEntry.Status,
		SubmittedAt: taskEntry.SubmittedAt,
		StartedAt:   taskEntry.StartedAt.Time,
		CompletedAt: taskEntry.CompletedAt.Time,
		ResultPath:  taskEntry.ResultPath.String,
		Details:     details,
	}

	return task, nil
}
