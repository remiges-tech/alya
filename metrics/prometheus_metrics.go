package metrics

import (
	"log"
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// PrometheusMetrics is a structure that implements the Metrics interface using Prometheus as the backend.
// It stores mappings for different Prometheus metric types (Counter, Gauge, Histogram) and their vector counterparts.
type PrometheusMetrics struct {
	counters      map[string]prometheus.Counter
	counterVecs   map[string]*prometheus.CounterVec // New map for CounterVec objects
	gauges        map[string]prometheus.Gauge
	gaugeVecs     map[string]*prometheus.GaugeVec // New map for CounterVec objects
	histograms    map[string]prometheus.Histogram
	histogramVecs map[string]*prometheus.HistogramVec
	customBuckets map[string][]float64 // Stores custom buckets for histograms
}

// NewPrometheusMetrics creates and initializes a new instance of PrometheusMetrics.
// This function sets up the internal maps used to store various types of Prometheus metrics,
// including counters, gauges, histograms, and their labeled (vector) versions, as well as custom buckets for histograms.
func NewPrometheusMetrics() *PrometheusMetrics {
	return &PrometheusMetrics{
		counters:      make(map[string]prometheus.Counter),
		counterVecs:   make(map[string]*prometheus.CounterVec),
		gauges:        make(map[string]prometheus.Gauge),
		gaugeVecs:     make(map[string]*prometheus.GaugeVec),
		histograms:    make(map[string]prometheus.Histogram),
		histogramVecs: make(map[string]*prometheus.HistogramVec),
		customBuckets: make(map[string][]float64),
	}
}

// SetCustomBuckets allows setting custom bucket sizes for histograms.
// requiring finer or broader granularity. The 'name' parameter specifies the metric name, and 'buckets' is a slice
// of float64 values defining the bucket thresholds.
func (p *PrometheusMetrics) SetCustomBuckets(name string, buckets []float64) {
	p.customBuckets[name] = buckets
}

// Register creates and registers a new metric in the Prometheus registry based on the provided type.
// Supported metric types include 'Counter', 'Gauge', and 'Histogram'.
// The method takes the metric 'name', its 'metricType', and a 'help' string describing the metric.
// For 'Histogram' types, it uses custom buckets if they have been set; otherwise, it falls back to default buckets.
func (p *PrometheusMetrics) Register(name, metricType, help string) {
	switch metricType {
	case "Counter":
		// Creating a new Counter metric
		counter := prometheus.NewCounter(prometheus.CounterOpts{
			Name: name,
			Help: help,
		})
		// Registering the Counter with Prometheus
		prometheus.MustRegister(counter)
		// Storing the reference in the counters map
		p.counters[name] = counter

	case "Gauge":
		// Creating a new Gauge metric
		gauge := prometheus.NewGauge(prometheus.GaugeOpts{
			Name: name,
			Help: help,
		})
		// Registering the Gauge with Prometheus
		prometheus.MustRegister(gauge)
		// Storing the reference in the gauges map
		p.gauges[name] = gauge

	case "Histogram":
		buckets, ok := p.customBuckets[name]
		if !ok {
			buckets = prometheus.DefBuckets // Use default buckets if not specified
		}
		histogram := prometheus.NewHistogram(prometheus.HistogramOpts{
			Name:    name,
			Help:    help,
			Buckets: buckets,
		})
		prometheus.MustRegister(histogram)
		p.histograms[name] = histogram
	default:
		// Handle unknown metric type
		log.Printf("Error: Attempted to register unknown metric type '%s' with name '%s'", metricType, name)
	}
}

// Record updates the value of a Prometheus metric without labels.
// It is used for recording values for counters, gauges, and histograms based on the metric 'name'.
// The method identifies the correct metric type and performs the appropriate action: 'Add' for counters,
// 'Set' for gauges, and 'Observe' for histograms. The 'value' parameter is the value to record.
func (p *PrometheusMetrics) Record(name string, value float64) {
	if counter, ok := p.counters[name]; ok {
		counter.Add(value)
		return
	}

	if gauge, ok := p.gauges[name]; ok {
		gauge.Set(value)
		return
	}

	if histogram, ok := p.histograms[name]; ok {
		histogram.Observe(value)
		return
	}

}

// RegisterWithLabels creates and registers a new labeled metric.
// This method is similar to 'Register' but for metrics with labels (like CounterVec, GaugeVec, HistogramVec).
// It takes the metric 'name', 'metricType', a 'help' description, and a slice of 'labels' (the label keys).
// For 'HistogramVec', it respects custom buckets if set for the given metric name.
func (p *PrometheusMetrics) RegisterWithLabels(name, metricType, help string, labels []string) {
	// Creating a new Counter metric with labels
	switch metricType {
	case "Counter":
		counterVec := prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: name,
			Help: help,
		}, labels)
		// Registering the Counter with Prometheus
		prometheus.MustRegister(counterVec)
		// Storing the reference in the counters map
		p.counterVecs[name] = counterVec
	case "Gauge":
		// Creating a new Gauge metric with labels
		gaugeVec := prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: name,
			Help: help,
		}, labels)
		// Registering the Gauge with Prometheus
		prometheus.MustRegister(gaugeVec)
		// Storing the reference in the gaugeVecs map
		p.gaugeVecs[name] = gaugeVec
	case "Histogram":
		// Creating a new Histogram metric with labels
		buckets, ok := p.customBuckets[name]
		if !ok {
			buckets = prometheus.DefBuckets // Use default buckets if not specified
		}
		histogramVec := prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    name,
			Help:    help,
			Buckets: buckets,
		}, labels)
		// Registering the Histogram with Prometheus
		prometheus.MustRegister(histogramVec)
		// Storing the reference in the histogramVecs map
		p.histogramVecs[name] = histogramVec
	}
}

// RecordWithLabels updates the value of a labeled Prometheus metric.
// This method is used for metrics that were registered with labels, such as those created via 'RegisterWithLabels'.
// It finds the appropriate metric based on 'name' and updates it with the given 'value' and 'labelValues'.
// The 'labelValues' are variadic parameters that should match the order and number of labels defined during registration.
func (p *PrometheusMetrics) RecordWithLabels(name string, value float64, labelValues ...string) {
	if counterVec, ok := p.counterVecs[name]; ok {
		counterVec.WithLabelValues(labelValues...).Add(value)
		return
	}

	if gaugeVec, ok := p.gaugeVecs[name]; ok {
		gaugeVec.WithLabelValues(labelValues...).Set(value)
		return
	}

	if histogramVec, ok := p.histogramVecs[name]; ok {
		histogramVec.WithLabelValues(labelValues...).Observe(value)
		return
	}
}

// StartMetricsServer initializes and starts an HTTP server on the specified 'port' to expose Prometheus metrics.
// This server provides an endpoint for Prometheus to scrape the collected metrics.
// Typically it would be used to start a metrics server in a separate goroutine to keep it running independently.
func (p *PrometheusMetrics) StartMetricsServer(port string) {
	http.Handle("/metrics", promhttp.Handler())
	http.ListenAndServe(":"+port, nil)
}
