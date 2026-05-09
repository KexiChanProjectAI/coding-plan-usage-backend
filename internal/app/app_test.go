package app

import (
	"context"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/quotahub/ucpqa/frontend"
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

func TestSPAHandlerServesIndexHTML(t *testing.T) {
	if !frontendBuilt() {
		t.Skip("frontend dist/index.html not embedded; run frontend build first")
	}

	app := newTestApp(t, 18121, 18122)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	resp := httptest.NewRecorder()
	app.apiServer.Handler.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("GET / returned %d, want 200", resp.Code)
	}
	if !strings.Contains(resp.Body.String(), "Coding Plans") {
		t.Fatalf("GET / response does not contain expected app content")
	}
	if got := resp.Header().Get("Cache-Control"); got != "no-cache" {
		t.Fatalf("GET / Cache-Control = %q, want %q", got, "no-cache")
	}
}

func TestSPAHandlerServesAssets(t *testing.T) {
	if !frontendBuilt() {
		t.Skip("frontend dist/index.html not embedded; run frontend build first")
	}

	assetPath, ok := firstEmbeddedAssetPath(t)
	if !ok {
		t.Skip("no embedded /assets/ file found in frontend dist")
	}

	app := newTestApp(t, 18123, 18124)
	req := httptest.NewRequest(http.MethodGet, assetPath, nil)
	resp := httptest.NewRecorder()
	app.apiServer.Handler.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("GET %s returned %d, want 200", assetPath, resp.Code)
	}
	if got := resp.Header().Get("Cache-Control"); got != "public, max-age=31536000, immutable" {
		t.Fatalf("GET %s Cache-Control = %q, want immutable asset cache", assetPath, got)
	}
}

func TestSPAHandlerFallbackToIndexHTML(t *testing.T) {
	if !frontendBuilt() {
		t.Skip("frontend dist/index.html not embedded; run frontend build first")
	}

	app := newTestApp(t, 18125, 18126)
	req := httptest.NewRequest(http.MethodGet, "/some/spa/route", nil)
	resp := httptest.NewRecorder()
	app.apiServer.Handler.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("GET /some/spa/route returned %d, want 200", resp.Code)
	}
	if !strings.Contains(resp.Body.String(), "Coding Plans") {
		t.Fatalf("SPA fallback response does not contain index.html content")
	}
}

func TestSPAAPIPathsReturn404(t *testing.T) {
	app := newTestApp(t, 18127, 18128)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/nonexistent", nil)
	resp := httptest.NewRecorder()
	app.apiServer.Handler.ServeHTTP(resp, req)

	if resp.Code != http.StatusNotFound {
		t.Fatalf("GET /api/v1/nonexistent returned %d, want 404", resp.Code)
	}
	body := resp.Body.String()
	if strings.Contains(strings.ToLower(body), "<html") {
		t.Fatalf("expected JSON 404 for API path, got HTML body")
	}
	if !strings.Contains(body, "not found") {
		t.Fatalf("expected not found JSON body, got %q", body)
	}
}

func TestSPAHandlerNoFrontendBuilt(t *testing.T) {
	// The embedded dist always contains dist/.gitkeep to keep compilation working.
	// In that state (no built index.html), SPA fallback must return 404 JSON.
	if frontendBuilt() {
		t.Skip("frontend index.html is embedded; cannot simulate no-frontend-built state in this binary")
	}

	app := newTestApp(t, 18129, 18130)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	resp := httptest.NewRecorder()
	app.apiServer.Handler.ServeHTTP(resp, req)

	if resp.Code != http.StatusNotFound {
		t.Fatalf("GET / returned %d, want 404 when frontend not built", resp.Code)
	}
	if !strings.Contains(resp.Body.String(), "frontend not built") {
		t.Fatalf("expected frontend not built message, got %q", resp.Body.String())
	}
}

func newTestApp(t *testing.T, apiPort, metricsPort int) *Composition {
	t.Helper()

	builder := &Builder{
		Config: &config.Config{
			Server: config.ServerConfig{
				APIPort:     apiPort,
				MetricsPort: metricsPort,
			},
			Global: config.GlobalConfig{MaxStaleDuration: 5 * time.Minute},
		},
		Store:   store.New(),
		Metrics: metrics.New(),
	}

	app, err := builder.Build()
	if err != nil {
		t.Fatalf("failed to build app: %v", err)
	}
	return app
}

