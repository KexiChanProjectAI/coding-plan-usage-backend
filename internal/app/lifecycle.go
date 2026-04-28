// Package app provides application composition and lifecycle management.
package app

import (
	"context"
	"errors"
	"log"
	"net/http"
	"sync"
	"time"
)

// Shutdownable is the interface for components that support graceful shutdown.
// All components that hold resources (goroutines, connections, listeners) must implement this.
type Shutdownable interface {
	// Shutdown gracefully shuts down the component within the given context.
	// Components should:
	//   - Stop accepting new work
	//   - Drain existing work within bounded time
	//   - Release all resources
	//   - Return nil on success, or error if graceful shutdown failed
	//   - Context cancellation should cause Shutdown to return immediately
	Shutdown(ctx context.Context) error
}

// ShutdownCoordinator manages graceful shutdown of all application components.
// Components are shut down in reverse order of registration (LIFO).
type ShutdownCoordinator struct {
	timeout    time.Duration
	components []Shutdownable
	mu         sync.Mutex
}

// NewShutdownCoordinator creates a new shutdown coordinator with the given timeout.
func NewShutdownCoordinator(timeout time.Duration) *ShutdownCoordinator {
	return &ShutdownCoordinator{
		timeout:    timeout,
		components: make([]Shutdownable, 0),
	}
}

// Register adds a component to the shutdown list.
// Components are shut down in reverse order (LIFO).
func (sc *ShutdownCoordinator) Register(component Shutdownable) {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	sc.components = append(sc.components, component)
}

