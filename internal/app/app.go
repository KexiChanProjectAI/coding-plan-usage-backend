package app

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"

	"github.com/quotahub/ucpqa/frontend"
	"github.com/quotahub/ucpqa/internal/config"
	"github.com/quotahub/ucpqa/internal/domain"
	"github.com/quotahub/ucpqa/internal/infrastructure/metrics"
	"github.com/quotahub/ucpqa/internal/infrastructure/store"
	"github.com/quotahub/ucpqa/internal/runtime/broadcaster"
	"github.com/quotahub/ucpqa/internal/runtime/syncmanager"
	"github.com/quotahub/ucpqa/internal/transport/http/api"
	"github.com/quotahub/ucpqa/internal/transport/sse"
	"github.com/quotahub/ucpqa/internal/transport/ws"
)

type Builder struct {
	Config      *config.Config
	Store       *store.Store
	Metrics     *metrics.Metrics
	SyncManager *syncmanager.SyncManager
}

func (b *Builder) Build() (*Composition, error) {
	if b.Config == nil {
		return nil, fmt.Errorf("config is required")
	}
	if b.Store == nil {
		return nil, fmt.Errorf("store is required")
	}
	if b.Metrics == nil {
		return nil, fmt.Errorf("metrics is required")
	}

	sseBroker := sse.New()
	sseHandler := sse.NewHandler(sseBroker)

	refreshAllProviders := func() {
		if b.SyncManager == nil {
			return
		}
		for providerName := range b.Config.Providers {
			b.SyncManager.Refresh(providerName)
		}
	}

	wsHub := ws.NewHub(refreshAllProviders)
	wsHandler := ws.NewHandler(wsHub, ws.Upgrader)
	usageHandler := api.NewUsageHandler(b.Store, b.Config.Global.MaxStaleDuration)

	publishCurrentUsage := func() {
		payload, err := json.Marshal(api.StreamBatchMessage{
			Type: "batch",
			Data: usageHandler.Responses(),
		})
		if err != nil {
			log.Printf("[app] failed to marshal realtime usage batch: %v", err)
			return
		}

		sseBroker.Publish(payload)
		wsHub.Broadcast(payload)
	}
	batchBroadcaster := broadcaster.New(time.Second, publishCurrentUsage)

	previousOnUpdate := b.Store.OnUpdate
	b.Store.OnUpdate = func(snapshot domain.AccountSnapshot) {
		if previousOnUpdate != nil {
			previousOnUpdate(snapshot)
		}

		publishCurrentUsage()
		b.Metrics.UpdateFromSnapshot(snapshot)
	}

	apiServer := b.buildAPIServer(usageHandler, sseHandler, wsHandler)
	metricsServer := b.buildMetricsServer()

	shutdownTimeout := 30 * time.Second
	if b.Config.Global.MaxStaleDuration > 0 {
		shutdownTimeout = b.Config.Global.MaxStaleDuration * 2
		if shutdownTimeout < 30*time.Second {
			shutdownTimeout = 30 * time.Second
		}
	}

	comp := &Composition{
		config:        b.Config,
		store:         b.Store,
		metrics:       b.Metrics,
		sseBroker:     sseBroker,
		sseHandler:    sseHandler,
		wsHub:         wsHub,
		wsHandler:     wsHandler,
		broadcaster:   batchBroadcaster,
		syncManager:   b.SyncManager,
		apiServer:     apiServer,
		metricsServer: metricsServer,
	}

	sc := NewShutdownCoordinator(shutdownTimeout)
	sc.Register(&HTTPShutdownable{Server: apiServer})
	sc.Register(&HTTPShutdownable{Server: metricsServer})
	sc.Register(NewSSEBrokerShutdownable(sseBroker))
	sc.Register(NewWSHubShutdownable(wsHub))
	if b.SyncManager != nil {
		sc.Register(&SyncManagerAdapter{stop: b.SyncManager.Stop})
	}
	comp.shutdownCoordinator = sc

	return comp, nil
}

func (b *Builder) buildAPIServer(usageHandler *api.UsageHandler, sseHandler *sse.Handler, wsHandler *ws.Handler) *http.Server {
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(b.Metrics.HTTPMiddleware())
	if b.Config.Server.CorsEnabled {
		corsConfig := cors.Config{
			AllowOrigins: b.Config.Server.CorsAllowedOrigins,
			AllowMethods: b.Config.Server.CorsAllowedMethods,
			AllowHeaders: b.Config.Server.CorsAllowedHeaders,
		}
		if len(corsConfig.AllowMethods) == 0 {
			corsConfig.AllowMethods = []string{"GET", "OPTIONS"}
		}
		router.Use(cors.New(corsConfig))
	}

	v1 := router.Group("/api/v1")
	{
		v1.GET("/usage", usageHandler.GetUsage)
		v1.GET("/stream", sseHandler.StreamHandler())
	}
	router.GET("/ws", wsHandler.WSHandler())

	distFS, err := fs.Sub(frontend.DistFS, "dist")
	if err != nil {
		log.Printf("[app] failed to access embedded frontend dist: %v", err)
	} else {
		registerSPARoutes(router, distFS)
	}

	return &http.Server{
		Addr:    fmt.Sprintf(":%d", b.Config.Server.APIPort),
		Handler: router,
	}
}

