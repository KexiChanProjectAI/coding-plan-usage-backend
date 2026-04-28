package app

import (
	"context"
	"net/http"
	"net/http/httptest"
	"runtime"
	"sync/atomic"
	"testing"
	"time"
)

type mockShutdownable struct {
	name      string
	shutdown  func(ctx context.Context) error
	callCount atomic.Int32
}

func (m *mockShutdownable) Shutdown(ctx context.Context) error {
	m.callCount.Add(1)
	return m.shutdown(ctx)
}

func (m *mockShutdownable) CallCount() int {
	return int(m.callCount.Load())
}

func TestShutdownCoordinator_ShutdownCompletesWithinTimeout(t *testing.T) {
	sc := NewShutdownCoordinator(500 * time.Millisecond)

	var callOrder []string
	m1 := &mockShutdownable{
		name: "fast",
		shutdown: func(ctx context.Context) error {
			callOrder = append(callOrder, "m1-start")
			time.Sleep(50 * time.Millisecond)
			callOrder = append(callOrder, "m1-end")
			return nil
		},
	}
	m2 := &mockShutdownable{
		name: "slow",
		shutdown: func(ctx context.Context) error {
			callOrder = append(callOrder, "m2-start")
			time.Sleep(100 * time.Millisecond)
			callOrder = append(callOrder, "m2-end")
			return nil
		},
	}

	sc.Register(m1)
	sc.Register(m2)

	start := time.Now()
	err := sc.Shutdown(context.Background())
	elapsed := time.Since(start)

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if elapsed > 500*time.Millisecond {
		t.Errorf("shutdown took too long: %v > 500ms", elapsed)
	}
}

func TestShutdownCoordinator_ComponentsShutDownInReverseOrder(t *testing.T) {
	sc := NewShutdownCoordinator(5 * time.Second)

	var callOrder []string
	m1 := &mockShutdownable{
		name: "first",
		shutdown: func(ctx context.Context) error {
			callOrder = append(callOrder, "first")
			return nil
		},
	}
	m2 := &mockShutdownable{
		name: "second",
		shutdown: func(ctx context.Context) error {
			callOrder = append(callOrder, "second")
			return nil
		},
	}
	m3 := &mockShutdownable{
		name: "third",
		shutdown: func(ctx context.Context) error {
			callOrder = append(callOrder, "third")
			return nil
		},
	}

	sc.Register(m1)
	sc.Register(m2)
	sc.Register(m3)

	err := sc.Shutdown(context.Background())
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if len(callOrder) != 3 {
		t.Fatalf("expected 3 calls, got %d", len(callOrder))
	}
	if callOrder[0] != "third" {
		t.Errorf("expected first to be 'third', got %q", callOrder[0])
	}
	if callOrder[1] != "second" {
		t.Errorf("expected second to be 'second', got %q", callOrder[1])
	}
	if callOrder[2] != "first" {
		t.Errorf("expected third to be 'first', got %q", callOrder[2])
	}
}

func TestShutdownCoordinator_AllComponentsReceiveShutdownSignal(t *testing.T) {
	sc := NewShutdownCoordinator(5 * time.Second)

	m1 := &mockShutdownable{name: "m1", shutdown: func(ctx context.Context) error { return nil }}
	m2 := &mockShutdownable{name: "m2", shutdown: func(ctx context.Context) error { return nil }}
	m3 := &mockShutdownable{name: "m3", shutdown: func(ctx context.Context) error { return nil }}

	sc.Register(m1)
	sc.Register(m2)
	sc.Register(m3)

	err := sc.Shutdown(context.Background())
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if m1.CallCount() != 1 {
		t.Errorf("m1 call count: expected 1, got %d", m1.CallCount())
	}
	if m2.CallCount() != 1 {
		t.Errorf("m2 call count: expected 1, got %d", m2.CallCount())
	}
	if m3.CallCount() != 1 {
		t.Errorf("m3 call count: expected 1, got %d", m3.CallCount())
	}
}

