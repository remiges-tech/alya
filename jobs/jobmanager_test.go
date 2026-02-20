package jobs

import (
	"errors"
	"log"
	"strings"
	"testing"

	"github.com/remiges-tech/logharbour/logharbour"
	"github.com/stretchr/testify/assert"
)

func TestJobManagerInstanceID(t *testing.T) {
	loggerCtx := &logharbour.LoggerContext{}
	logger := logharbour.NewLogger(loggerCtx, "test", log.Writer())

	t.Run("instance ID is set on creation", func(t *testing.T) {
		jm := NewJobManager(nil, nil, nil, logger, nil)
		assert.NotEmpty(t, jm.instanceID, "instance ID should be set")
	})

	t.Run("instance ID contains hostname", func(t *testing.T) {
		jm := NewJobManager(nil, nil, nil, logger, nil)
		// Instance ID format: hostname-PID-timestamp
		parts := strings.Split(jm.instanceID, "-")
		assert.GreaterOrEqual(t, len(parts), 3, "instance ID should have at least 3 parts")
	})

	t.Run("different JobManagers have different instance IDs", func(t *testing.T) {
		jm1 := NewJobManager(nil, nil, nil, logger, nil)
		jm2 := NewJobManager(nil, nil, nil, logger, nil)
		assert.NotEqual(t, jm1.instanceID, jm2.instanceID,
			"different JobManagers should have different instance IDs")
	})
}

func TestRegisterInitializer(t *testing.T) {
	loggerCtx := &logharbour.LoggerContext{}
	logger := logharbour.NewLogger(loggerCtx, "test", log.Writer())
	jm := NewJobManager(nil, nil, nil, logger, nil)

	stubInitializer := &stubInitializer{}

	err := jm.RegisterInitializer("app1", stubInitializer)
	assert.NoError(t, err)

	err = jm.RegisterInitializer("app1", stubInitializer)
	assert.Error(t, err)
	assert.True(t, errors.Is(err, ErrInitializerAlreadyRegistered))
	assert.Equal(t, "initializer already registered for this app: app=app1", err.Error())
}

type stubInitializer struct{}

func (i *stubInitializer) Init(app string) (InitBlock, error) {
	return nil, nil
}