func registerSPARoutes(router *gin.Engine, distFS fs.FS) {
	router.NoRoute(func(c *gin.Context) {
		requestPath := c.Request.URL.Path
		cleanedPath := strings.TrimPrefix(path.Clean("/"+requestPath), "/")

		if strings.HasPrefix(requestPath, "/api/") || requestPath == "/ws" || requestPath == "/metrics" {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}

		if filepath.Ext(cleanedPath) != "" {
			serveEmbeddedFile(c, distFS, cleanedPath)
			return
		}

		serveIndexHTML(c, distFS)
	})
}

func serveEmbeddedFile(c *gin.Context, distFS fs.FS, filePath string) {
	file, err := distFS.Open(filePath)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	defer file.Close()

	fileInfo, err := file.Stat()
	if err != nil || fileInfo.IsDir() {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}

	content, err := io.ReadAll(file)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to read frontend file"})
		return
	}

	if strings.HasPrefix(filePath, "assets/") {
		c.Header("Cache-Control", "public, max-age=31536000, immutable")
	} else {
		c.Header("Cache-Control", "public, max-age=3600")
	}
	contentType := http.DetectContentType(content)
	if ext := filepath.Ext(filePath); ext != "" {
		if byExt := mimeTypeByExt(ext); byExt != "" {
			contentType = byExt
		}
	}
	c.Data(http.StatusOK, contentType, content)
}

func serveIndexHTML(c *gin.Context, distFS fs.FS) {
	file, err := distFS.Open("index.html")
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "frontend not built"})
		return
	}
	defer file.Close()

	content, err := io.ReadAll(file)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to read frontend index"})
		return
	}

	c.Header("Cache-Control", "no-cache")
	c.Data(http.StatusOK, "text/html; charset=utf-8", content)
}

func mimeTypeByExt(ext string) string {
	switch strings.ToLower(ext) {
	case ".js", ".mjs":
		return "application/javascript; charset=utf-8"
	case ".css":
		return "text/css; charset=utf-8"
	case ".html":
		return "text/html; charset=utf-8"
	case ".json":
		return "application/json; charset=utf-8"
	case ".svg":
		return "image/svg+xml"
	default:
		return ""
	}
}

func (b *Builder) buildMetricsServer() *http.Server {
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.GET("/metrics", func(c *gin.Context) {
		b.Metrics.Handler().ServeHTTP(c.Writer, c.Request)
	})

	return &http.Server{
		Addr:    fmt.Sprintf(":%d", b.Config.Server.MetricsPort),
		Handler: router,
	}
}

type Composition struct {
	config              *config.Config
	store               *store.Store
	metrics             *metrics.Metrics
	sseBroker           *sse.Broker
	sseHandler          *sse.Handler
	wsHub               *ws.Hub
	wsHandler           *ws.Handler
	broadcaster         *broadcaster.Broadcaster
	syncManager         *syncmanager.SyncManager
	apiServer           *http.Server
	metricsServer       *http.Server
	shutdownCoordinator *ShutdownCoordinator
	cancelMain          context.CancelFunc
}

func (c *Composition) Run(ctx context.Context) error {
	log.Printf("[app] starting application on API port %d, metrics port %d",
		c.config.Server.APIPort, c.config.Server.MetricsPort)

	go func() {
		log.Println("[app] starting WebSocket hub")
		c.wsHub.Run()
		log.Println("[app] WebSocket hub stopped")
	}()

	go func() {
		log.Println("[app] starting SSE broker")
		c.sseBroker.Run()
		log.Println("[app] SSE broker stopped")
	}()

	if c.broadcaster != nil {
		go func() {
			log.Println("[app] starting realtime broadcaster")
			c.broadcaster.Start(ctx)
			log.Println("[app] realtime broadcaster stopped")
		}()
	}

	if c.syncManager != nil {
		go func() {
			log.Println("[app] starting sync manager")
			c.syncManager.Start(ctx)
			log.Println("[app] sync manager stopped")
		}()
	}

	serverErr := make(chan error, 1)
	go func() {
		log.Printf("[app] starting API server on %s", c.apiServer.Addr)
		if err := c.apiServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serverErr <- fmt.Errorf("API server error: %w", err)
		}
	}()

	go func() {
		log.Printf("[app] starting metrics server on %s", c.metricsServer.Addr)
		if err := c.metricsServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serverErr <- fmt.Errorf("metrics server error: %w", err)
		}
	}()

	select {
	case <-ctx.Done():
		log.Println("[app] context cancelled, initiating shutdown")
	case err := <-serverErr:
		log.Printf("[app] server error: %v", err)
	}

	return c.Stop()
}

func (c *Composition) Stop() error {
	if c.cancelMain != nil {
		log.Println("[app] cancelling main context")
		c.cancelMain()
	}

	ctx, cancel := context.WithTimeout(context.Background(), c.shutdownCoordinator.timeout)
	defer cancel()

	return c.shutdownCoordinator.Shutdown(ctx)
}

func (c *Composition) APIAddr() string {
	return c.apiServer.Addr
}

func (c *Composition) MetricsAddr() string {
	return c.metricsServer.Addr
}

func (c *Composition) Store() *store.Store {
	return c.store
}

func (c *Composition) Metrics() *metrics.Metrics {
	return c.metrics
}

func (c *Composition) WSHub() *ws.Hub {
	return c.wsHub
}

func (c *Composition) SSEBroker() *sse.Broker {
	return c.sseBroker
}

func (c *Composition) SyncManager() *syncmanager.SyncManager {
	return c.syncManager
}

func (c *Composition) APIServer() *http.Server {
	return c.apiServer
}

func (c *Composition) MetricsServer() *http.Server {
	return c.metricsServer
}
