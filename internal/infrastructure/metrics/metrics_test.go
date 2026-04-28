package metrics

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"

	"github.com/quotahub/ucpqa/internal/domain"
)

func TestNew(t *testing.T) {
	m := New()

	if m == nil {
		t.Fatal("New() returned nil")
	}
	if m.registry == nil {
		t.Error("registry is nil")
	}
	if m.usageGauge == nil {
		t.Error("usageGauge is nil")
	}
	if m.resetGauge == nil {
		t.Error("resetGauge is nil")
	}
	if m.httpRequests == nil {
		t.Error("httpRequests is nil")
	}
	if m.httpDuration == nil {
		t.Error("httpDuration is nil")
	}
}

func TestUpdateFromSnapshot(t *testing.T) {
	m := New()

	resetTime := time.Now().Add(5 * time.Hour)
	snapshot := domain.AccountSnapshot{
		Platform:     "minimax",
		AccountAlias: "test-account",
		Quotas: map[domain.Tier]domain.QuotaTier{
			domain.Tier5H: {Used: 10, Total: 100, ResetAt: resetTime},
			domain.Tier1W: {Used: 20, Total: 200, ResetAt: resetTime},
			domain.Tier1M: {Used: 30, Total: 300, ResetAt: resetTime},
		},
		Version: 1,
	}

	m.UpdateFromSnapshot(snapshot)

	expectedMetrics := []struct {
		name   string
		labels prometheus.Labels
		value  float64
	}{
		{
			name:   "coding_plan_usage_value",
			labels: prometheus.Labels{"platform": "minimax", "account": "test-account", "tier": "5H", "type": "used"},
			value:  10,
		},
		{
			name:   "coding_plan_usage_value",
			labels: prometheus.Labels{"platform": "minimax", "account": "test-account", "tier": "5H", "type": "total"},
			value:  100,
		},
		{
			name:   "coding_plan_usage_value",
			labels: prometheus.Labels{"platform": "minimax", "account": "test-account", "tier": "1W", "type": "used"},
			value:  20,
		},
		{
			name:   "coding_plan_usage_value",
			labels: prometheus.Labels{"platform": "minimax", "account": "test-account", "tier": "1W", "type": "total"},
			value:  200,
		},
		{
			name:   "coding_plan_usage_value",
			labels: prometheus.Labels{"platform": "minimax", "account": "test-account", "tier": "1M", "type": "used"},
			value:  30,
		},
		{
			name:   "coding_plan_usage_value",
			labels: prometheus.Labels{"platform": "minimax", "account": "test-account", "tier": "1M", "type": "total"},
			value:  300,
		},
		{
			name:   "coding_plan_reset_timestamp",
			labels: prometheus.Labels{"platform": "minimax", "account": "test-account", "tier": "5H"},
			value:  float64(resetTime.Unix()),
		},
		{
			name:   "coding_plan_reset_timestamp",
			labels: prometheus.Labels{"platform": "minimax", "account": "test-account", "tier": "1W"},
			value:  float64(resetTime.Unix()),
		},
		{
			name:   "coding_plan_reset_timestamp",
			labels: prometheus.Labels{"platform": "minimax", "account": "test-account", "tier": "1M"},
			value:  float64(resetTime.Unix()),
		},
	}

	for _, em := range expectedMetrics {
		t.Run(em.name, func(t *testing.T) {
			var metric dto.Metric
			var found bool

			if em.name == "coding_plan_usage_value" {
				gauge, err := m.usageGauge.GetMetricWithLabelValues(
					em.labels["platform"],
					em.labels["account"],
					em.labels["tier"],
					em.labels["type"],
				)
				if err != nil {
					t.Fatalf("failed to get gauge: %v", err)
				}
				gauge.Write(&metric)
				found = true
			} else {
				gauge, err := m.resetGauge.GetMetricWithLabelValues(
					em.labels["platform"],
					em.labels["account"],
					em.labels["tier"],
				)
				if err != nil {
					t.Fatalf("failed to get gauge: %v", err)
				}
				gauge.Write(&metric)
				found = true
			}

			if !found {
				t.Fatalf("metric %s with labels %v not found", em.name, em.labels)
			}

			if *metric.Gauge.Value != em.value {
				t.Errorf("expected value %f for %s %v, got %f",
					em.value, em.name, em.labels, *metric.Gauge.Value)
			}
		})
	}
}

func TestHTTPMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)

	m := New()

	router := gin.New()
	router.Use(m.HTTPMiddleware())
	router.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})
	router.POST("/test", func(c *gin.Context) {
		c.String(http.StatusCreated, "created")
	})

	t.Run("GET request recorded", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/test", nil)
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", w.Code)
		}

		var metric dto.Metric
		counter, err := m.httpRequests.GetMetricWithLabelValues("GET", "/test", "200")
		if err != nil {
			t.Fatalf("failed to get counter: %v", err)
		}
		counter.Write(&metric)

		if *metric.Counter.Value != 1 {
			t.Errorf("expected counter value 1, got %f", *metric.Counter.Value)
		}
	})

	t.Run("POST request recorded", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/test", nil)
		router.ServeHTTP(w, req)

		if w.Code != http.StatusCreated {
			t.Errorf("expected status 201, got %d", w.Code)
		}

		var metric dto.Metric
		counter, err := m.httpRequests.GetMetricWithLabelValues("POST", "/test", "201")
		if err != nil {
			t.Fatalf("failed to get counter: %v", err)
		}
		counter.Write(&metric)

		if *metric.Counter.Value != 1 {
			t.Errorf("expected counter value 1, got %f", *metric.Counter.Value)
		}
	})

	t.Run("request duration recorded", func(t *testing.T) {
		histogram := m.httpDuration.WithLabelValues("GET", "/test")

		ch := make(chan prometheus.Metric, 1)
		histogram.(prometheus.Histogram).Collect(ch)
		m := <-ch

		var metric dto.Metric
		m.Write(&metric)

		if len(metric.Histogram.Bucket) == 0 {
			t.Error("histogram has no buckets")
		}
	})
}