// Shutdown shuts down all registered components in reverse order.
// Each component gets a portion of the overall timeout.
// If a component exceeds its time budget, an error is logged but shutdown continues.
func (sc *ShutdownCoordinator) Shutdown(ctx context.Context) error {
	sc.mu.Lock()
	components := make([]Shutdownable, len(sc.components))
	copy(components, sc.components)
	// Reverse the slice for LIFO order
	for i, j := 0, len(components)-1; i < j; i, j = i+1, j-1 {
		components[i], components[j] = components[j], components[i]
	}
	sc.mu.Unlock()

	if len(components) == 0 {
		return nil
	}

	// Allocate time per component, but cap at the overall timeout
	perComponentTimeout := sc.timeout / time.Duration(len(components))
	if perComponentTimeout < time.Second {
		perComponentTimeout = time.Second // Minimum 1 second per component
	}

	var errs []error
	for _, component := range components {
		compCtx, cancel := context.WithTimeout(ctx, perComponentTimeout)
		start := time.Now()

		log.Printf("[lifecycle] shutting down component %T", component)

		if err := component.Shutdown(compCtx); err != nil {
			log.Printf("[lifecycle] shutdown error for %T: %v (elapsed: %v)", component, err, time.Since(start))
			errs = append(errs, err)
		} else {
			log.Printf("[lifecycle] component %T shut down successfully (elapsed: %v)", component, time.Since(start))
		}
		cancel()
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}

// HTTPShutdownable adapts an *http.Server to the Shutdownable interface.
type HTTPShutdownable struct {
	Server *http.Server
}

// Shutdown gracefully shuts down the HTTP server.
func (h *HTTPShutdownable) Shutdown(ctx context.Context) error {
	return h.Server.Shutdown(ctx)
}

// SyncManagerShutdownable adapts syncmanager to the Shutdownable interface.
type SyncManagerShutdownable struct {
	sm *SyncManagerWrapper
}

// SyncManagerWrapper wraps the syncmanager to provide a Stop method.
type SyncManagerWrapper struct {
	stop func()
}

// NewSyncManagerWrapper creates a wrapper for syncmanager's stop function.
func NewSyncManagerWrapper(stop func()) *SyncManagerWrapper {
	return &SyncManagerWrapper{stop: stop}
}

// Stop signals the sync manager to stop.
func (w *SyncManagerWrapper) Stop() {
	w.stop()
}

// SyncManagerShutdownable wraps a SyncManagerWrapper for graceful shutdown.
func (sm *SyncManagerShutdownable) Shutdown(ctx context.Context) error {
	done := make(chan struct{})
	go func() {
		sm.sm.Stop()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// WSHubShutdownable adapts the WebSocket hub to the Shutdownable interface.
type WSHubShutdownable struct {
	hub interface {
		Stop()
	}
}

// NewWSHubShutdownable creates a Shutdownable for a WebSocket hub.
func NewWSHubShutdownable(hub interface {
	Stop()
}) *WSHubShutdownable {
	return &WSHubShutdownable{hub: hub}
}

// Shutdown gracefully shuts down the WebSocket hub.
func (w *WSHubShutdownable) Shutdown(ctx context.Context) error {
	done := make(chan struct{})
	go func() {
		w.hub.Stop()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// SSEBrokerShutdownable adapts the SSE broker to the Shutdownable interface.
type SSEBrokerShutdownable struct {
	broker interface {
		Stop()
	}
}

// NewSSEBrokerShutdownable creates a Shutdownable for an SSE broker.
func NewSSEBrokerShutdownable(broker interface {
	Stop()
}) *SSEBrokerShutdownable {
	return &SSEBrokerShutdownable{broker: broker}
}

// Shutdown gracefully shuts down the SSE broker.
func (s *SSEBrokerShutdownable) Shutdown(ctx context.Context) error {
	done := make(chan struct{})
	go func() {
		s.broker.Stop()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// App manages the application lifecycle and coordinates graceful shutdown.
type App struct {
	shutdownCoordinator *ShutdownCoordinator
	cancelMain         context.CancelFunc
	apiServer          *http.Server
	metricsServer      *http.Server
	sseBroker          interface{ Stop() }
	wsHub              interface{ Stop() }
	syncManager        interface{ Stop() }
}

// NewApp creates a new application instance with the given components.
func NewApp(
	shutdownTimeout time.Duration,
	cancelMain context.CancelFunc,
	apiServer *http.Server,
	metricsServer *http.Server,
	sseBroker interface{ Stop() },
	wsHub interface{ Stop() },
	syncManager interface{ Stop() },
) *App {
	sc := NewShutdownCoordinator(shutdownTimeout)

	app := &App{
		shutdownCoordinator: sc,
		cancelMain:         cancelMain,
		apiServer:          apiServer,
		metricsServer:      metricsServer,
		sseBroker:          sseBroker,
		wsHub:              wsHub,
		syncManager:        syncManager,
	}

	if apiServer != nil {
		sc.Register(&HTTPShutdownable{Server: apiServer})
	}
	if metricsServer != nil {
		sc.Register(&HTTPShutdownable{Server: metricsServer})
	}
	if sseBroker != nil {
		sc.Register(NewSSEBrokerShutdownable(sseBroker))
	}
	if wsHub != nil {
		sc.Register(NewWSHubShutdownable(wsHub))
	}
	if syncManager != nil {
		sc.Register(&SyncManagerAdapter{stop: syncManager.Stop})
	}

	return app
}

// SyncManagerAdapter adapts a sync manager's Stop() to Shutdownable.
type SyncManagerAdapter struct {
	stop func()
}

func (s *SyncManagerAdapter) Shutdown(ctx context.Context) error {
	done := make(chan struct{})
	go func() {
		s.stop()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Stop gracefully shuts down all application components in order:
//  1. Cancel main context
//  2. Stop sync manager (wait for pollers)
//  3. Stop SSE broker (close subscribers)
//  4. Stop WS hub (disconnect clients)
//  5. Shutdown API server (with timeout)
//  6. Shutdown metrics server (with timeout)
func (a *App) Stop() error {
	if a.cancelMain != nil {
		log.Println("[lifecycle] cancelling main context")
		a.cancelMain()
	}

	ctx, cancel := context.WithTimeout(context.Background(), a.shutdownCoordinator.timeout)
	defer cancel()

	return a.shutdownCoordinator.Shutdown(ctx)
}