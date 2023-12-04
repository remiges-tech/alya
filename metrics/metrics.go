// Package metrics provides an abstract interface for recording and
// managing various types of metrics within an application. It is designed
// to offer a unified and simple API for common metric operations, such as
// registering and recording standard and labeled metrics.
//
// The Metrics interface defined in this package serves as the foundation
// for implementing specific metrics systems, such as a Prometheus-based
// metrics system.
//
// Key functionalities include:
//   - Register: To define and set up new metrics.
//   - Record: To record values for the standard metrics.
//   - RegisterWithLabels: To create new metrics with associated labels.
//   - RecordWithLabels: To record values for labeled metrics, providing
//     label values dynamically.
//
// Usage Example:
//
//	metricsSystem := metrics.NewPrometheusMetrics() // Assuming a Prometheus implementation
//	metricsSystem.Register("requests_total", "Counter", "Total number of HTTP requests")
//	metricsSystem.Record("requests_total", 1)
//	metricsSystem.RegisterWithLabels("http_requests_total", "Counter", "HTTP requests with method and status", []string{"method", "status"})
//	metricsSystem.RecordWithLabels("http_requests_total", 1, "GET", "200")
package metrics

type Metrics interface {
	Register(name, metricType, help string)
	Record(name string, value float64)
	RegisterWithLabels(name, metricType, help string, labels []string)
	RecordWithLabels(name string, value float64, labelValues ...string)
}