func frontendBuilt() bool {
	distFS, err := fs.Sub(frontend.DistFS, "dist")
	if err != nil {
		return false
	}
	_, err = fs.Stat(distFS, "index.html")
	return err == nil
}

func firstEmbeddedAssetPath(t *testing.T) (string, bool) {
	t.Helper()

	distFS, err := fs.Sub(frontend.DistFS, "dist")
	if err != nil {
		return "", false
	}

	entries, err := fs.ReadDir(distFS, "assets")
	if err != nil {
		return "", false
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasSuffix(name, ".js") || strings.HasSuffix(name, ".css") {
			return "/assets/" + name, true
		}
	}

	return "", false
}

func TestAPICORSDisabledDoesNotSetAllowOrigin(t *testing.T) {
	cfg := &config.Config{
		Server: config.ServerConfig{
			APIPort:     18121,
			MetricsPort: 18122,
			CorsEnabled: false,
		},
		Global: config.GlobalConfig{
			MaxStaleDuration: 5 * time.Minute,
		},
	}

	builder := &Builder{Config: cfg, Store: store.New(), Metrics: metrics.New()}
	app, err := builder.Build()
	if err != nil {
		t.Fatalf("failed to build app: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/usage", nil)
	req.Header.Set("Origin", "http://localhost:5173")
	resp := httptest.NewRecorder()
	app.apiServer.Handler.ServeHTTP(resp, req)

	if got := resp.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("Access-Control-Allow-Origin = %q, want empty when CORS disabled", got)
	}
}

func TestAPICORSEnabledSetsHeadersForGetAndOptions(t *testing.T) {
	allowedOrigin := "https://frontend.example.com"
	cfg := &config.Config{
		Server: config.ServerConfig{
			APIPort:            18131,
			MetricsPort:        18132,
			CorsEnabled:        true,
			CorsAllowedOrigins: []string{allowedOrigin},
			CorsAllowedMethods: []string{"GET", "OPTIONS"},
			CorsAllowedHeaders: []string{"Content-Type", "Authorization"},
		},
		Global: config.GlobalConfig{
			MaxStaleDuration: 5 * time.Minute,
		},
	}

	builder := &Builder{Config: cfg, Store: store.New(), Metrics: metrics.New()}
	app, err := builder.Build()
	if err != nil {
		t.Fatalf("failed to build app: %v", err)
	}

	getReq := httptest.NewRequest(http.MethodGet, "/api/v1/usage", nil)
	getReq.Header.Set("Origin", allowedOrigin)
	getResp := httptest.NewRecorder()
	app.apiServer.Handler.ServeHTTP(getResp, getReq)

	if got := getResp.Header().Get("Access-Control-Allow-Origin"); got != allowedOrigin {
		t.Errorf("GET Access-Control-Allow-Origin = %q, want %q", got, allowedOrigin)
	}

	optionsReq := httptest.NewRequest(http.MethodOptions, "/api/v1/usage", nil)
	optionsReq.Header.Set("Origin", allowedOrigin)
	optionsReq.Header.Set("Access-Control-Request-Method", "GET")
	optionsReq.Header.Set("Access-Control-Request-Headers", "Content-Type")
	optionsResp := httptest.NewRecorder()
	app.apiServer.Handler.ServeHTTP(optionsResp, optionsReq)

	if optionsResp.Code != http.StatusNoContent {
		t.Errorf("OPTIONS status = %d, want %d", optionsResp.Code, http.StatusNoContent)
	}
	if got := optionsResp.Header().Get("Access-Control-Allow-Origin"); got != allowedOrigin {
		t.Errorf("OPTIONS Access-Control-Allow-Origin = %q, want %q", got, allowedOrigin)
	}
	if got := optionsResp.Header().Get("Access-Control-Allow-Methods"); got == "" {
		t.Errorf("OPTIONS Access-Control-Allow-Methods is empty, want non-empty")
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
