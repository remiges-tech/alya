package metrics

import (
	"log"
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// PrometheusMetrics implements the Metrics interface for Prometheus
type PrometheusMetrics struct {
	counters      map[string]prometheus.Counter
	counterVecs   map[string]*prometheus.CounterVec // New map for CounterVec objects
	gauges        map[string]prometheus.Gauge
	gaugeVecs     map[string]*prometheus.GaugeVec // New map for CounterVec objects
	histograms    map[string]prometheus.Histogram
	histogramVecs map[string]*prometheus.HistogramVec
	customBuckets map[string][]float64 // Stores custom buckets for histograms
}

// NewPrometheusMetrics creates a new PrometheusMetrics instance
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

// SetCustomBuckets allows setting custom buckets for a specific histogram
func (p *PrometheusMetrics) SetCustomBuckets(name string, buckets []float64) {
	p.customBuckets[name] = buckets
}

// Register creates and registers a new Prometheus metric
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

// Record updates the value of a Prometheus metric
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

// RegisterWithLabels creates and registers a new Prometheus metric with labels
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

// RecordWithLabels updates the value of a Prometheus metric with specific label values
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

// StartMetricsServer starts an HTTP server for Prometheus to scrape
func (p *PrometheusMetrics) StartMetricsServer(port string) {
	http.Handle("/metrics", promhttp.Handler())
	http.ListenAndServe(":"+port, nil)
}
