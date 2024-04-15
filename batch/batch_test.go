package batch_test

import (
	"errors"
	"testing"

	"github.com/remiges-tech/alya/batch"
	"github.com/remiges-tech/alya/batch/pg/batchsqlc"
	"github.com/remiges-tech/alya/wscutils"
	"github.com/stretchr/testify/assert"
)

type mockBatchProcessor struct{}

func (m *mockBatchProcessor) DoBatchJob(initBlock batch.InitBlock, context batch.JSONstr, line int, input batch.JSONstr) (batchsqlc.StatusEnum, batch.JSONstr, []wscutils.ErrorMessage, map[string]string, error) {
	return batchsqlc.StatusEnumSuccess, "", nil, nil, nil
}
func TestRegisterBatchProcessor(t *testing.T) {
	jm := batch.NewJobManager(nil, nil, nil)

	// Test registering a new processor
	err := jm.RegisterProcessorBatch("app1", "op1", &mockBatchProcessor{})
	assert.NoError(t, err)

	// Test registering a duplicate processor
	err = jm.RegisterProcessorBatch("app1", "op1", &mockBatchProcessor{})
	assert.Error(t, err)
	assert.True(t, errors.Is(err, batch.ErrProcessorAlreadyRegistered))

	// Test registering a different processor for the same app but different op
	err = jm.RegisterProcessorBatch("app1", "op2", &mockBatchProcessor{})
	assert.NoError(t, err)

	// Test registering a different processor for a different app
	err = jm.RegisterProcessorBatch("app2", "op1", &mockBatchProcessor{})
	assert.NoError(t, err)
}
