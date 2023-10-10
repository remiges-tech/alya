package task

import (
	"encoding/json"
	amqp "github.com/rabbitmq/amqp091-go"
	"testing"
)

// MockTaskProcessor is a mock implementation of a task processor for testing
type MockTaskProcessor struct {
	ProcessFunc func(task Task) (Task, error)
}

func (mtp *MockTaskProcessor) Process(task Task) (Task, error) {
	return mtp.ProcessFunc(task)
}

func TestConsumeTask(t *testing.T) {
	// Create a buffered channel to mimic RabbitMQ message consumption
	msgs := make(chan amqp.Delivery, 1)

	task := Task{ID: "1", Type: "mock"}
	body, _ := json.Marshal(task)

	msgs <- amqp.Delivery{Body: body}

	// Set up RabbitMQ with a mock task processor
	rmq := &RabbitMQ{Channel: &amqp.Channel{}}
	rmq.Processors = make(map[string]TaskProcessor)
	rmq.Processors["mock"] = &MockTaskProcessor{
		ProcessFunc: func(task Task) (Task, error) {
			return Task{ID: "1", Type: "mock", Status: "processed"}, nil
		},
	}

	// Call ConsumeTask
	result, err := rmq.ConsumeTask()
	if err != nil {
		t.Errorf("Expected no error, but got %v", err)
	}

	// Assert based on what ConsumeTask should have done with the Task
	if result.Status != "processed" {
		t.Errorf("Expected status to be 'processed', but got %v", result.Status)
	}
}
