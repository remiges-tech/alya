package jobs_test

import (
	"errors"
	"log"
	"testing"

	"github.com/remiges-tech/alya/jobs"
	"github.com/remiges-tech/alya/jobs/pg/batchsqlc"
	"github.com/remiges-tech/alya/wscutils"
	"github.com/remiges-tech/logharbour/logharbour"
	"github.com/stretchr/testify/assert"
)

type mockSlowQueryProcessor struct{}

func (m *mockSlowQueryProcessor) DoSlowQuery(initBlock jobs.InitBlock, context jobs.JSONstr, input jobs.JSONstr) (batchsqlc.StatusEnum, jobs.JSONstr, []wscutils.ErrorMessage, map[string]string, error) {
	result, err := jobs.NewJSONstr("{}")
	if err != nil {
		return batchsqlc.StatusEnumFailed, jobs.JSONstr{}, nil, nil, err
	}
	return batchsqlc.StatusEnumSuccess, result, nil, nil, nil
}

func (m *mockSlowQueryProcessor) MarkDone(initBlock jobs.InitBlock, context jobs.JSONstr, details jobs.BatchDetails_t) error {
	// This is a mock implementation, so we'll just return nil(no error)
	return nil
}

func TestRegisterSlowQueryProcessor(t *testing.T) {
	// Create a test logger
	loggerCtx := &logharbour.LoggerContext{}
	logger := logharbour.NewLogger(loggerCtx, "test", log.Writer())
	jm := jobs.NewJobManager(nil, nil, nil, logger, nil)

	// Test registering a new processor
	err := jm.RegisterProcessorSlowQuery("app1", "op1", &mockSlowQueryProcessor{})
	assert.NoError(t, err)

	// Test registering a duplicate processor
	err = jm.RegisterProcessorSlowQuery("app1", "op1", &mockSlowQueryProcessor{})
	assert.Error(t, err)
	assert.True(t, errors.Is(err, jobs.ErrProcessorAlreadyRegistered))

	// Test registering a different processor for the same app but different op
	err = jm.RegisterProcessorSlowQuery("app1", "op2", &mockSlowQueryProcessor{})
	assert.NoError(t, err)

	// Test registering a different processor for a different app
	err = jm.RegisterProcessorSlowQuery("app2", "op1", &mockSlowQueryProcessor{})
	assert.NoError(t, err)
}
