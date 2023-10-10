package task

import (
	"context"
	"encoding/json"
	"fmt"
	amqp "github.com/rabbitmq/amqp091-go"
	"time"
)

type RabbitMQHandler interface {
	PublishTask(ctx context.Context, taskId string) error
	ConsumeTask() (string, error)
}

type RabbitMQ struct {
	Connection *amqp.Connection
	Channel    *amqp.Channel
	Processors map[string]TaskProcessor
}

type TaskProcessor interface {
	Process(task Task) (Task, error)
}

// NewRabbitMQ creates a new RabbitMQ instance and initializes the connection and channel.
func NewRabbitMQ(connectionString string) (*RabbitMQ, error) {
	conn, err := amqp.Dial(connectionString)
	if err != nil {
		return nil, err
	}

	ch, err := conn.Channel()
	if err != nil {
		return nil, err
	}

	return &RabbitMQ{Connection: conn, Channel: ch}, nil
}

// SetProcessor sets a TaskProcessor for a specific type of task
// The plan is to use it for dependency injection. We should be able to use
// task package from any other package. The developers do not need to
// change the task package for adding a new task type.
func (r *RabbitMQ) SetProcessor(taskType string, processor TaskProcessor) {
	r.Processors[taskType] = processor
}

func (r *RabbitMQ) PublishTask(ctx context.Context, taskId string) error {
	// Declare a durable queue
	queue, err := r.Channel.QueueDeclare(
		"task_queue", // name
		true,         // durable
		false,        // delete when unused
		false,        // exclusive
		false,        // no-wait
		nil,          // arguments
	)
	if err != nil {
		return err
	}

	// Serialize Task to JSON
	body, err := json.Marshal(taskId)
	if err != nil {
		return err
	}

	// Publish the message with context
	err = r.Channel.PublishWithContext(
		ctx,        // passing context
		"",         // exchange
		queue.Name, // routing key
		false,      // mandatory
		false,      // immediate
		amqp.Publishing{
			ContentType: "application/json",
			Body:        body,
		})
	return err
}

func (r *RabbitMQ) ConsumeTask() (*Task, error) {
	// Consume a delivery from the queue
	msgs, err := r.Channel.Consume(
		"task_queue", // queue
		"",           // consumer
		true,         // auto-acknowledgment
		false,        // exclusive
		false,        // no-local
		false,        // no-wait
		nil,          // arguments
	)
	if err != nil {
		return nil, err
	}

	// Use a select to handle the msg channel with a timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	select {
	case d := <-msgs:
		// Received a delivery
		var task Task
		if err := json.Unmarshal(d.Body, &task); err != nil {
			// Failed to de-serialize the delivery
			return nil, err
		}

		// Fetch the appropriate processor from map
		processor, ok := r.Processors[task.Type]
		if !ok {
			return nil, fmt.Errorf("Unknown task type: %s", task.Type)
		}

		// Use the processor to process the task
		task, err = processor.Process(task)
		if err != nil {
			return nil, err
		}

		return &task, nil
	case <-ctx.Done():
		// Timed out waiting for a delivery
		return nil, ctx.Err()
	}
}
