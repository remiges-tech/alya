package jobs_test

import (
	"errors"
	"log"
	"strings"
	"testing"

	"github.com/remiges-tech/alya/jobs"
	"github.com/remiges-tech/logharbour/logharbour"
	"github.com/stretchr/testify/assert"
)

func TestJobManagerInstanceID(t *testing.T) {
	loggerCtx := &logharbour.LoggerContext{}
	logger := logharbour.NewLogger(loggerCtx, "test", log.Writer())

	t.Run("instance ID is set on creation", func(t *testing.T) {
		jm := jobs.NewJobManager(nil, nil, nil, logger, nil)
		instanceID := jm.InstanceID()
		assert.NotEmpty(t, instanceID, "instance ID should be set")
	})

	t.Run("instance ID contains hostname", func(t *testing.T) {
		jm := jobs.NewJobManager(nil, nil, nil, logger, nil)
		instanceID := jm.InstanceID()
		// Instance ID format: hostname-PID-timestamp
		parts := strings.Split(instanceID, "-")
		assert.GreaterOrEqual(t, len(parts), 3, "instance ID should have at least 3 parts")
	})

	t.Run("different JobManagers have different instance IDs", func(t *testing.T) {
		jm1 := jobs.NewJobManager(nil, nil, nil, logger, nil)
		jm2 := jobs.NewJobManager(nil, nil, nil, logger, nil)
		assert.NotEqual(t, jm1.InstanceID(), jm2.InstanceID(),
			"different JobManagers should have different instance IDs")
	})
}

func TestRegisterInitializer(t *testing.T) {
	// Create a test logger
	loggerCtx := &logharbour.LoggerContext{}
	logger := logharbour.NewLogger(loggerCtx, "test", log.Writer())
	jm := jobs.NewJobManager(nil, nil, nil, logger, nil)

	// Create a mock initializer
	mockInitializer := &MockInitializer{}

	// Test registering a new initializer
	err := jm.RegisterInitializer("app1", mockInitializer)
	assert.NoError(t, err)

	// Test registering a duplicate initializer
	err = jm.RegisterInitializer("app1", mockInitializer)
	assert.Error(t, err)
	assert.True(t, errors.Is(err, jobs.ErrInitializerAlreadyRegistered))
	assert.Equal(t, "initializer already registered for this app: app=app1", err.Error())
}

type MockInitializer struct{}

func (i *MockInitializer) Init(app string) (jobs.InitBlock, error) {
	return nil, nil
}
