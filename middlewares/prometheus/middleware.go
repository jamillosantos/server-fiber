// Package prometheus provides a Fiber middleware that collects HTTP metrics
// and exposes them in Prometheus format.
//
// The following metrics are collected:
//
//   - <namespace>_requests_total (counter): Total number of completed HTTP requests,
//     labeled by method, status code, and path.
//
//   - <namespace>_requests_in_flight (gauge): Number of HTTP requests currently
//     being processed, labeled by method and path.
//
//   - <namespace>_request_duration_seconds (histogram): HTTP request latency in
//     seconds, labeled by method, status code, and path.
//
//   - <namespace>_request_size_bytes (histogram): Size of incoming HTTP request
//     bodies in bytes, labeled by method, status code, and path.
//
//   - <namespace>_response_size_bytes (histogram): Size of outgoing HTTP response
//     bodies in bytes, labeled by method, status code, and path.
//
//   - <namespace>_panics_total (counter): Total number of requests that caused a
//     panic, labeled by method and path. The panic is re-raised after recording
//     so upstream recovery middleware (e.g. fiber/middleware/recover) can handle it.
//
// Usage:
//
//	app := fiber.New()
//	app.Use(prometheus.Middleware())
//
// To use a custom namespace:
//
//	app.Use(prometheus.Middleware(prometheus.WithNamespace("myapp")))
package prometheus

import (
	"strconv"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/prometheus/client_golang/prometheus"
)

var (
	metricHTTPLabels         = []string{"method", "code", "path"}
	metricHTTPInFlightLabels = []string{"method", "path"}

	// DefaultSizeBuckets are the default histogram buckets for request and response
	// size metrics: 100B, 1KB, 10KB, 100KB, 1MB, 10MB, 100MB.
	DefaultSizeBuckets = prometheus.ExponentialBuckets(100, 10, 7)
)

type middlewareOptions struct {
	Namespace            string
	DurationBuckets      []float64
	RequestSizeBuckets   []float64
	ResponseSizeBuckets  []float64
}

// Option configures the Prometheus middleware.
type Option func(options *middlewareOptions)

type httpMetrics struct {
	requestsTotalCounter   *prometheus.CounterVec
	requestsInFlightTotal  *prometheus.GaugeVec
	requestDurationSeconds *prometheus.HistogramVec
	requestSizeBytes       *prometheus.HistogramVec
	responseSizeBytes      *prometheus.HistogramVec
	panicsTotalCounter     *prometheus.CounterVec
}

var (
	mapMetrics = make(map[string]*httpMetrics)
)

// WithNamespace sets the metric namespace (prefix). Defaults to "http".
// For example, WithNamespace("myapp") produces metrics like myapp_requests_total.
func WithNamespace(namespace string) Option {
	return func(options *middlewareOptions) {
		options.Namespace = namespace
	}
}

// WithBuckets sets the histogram buckets for request duration. Defaults to prometheus.DefBuckets.
func WithBuckets(buckets []float64) Option {
	return func(options *middlewareOptions) {
		options.DurationBuckets = buckets
	}
}

// WithRequestSizeBuckets sets the histogram buckets for request body size. Defaults to DefaultSizeBuckets.
func WithRequestSizeBuckets(buckets []float64) Option {
	return func(options *middlewareOptions) {
		options.RequestSizeBuckets = buckets
	}
}

// WithResponseSizeBuckets sets the histogram buckets for response body size. Defaults to DefaultSizeBuckets.
func WithResponseSizeBuckets(buckets []float64) Option {
	return func(options *middlewareOptions) {
		options.ResponseSizeBuckets = buckets
	}
}

// Middleware returns a Fiber handler that records HTTP metrics for each request.
// Metrics are registered with the default Prometheus registry on first call per namespace.
// Panics if called twice with the same namespace after the registry has been reset.
func Middleware(options ...Option) fiber.Handler {
	var opts = middlewareOptions{
		Namespace:           "http",
		DurationBuckets:     prometheus.DefBuckets,
		RequestSizeBuckets:  DefaultSizeBuckets,
		ResponseSizeBuckets: DefaultSizeBuckets,
	}
	for _, opt := range options {
		opt(&opts)
	}

	metric, ok := mapMetrics[opts.Namespace]
	if !ok {
		metric = &httpMetrics{
			requestsTotalCounter: prometheus.NewCounterVec(prometheus.CounterOpts{
				Namespace: opts.Namespace,
				Name:      "requests_total",
				Help:      "Total number of HTTP requests completed.",
			}, metricHTTPLabels),
			requestsInFlightTotal: prometheus.NewGaugeVec(prometheus.GaugeOpts{
				Namespace: opts.Namespace,
				Name:      "requests_in_flight",
				Help:      "Number of HTTP requests currently being processed.",
			}, metricHTTPInFlightLabels),
			requestDurationSeconds: prometheus.NewHistogramVec(prometheus.HistogramOpts{
				Namespace: opts.Namespace,
				Name:      "request_duration_seconds",
				Help:      "HTTP request latencies in seconds.",
				Buckets:   opts.DurationBuckets,
			}, metricHTTPLabels),
			requestSizeBytes: prometheus.NewHistogramVec(prometheus.HistogramOpts{
				Namespace: opts.Namespace,
				Name:      "request_size_bytes",
				Help:      "HTTP request sizes in bytes.",
				Buckets:   opts.RequestSizeBuckets,
			}, metricHTTPLabels),
			responseSizeBytes: prometheus.NewHistogramVec(prometheus.HistogramOpts{
				Namespace: opts.Namespace,
				Name:      "response_size_bytes",
				Help:      "HTTP response sizes in bytes.",
				Buckets:   opts.ResponseSizeBuckets,
			}, metricHTTPLabels),
			panicsTotalCounter: prometheus.NewCounterVec(prometheus.CounterOpts{
				Namespace: opts.Namespace,
				Name:      "panics_total",
				Help:      "Total number of HTTP requests that caused a panic.",
			}, metricHTTPInFlightLabels),
		}
		prometheus.MustRegister(
			metric.requestsTotalCounter,
			metric.requestsInFlightTotal,
			metric.requestDurationSeconds,
			metric.requestSizeBytes,
			metric.responseSizeBytes,
			metric.panicsTotalCounter,
		)
		mapMetrics[opts.Namespace] = metric
	}

	return func(ctx fiber.Ctx) error {
		start := time.Now()
		method := ctx.Method()
		path := ctx.Path()

		inflight := metric.requestsInFlightTotal.WithLabelValues(method, path)
		inflight.Inc()
		defer func() {
			inflight.Dec()
			if r := recover(); r != nil {
				metric.panicsTotalCounter.WithLabelValues(method, path).Inc()
				panic(r)
			}
			labelValues := []string{method, strconv.Itoa(ctx.Response().StatusCode()), path}
			metric.requestsTotalCounter.WithLabelValues(labelValues...).Inc()
			metric.requestDurationSeconds.WithLabelValues(labelValues...).Observe(time.Since(start).Seconds())
			metric.requestSizeBytes.WithLabelValues(labelValues...).Observe(float64(len(ctx.Request().Body())))
			metric.responseSizeBytes.WithLabelValues(labelValues...).Observe(float64(len(ctx.Response().Body())))
		}()

		return ctx.Next()
	}
}
