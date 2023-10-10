package task

import (
	"context"
	"github.com/gin-gonic/gin"
	"net/http"
	"time"
)

// TODO:
// 1. add a route
// 2. fit it in alya framework -- request, response, error
// 3. make tasks an importable package -- the developers just need to provide a task type and a processor
// 4. add a tests for the task handler
// 5. add a wrapper function for consumer similar to startTask
// 6. integrate it in alya framework -- db connections, etc should be passed from the main of alya
//    so that db connections can be reused
// 7. Update submittedAt, startedAt, completedAt values
// 8. comments and documentation

// Task holds the details necessary for task processing
type Task struct {
	ID          string      `json:"id"`          // Unique identifier for the task
	Type        string      `json:"type"`        // task type identifier
	Status      string      `json:"status"`      // Represents the status of the task
	SubmittedAt time.Time   `json:"submittedAt"` // The time at which the task was submitted
	StartedAt   time.Time   `json:"startedAt"`   // The time at which the task started processing
	CompletedAt time.Time   `json:"completedAt"` // The when the task finished processing
	ResultPath  string      `json:"resultPath"`  // The location where the task's result is stored
	Details     interface{} `json:"details"`     // Holds any JSON data related to the task
}

// TaskHandler defines the database and RabbitMQ connections
type TaskHandler struct {
	db  DatabaseHandler
	rmq RabbitMQHandler
}

// NewTaskHandler is a constructor for TaskHandler
func NewTaskHandler(db DatabaseHandler, rmq RabbitMQHandler) *TaskHandler {
	return &TaskHandler{
		db:  db,
		rmq: rmq,
	}
}

// startTask is the handler for initiating a new task
func (t *TaskHandler) startTask(c *gin.Context) {
	var task Task

	if err := t.bindJSON(c, &task); err != nil {
		t.errorResponse(c, http.StatusBadRequest, "Invalid JSON data received")
		return
	}

	// Validate task ID here
	if task.ID == "" {
		t.errorResponse(c, http.StatusBadRequest, "Invalid Task ID")
		return
	}

	ctxRmq, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := t.db.CreateTaskEntry(ctxRmq, task); err != nil {
		t.errorResponse(c, http.StatusInternalServerError, "Failed to create task entry in database")
		return
	}

	ctxDb, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// use a context with a timeout for our PublishTask call
	if err := t.rmq.PublishTask(ctxDb, task.ID); err != nil {
		t.errorResponse(c, http.StatusInternalServerError, "Failed to publish task to message queue")
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Task initiated successfully", "task_id": task.ID})
}

// bindJSON is a utility function to bind incoming JSON payload to a struct
func (t *TaskHandler) bindJSON(c *gin.Context, obj interface{}) error {
	if err := c.ShouldBindJSON(&obj); err != nil {
		return err
	}
	return nil
}

// errorResponse is a utility function to set error responses
func (t *TaskHandler) errorResponse(c *gin.Context, statusCode int, errMsg string) {
	c.JSON(statusCode, gin.H{"error": errMsg})
}
