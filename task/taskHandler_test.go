package task

import (
	"bytes"
	"errors"
	"github.com/gin-gonic/gin"
	"net/http"
	"net/http/httptest"
	"testing"
)

type MockDBHandler struct {
	DatabaseHandler
	shouldFail bool
}

func (m *MockDBHandler) CreateTaskEntry(taskID string) error {
	if m.shouldFail {
		return errors.New("mock error")
	}
	return nil
}

type MockRabbitMQ struct {
	RabbitMQHandler
	shouldFail bool
}

func (m *MockRabbitMQ) PublishTask(taskID string) error {
	if m.shouldFail {
		return errors.New("mock error")
	}
	return nil
}

func TestTaskHandler_startTask(t *testing.T) {
	mockDB := &MockDBHandler{}
	mockMQ := &MockRabbitMQ{}
	taskHandler := NewTaskHandler(mockDB, mockMQ)

	validTaskJSON := `{"id": "task1", "status": "processing", "submittedAt": 1633588857, "startedAt": 1633588857, "completedAt": 0, "resultPath": "", "details": ""}`
	invalidTaskJSON := `{"submittedAt": 1633588857, "startedAt": 1633588857, "completedAt": 0, "resultPath": ""}`

	tests := []struct {
		name          string
		taskJSON      string
		mockDBFail    bool
		mockMQFail    bool
		expectedCode  int
		expectedError string
	}{
		{"valid task", validTaskJSON, false, false, 200, ""},
		{"task json binding failure", invalidTaskJSON, false, false, 400, `{"error":"Invalid Task ID"}`},
		{"database failure", validTaskJSON, true, false, 500, `{"error":"Failed to create task entry in database"}`},
		{"message queue failure", validTaskJSON, false, true, 500, `{"error":"Failed to publish task to message queue"}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockDB.shouldFail = tt.mockDBFail
			mockMQ.shouldFail = tt.mockMQFail

			req, _ := http.NewRequest("POST", "/startTask", bytes.NewBuffer([]byte(tt.taskJSON)))
			rec := httptest.NewRecorder()

			c, router := gin.CreateTestContext(rec)
			router.POST("/startTask", taskHandler.startTask)
			router.ServeHTTP(c.Writer, req)

			if c.Writer.Status() != tt.expectedCode {
				t.Errorf("unexpected status code: got %v want %v", c.Writer.Status(), tt.expectedCode)
			}
			if tt.expectedError != "" && rec.Body.String() != tt.expectedError {
				t.Errorf("unexpected body: got %v want %v", rec.Body.String(), tt.expectedError)
			}
		})
	}
}