func TestHandler(t *testing.T) {
	m := New()

	resetTime := time.Now().Add(5 * time.Hour)
	snapshot := domain.AccountSnapshot{
		Platform:     "codex",
		AccountAlias: "codex-account",
		Quotas: map[domain.Tier]domain.QuotaTier{
			domain.Tier5H: {Used: 5, Total: 50, ResetAt: resetTime},
		},
		Version: 1,
	}
	m.UpdateFromSnapshot(snapshot)

	handler := m.Handler()

	req, _ := http.NewRequest("GET", "/metrics", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	content := w.Body.String()

	expectedStrings := []string{
		"coding_plan_usage_value",
		`platform="codex"`,
		`account="codex-account"`,
		`tier="5H"`,
		`type="used"`,
		`type="total"`,
		"coding_plan_reset_timestamp",
	}

	for _, expected := range expectedStrings {
		if !strings.Contains(content, expected) {
			t.Errorf("expected metrics output to contain %q, got:\n%s", expected, content)
		}
	}
}

func TestMetricsRegistered(t *testing.T) {
	m := New()

	m.usageGauge.WithLabelValues("test", "test", "5H", "used").Set(1)
	m.resetGauge.WithLabelValues("test", "test", "5H").Set(1)
	m.httpRequests.WithLabelValues("GET", "/test", "200").Inc()
	m.httpDuration.WithLabelValues("GET", "/test").Observe(0.1)

	collectors, err := m.registry.Gather()
	if err != nil {
		t.Fatalf("failed to gather collectors: %v", err)
	}
	collectorNames := make(map[string]bool)
	for _, c := range collectors {
		collectorNames[*c.Name] = true
	}

	expectedCollectors := []string{
		"coding_plan_usage_value",
		"coding_plan_reset_timestamp",
		"http_requests_total",
		"http_request_duration_seconds",
	}

	for _, name := range expectedCollectors {
		if !collectorNames[name] {
			t.Errorf("expected collector %q to be registered", name)
		}
	}
}

func TestUpdateFromSnapshotIgnoresUnsupportedTiers(t *testing.T) {
	m := New()

	resetTime := time.Now().Add(5 * time.Hour)
	unsupportedTier := domain.Tier("UNSUPPORTED")
	snapshot := domain.AccountSnapshot{
		Platform:     "test",
		AccountAlias: "test-account",
		Quotas: map[domain.Tier]domain.QuotaTier{
			domain.Tier5H:      {Used: 10, Total: 100, ResetAt: resetTime},
			unsupportedTier:   {Used: 999, Total: 999, ResetAt: resetTime},
		},
		Version: 1,
	}

	m.UpdateFromSnapshot(snapshot)

	var metric dto.Metric
	gauge, err := m.usageGauge.GetMetricWithLabelValues("test", "test-account", "UNSUPPORTED", "used")
	if err != nil {
		t.Fatalf("failed to get gauge for unsupported tier: %v", err)
	}
	gauge.Write(&metric)

	if *metric.Gauge.Value != 0 {
		t.Errorf("expected unsupported tier to have value 0 (not registered), got %f", *metric.Gauge.Value)
	}
}

func TestUpdateFromSnapshotOverwritesPreviousValues(t *testing.T) {
	m := New()

	resetTime1 := time.Now()
	resetTime2 := time.Now().Add(10 * time.Hour)

	snapshot1 := domain.AccountSnapshot{
		Platform:     "overwrite",
		AccountAlias: "test",
		Quotas: map[domain.Tier]domain.QuotaTier{
			domain.Tier5H: {Used: 100, Total: 200, ResetAt: resetTime1},
		},
		Version: 1,
	}
	m.UpdateFromSnapshot(snapshot1)

	snapshot2 := domain.AccountSnapshot{
		Platform:     "overwrite",
		AccountAlias: "test",
		Quotas: map[domain.Tier]domain.QuotaTier{
			domain.Tier5H: {Used: 150, Total: 250, ResetAt: resetTime2},
		},
		Version: 2,
	}
	m.UpdateFromSnapshot(snapshot2)

	var usedMetric, totalMetric dto.Metric

	usedGauge, _ := m.usageGauge.GetMetricWithLabelValues("overwrite", "test", "5H", "used")
	usedGauge.Write(&usedMetric)
	if *usedMetric.Gauge.Value != 150 {
		t.Errorf("expected used value 150 after overwrite, got %f", *usedMetric.Gauge.Value)
	}

	totalGauge, _ := m.usageGauge.GetMetricWithLabelValues("overwrite", "test", "5H", "total")
	totalGauge.Write(&totalMetric)
	if *totalMetric.Gauge.Value != 250 {
		t.Errorf("expected total value 250 after overwrite, got %f", *totalMetric.Gauge.Value)
	}
}

func TestFullPath(t *testing.T) {
	gin.SetMode(gin.TestMode)

	m := New()

	router := gin.New()
	router.Use(m.HTTPMiddleware())

	var capturedPath string
	router.GET("/api/v1/resource/:id", func(c *gin.Context) {
		capturedPath = c.FullPath()
		c.String(http.StatusOK, "ok")
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/resource/123", nil)
	router.ServeHTTP(w, req)

	if capturedPath != "/api/v1/resource/:id" {
		t.Errorf("expected full path /api/v1/resource/:id, got %s", capturedPath)
	}
}

func ExampleMetrics_UpdateFromSnapshot() {
	m := New()

	snapshot := domain.AccountSnapshot{
		Platform:     "example",
		AccountAlias: "example-account",
		Quotas: map[domain.Tier]domain.QuotaTier{
			domain.Tier5H: {Used: 42, Total: 100, ResetAt: time.Now().Add(5 * time.Hour)},
		},
		Version: 1,
	}

	m.UpdateFromSnapshot(snapshot)

	fmt.Println("Metrics updated from snapshot")
}

func ExampleMetrics_Handler() {
	m := New()

	handler := m.Handler()
	fmt.Printf("Handler type: %T\n", handler)
}