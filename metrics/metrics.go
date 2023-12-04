package metrics

type Metrics interface {
	Register(name, metricType, help string)
	Record(name string, value float64)
	RegisterWithLabels(name, metricType, help string, labels []string)
	RecordWithLabels(name string, value float64, labelValues ...string)
}
