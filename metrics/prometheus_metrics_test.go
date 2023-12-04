package metrics

import (
	"testing"
)

func TestRegisterWithLabels(t *testing.T) {
	metrics := NewPrometheusMetrics()

	metrics.RegisterWithLabels("test_metric1", "Counter", "Test metric with labels", []string{"label1", "label2"})

	if _, ok := metrics.counterVecs["test_metric1"]; !ok {
		t.Errorf("Metric 'test_metric' was not registered")
	}
}

func TestRecordWithLabels(t *testing.T) {
	metrics := NewPrometheusMetrics()

	metrics.RegisterWithLabels("test_metric2", "Counter", "Test metric with labels", []string{"label1", "label2"})
	metrics.RecordWithLabels("test_metric", 1.0, "value1", "value2")

	if _, ok := metrics.counterVecs["test_metric2"]; !ok {
		t.Errorf("Metric 'test_metric' was not recorded")
	}
}
