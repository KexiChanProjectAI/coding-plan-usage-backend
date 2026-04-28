package app

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/quotahub/ucpqa/internal/config"
	"github.com/quotahub/ucpqa/internal/infrastructure/metrics"
	"github.com/quotahub/ucpqa/internal/infrastructure/store"
)

func TestApplicationBuildsRoutes(t *testing.T) {
	cfg := &config.Config{
		Server: config.ServerConfig{
			APIPort:     18091,
			MetricsPort: 18092,
		},
		Global: config.GlobalConfig{
			MaxStaleDuration: 5 * time.Minute,
		},
	}

	st := store.New()
	m := metrics.New()

	builder := &Builder{
		Config:  cfg,
		Store:   st,
		Metrics: m,
	}

	app, err := builder.Build()
	if err != nil {
		t.Fatalf("failed to build app: %v", err)
	}

	routes := []struct {
		method string
		path   string
	}{
		{"GET", "/api/v1/usage"},
		{"GET", "/ws"},
	}

	for _, route := range routes {
		req := httptest.NewRequest(route.method, route.path, nil)
		resp := httptest.NewRecorder()
		app.apiServer.Handler.ServeHTTP(resp, req)

		if resp.Code == http.StatusNotFound {
			t.Errorf("route %s %s not found (got 404)", route.method, route.path)
		}
	}

	// SSE route: verify route exists by checking it doesn't 404
	go app.SSEBroker().Run()
	sseReq := httptest.NewRequest("GET", "/api/v1/stream", nil)
	sseResp := httptest.NewRecorder()
	ctx, cancel := context.WithCancel(context.Background())
	sseReq = sseReq.WithContext(ctx)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		app.apiServer.Handler.ServeHTTP(sseResp, sseReq)
	}()
	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()
	wg.Wait()
	app.SSEBroker().Stop()
	if sseResp.Code == http.StatusNotFound {
		t.Errorf("route GET /api/v1/stream not found (got 404)")
	}
}

func TestMetricsServerSeparatedFromAPI(t *testing.T) {
	cfg := &config.Config{
		Server: config.ServerConfig{
			APIPort:     18101,
			MetricsPort: 18102,
		},
		Global: config.GlobalConfig{
			MaxStaleDuration: 5 * time.Minute,
		},
	}

	st := store.New()
	m := metrics.New()

	builder := &Builder{
		Config:  cfg,
		Store:   st,
		Metrics: m,
	}

	app, err := builder.Build()
	if err != nil {
		t.Fatalf("failed to build app: %v", err)
	}

	if app.APIAddr() != ":18101" {
		t.Errorf("APIAddr() = %v, want :18101", app.APIAddr())
	}

	if app.MetricsAddr() != ":18102" {
		t.Errorf("MetricsAddr() = %v, want :18102", app.MetricsAddr())
	}

	apiServer := httptest.NewServer(app.apiServer.Handler)
	defer apiServer.Close()

	metricsServer := httptest.NewServer(app.metricsServer.Handler)
	defer metricsServer.Close()

	req := httptest.NewRequest("GET", "/metrics", nil)
	resp := httptest.NewRecorder()
	app.metricsServer.Handler.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Errorf("metrics endpoint returned status %d, want 200", resp.Code)
	}

	apiReq := httptest.NewRequest("GET", "/metrics", nil)
	apiResp := httptest.NewRecorder()
	app.apiServer.Handler.ServeHTTP(apiResp, apiReq)

	if apiResp.Code != http.StatusNotFound {
		t.Errorf("metrics endpoint on API server returned status %d, want 404", apiResp.Code)
	}
}

func TestBuilderValidation(t *testing.T) {
	st := store.New()
	m := metrics.New()

	tests := []struct {
		name    string
		builder *Builder
		wantErr bool
	}{
		{
			name: "nil config",
			builder: &Builder{
				Config:  nil,
				Store:   st,
				Metrics: m,
			},
			wantErr: true,
		},
		{
			name: "nil store",
			builder: &Builder{
				Config:  &config.Config{},
				Store:   nil,
				Metrics: m,
			},
			wantErr: true,
		},
		{
			name: "nil metrics",
			builder: &Builder{
				Config:  &config.Config{},
				Store:   st,
				Metrics: nil,
			},
			wantErr: true,
		},
		{
			name: "all valid",
			builder: &Builder{
				Config:  &config.Config{},
				Store:   st,
				Metrics: m,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := tt.builder.Build()
			if (err != nil) != tt.wantErr {
				t.Errorf("Build() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestCompositionAccessors(t *testing.T) {
	cfg := &config.Config{
		Server: config.ServerConfig{
			APIPort:     18111,
			MetricsPort: 18112,
		},
		Global: config.GlobalConfig{
			MaxStaleDuration: 5 * time.Minute,
		},
	}

	st := store.New()
	m := metrics.New()

	builder := &Builder{
		Config:  cfg,
		Store:   st,
		Metrics: m,
	}

	app, err := builder.Build()
	if err != nil {
		t.Fatalf("failed to build app: %v", err)
	}

	if app.Store() != st {
		t.Error("Store() returned wrong store")
	}

	if app.Metrics() != m {
		t.Error("Metrics() returned wrong metrics")
	}

	if app.WSHub() == nil {
		t.Error("WSHub() returned nil")
	}

	if app.SSEBroker() == nil {
		t.Error("SSEBroker() returned nil")
	}

	if app.SyncManager() != nil {
		t.Error("SyncManager() should be nil when not provided")
	}
}

func TestCompositionRunAndStop(t *testing.T) {
	cfg := &config.Config{
		Server: config.ServerConfig{
			APIPort:     18121,
			MetricsPort: 18122,
		},
		Global: config.GlobalConfig{
			MaxStaleDuration: 5 * time.Minute,
		},
	}

	st := store.New()
	m := metrics.New()

	builder := &Builder{
		Config:  cfg,
		Store:   st,
		Metrics: m,
	}

	app, err := builder.Build()
	if err != nil {
		t.Fatalf("failed to build app: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err = app.Run(ctx)
	if err != nil {
		t.Errorf("Run() error: %v", err)
	}
}
