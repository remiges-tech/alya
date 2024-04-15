package batch_test

import (
	"errors"
	"testing"

	"github.com/remiges-tech/alya/batch"
	"github.com/remiges-tech/alya/batch/pg/batchsqlc"
	"github.com/remiges-tech/alya/wscutils"
	"github.com/stretchr/testify/assert"
)

type mockSlowQueryProcessor struct{}

func (m *mockSlowQueryProcessor) DoSlowQuery(initBlock batch.InitBlock, context batch.JSONstr, input batch.JSONstr) (batchsqlc.StatusEnum, batch.JSONstr, []wscutils.ErrorMessage, map[string]string, error) {
	return batchsqlc.StatusEnumSuccess, "", nil, nil, nil
}

func TestRegisterSlowQueryProcessor(t *testing.T) {
	jm := batch.NewJobManager(nil, nil, nil)

	// Test registering a new processor
	err := jm.RegisterProcessorSlowQuery("app1", "op1", &mockSlowQueryProcessor{})
	assert.NoError(t, err)

	// Test registering a duplicate processor
	err = jm.RegisterProcessorSlowQuery("app1", "op1", &mockSlowQueryProcessor{})
	assert.Error(t, err)
	assert.True(t, errors.Is(err, batch.ErrProcessorAlreadyRegistered))

	// Test registering a different processor for the same app but different op
	err = jm.RegisterProcessorSlowQuery("app1", "op2", &mockSlowQueryProcessor{})
	assert.NoError(t, err)

	// Test registering a different processor for a different app
	err = jm.RegisterProcessorSlowQuery("app2", "op1", &mockSlowQueryProcessor{})
	assert.NoError(t, err)
}
