package app

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/quotahub/ucpqa/internal/config"
	"github.com/quotahub/ucpqa/internal/domain"
	"github.com/quotahub/ucpqa/internal/infrastructure/metrics"
	"github.com/quotahub/ucpqa/internal/infrastructure/store"
	"github.com/quotahub/ucpqa/internal/runtime/syncmanager"
	"github.com/quotahub/ucpqa/internal/transport/http/api"
	"github.com/quotahub/ucpqa/internal/transport/sse"
	"github.com/quotahub/ucpqa/internal/transport/ws"
	"github.com/quotahub/ucpqa/internal/web"
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

	previousOnUpdate := b.Store.OnUpdate
	b.Store.OnUpdate = func(snapshot domain.AccountSnapshot) {
		if previousOnUpdate != nil {
			previousOnUpdate(snapshot)
		}

		rawSnapshotBytes, err := json.Marshal(snapshot)
		if err != nil {
			log.Printf("[app] failed to marshal snapshot for platform %q: %v", snapshot.Platform, err)
			return
		}

		eventBytes, err := json.Marshal(sse.Event{
			Version:  snapshot.Version,
			Snapshot: json.RawMessage(rawSnapshotBytes),
		})
		if err != nil {
			log.Printf("[app] failed to marshal stream event for platform %q: %v", snapshot.Platform, err)
			return
		}

		sseBroker.Publish(eventBytes)
		wsHub.Broadcast(rawSnapshotBytes)
		b.Metrics.UpdateFromSnapshot(snapshot)
	}

	usageHandler := api.NewUsageHandler(b.Store, b.Config.Global.MaxStaleDuration)

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

	router.GET("/", func(c *gin.Context) {
		c.Header("Content-Type", "text/html; charset=utf-8")
		data, err := web.DashboardFS.ReadFile("dashboard.html")
		if err != nil {
			c.String(http.StatusInternalServerError, "failed to load dashboard")
			return
		}
		c.String(http.StatusOK, string(data))
	})

	v1 := router.Group("/api/v1")
	{
		v1.GET("/usage", usageHandler.GetUsage)
		v1.GET("/stream", sseHandler.StreamHandler())
	}
	router.GET("/ws", wsHandler.WSHandler())

	return &http.Server{
		Addr:    fmt.Sprintf(":%d", b.Config.Server.APIPort),
		Handler: router,
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