func TestShutdownCoordinator_NoGoroutineLeaks(t *testing.T) {
	initialGoroutines := runtime.NumGoroutine()

	sc := NewShutdownCoordinator(5 * time.Second)

	for i := 0; i < 5; i++ {
		m := &mockShutdownable{
			name: "goroutine",
			shutdown: func(ctx context.Context) error {
				done := make(chan struct{})
				go func() {
					defer close(done)
					select {
					case <-ctx.Done():
					case <-time.After(100 * time.Millisecond):
					}
				}()
				<-done
				return nil
			},
		}
		sc.Register(m)
	}

	err := sc.Shutdown(context.Background())
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	time.Sleep(200 * time.Millisecond)

	finalGoroutines := runtime.NumGoroutine()
	if finalGoroutines > initialGoroutines {
		t.Errorf("goroutine leak detected: started with %d, ended with %d",
			initialGoroutines, finalGoroutines)
	}
}

func TestShutdownCoordinator_ContextCancellation(t *testing.T) {
	sc := NewShutdownCoordinator(50 * time.Millisecond)

	m := &mockShutdownable{
		name: "blocking",
		shutdown: func(ctx context.Context) error {
			<-ctx.Done()
			return ctx.Err()
		},
	}
	sc.Register(m)

	err := sc.Shutdown(context.Background())
	if err == nil {
		t.Error("expected error due to context cancellation, got nil")
	}
}

func TestShutdownCoordinator_ComponentErrorDoesNotStopOthers(t *testing.T) {
	sc := NewShutdownCoordinator(5 * time.Second)

	var callCount atomic.Int32
	m1 := &mockShutdownable{
		name: "failing",
		shutdown: func(ctx context.Context) error {
			callCount.Add(1)
			return context.DeadlineExceeded
		},
	}
	m2 := &mockShutdownable{
		name: "success",
		shutdown: func(ctx context.Context) error {
			callCount.Add(1)
			return nil
		},
	}

	sc.Register(m1)
	sc.Register(m2)

	err := sc.Shutdown(context.Background())
	if err == nil {
		t.Error("expected error due to failing component")
	}

	if callCount.Load() != 2 {
		t.Errorf("expected both components to be called, got count %d", callCount.Load())
	}
}

func TestApp_Stop(t *testing.T) {
	cancelCalled := atomic.Bool{}
	mainCtx, cancel := context.WithCancel(context.Background())

	app := NewApp(
		5*time.Second,
		cancel,
		nil,
		nil,
		nil,
		nil,
		nil,
	)

	go func() {
		<-mainCtx.Done()
		cancelCalled.Store(true)
	}()

	err := app.Stop()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	time.Sleep(50 * time.Millisecond)

	if !cancelCalled.Load() {
		t.Error("main context was not cancelled")
	}
}

func TestApp_StopWithAllComponents(t *testing.T) {
	cancelCalled := atomic.Bool{}

	var apiServerCalled, metricsServerCalled, sseCalled, wsCalled, syncMgrCalled atomic.Bool

	mainCtx, cancel := context.WithCancel(context.Background())

	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		apiServerCalled.Store(true)
	}))
	metricsServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		metricsServerCalled.Store(true)
	}))

	app := NewApp(
		5*time.Second,
		cancel,
		apiServer.Config,
		metricsServer.Config,
		&mockBroker{stop: func() { sseCalled.Store(true) }},
		&mockBroker{stop: func() { wsCalled.Store(true) }},
		&mockBroker{stop: func() { syncMgrCalled.Store(true) }},
	)

	go func() {
		<-mainCtx.Done()
		cancelCalled.Store(true)
	}()

	err := app.Stop()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	time.Sleep(50 * time.Millisecond)

	if !cancelCalled.Load() {
		t.Error("main context was not cancelled")
	}

	if apiServer.Config == nil {
		t.Error("API server config is nil")
	}
	if metricsServer.Config == nil {
		t.Error("metrics server config is nil")
	}

	if !sseCalled.Load() {
		t.Error("SSE broker was not stopped")
	}
	if !wsCalled.Load() {
		t.Error("WS hub was not stopped")
	}
	if !syncMgrCalled.Load() {
		t.Error("sync manager was not stopped")
	}
}

type mockBroker struct {
	stop func()
}

func (m *mockBroker) Stop() {
	m.stop()
}