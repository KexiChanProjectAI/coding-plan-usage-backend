// Package metrics provides Prometheus metrics instrumentation for the quota hub.
package metrics

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/quotahub/ucpqa/internal/domain"
)

// Metrics holds the Prometheus metrics collectors and provides helpers for
// business metrics and HTTP instrumentation.
type Metrics struct {
	registry    *prometheus.Registry
	usageGauge  *prometheus.GaugeVec
	resetGauge  *prometheus.GaugeVec
	httpRequests *prometheus.CounterVec
	httpDuration *prometheus.HistogramVec
}

// New creates a new Metrics instance with all collectors registered.
func New() *Metrics {
	registry := prometheus.NewRegistry()

	// Business metrics: coding_plan_usage_value
	usageGauge := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "coding_plan_usage_value",
			Help: "Current usage value for a coding plan tier",
		},
		[]string{"platform", "account", "tier", "type"},
	)

	// Business metrics: coding_plan_reset_timestamp
	resetGauge := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "coding_plan_reset_timestamp",
			Help: "Unix timestamp of the next reset for a coding plan tier",
		},
		[]string{"platform", "account", "tier"},
	)

	// HTTP metrics: request counts
	httpRequests := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests",
		},
		[]string{"method", "path", "status"},
	)

	// HTTP metrics: request duration
	httpDuration := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "HTTP request duration in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "path"},
	)

	// Register all collectors
	registry.MustRegister(usageGauge)
	registry.MustRegister(resetGauge)
	registry.MustRegister(httpRequests)
	registry.MustRegister(httpDuration)

	return &Metrics{
		registry:    registry,
		usageGauge:  usageGauge,
		resetGauge:  resetGauge,
		httpRequests: httpRequests,
		httpDuration: httpDuration,
	}
}

// UpdateFromSnapshot updates the business metrics gauges from an account snapshot.
// This should be called when a snapshot is published from the store.
func (m *Metrics) UpdateFromSnapshot(snapshot domain.AccountSnapshot) {
	for tier, quota := range snapshot.Quotas {
		if !tier.IsSupported() {
			continue
		}

		// Set used and total usage values
		m.usageGauge.WithLabelValues(
			snapshot.Platform,
			snapshot.AccountAlias,
			tier.String(),
			"used",
		).Set(float64(quota.Used))

		m.usageGauge.WithLabelValues(
			snapshot.Platform,
			snapshot.AccountAlias,
			tier.String(),
			"total",
		).Set(float64(quota.Total))

		// Set reset timestamp
		m.resetGauge.WithLabelValues(
			snapshot.Platform,
			snapshot.AccountAlias,
			tier.String(),
		).Set(float64(quota.ResetAt.Unix()))
	}
}

// HTTPMiddleware returns a Gin middleware that records HTTP request counts and durations.
func (m *Metrics) HTTPMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		// Process request
		c.Next()

		// Record metrics after request is processed
		duration := time.Since(start).Seconds()
		status := strconv.Itoa(c.Writer.Status())
		method := c.Request.Method
		path := c.FullPath()
		if path == "" {
			path = "unknown"
		}

		m.httpRequests.WithLabelValues(method, path, status).Inc()
		m.httpDuration.WithLabelValues(method, path).Observe(duration)
	}
}

// Handler returns an HTTP handler for the /metrics endpoint.
func (m *Metrics) Handler() http.Handler {
	return promhttp.HandlerFor(m.registry, promhttp.HandlerOpts{})
}