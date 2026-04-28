package syncmanager

import (
	"context"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/quotahub/ucpqa/internal/config"
	"github.com/quotahub/ucpqa/internal/domain"
	providertype "github.com/quotahub/ucpqa/internal/domain/provider"
	"github.com/quotahub/ucpqa/internal/infrastructure/store"
)

// mockProvider is a test double for provider.Provider.
type raceMockProvider struct {
	name       string
	fetchFunc  func(ctx context.Context) (domain.AccountSnapshot, error)
	fetchCount int
	mu         sync.Mutex
}

func (m *raceMockProvider) Fetch(ctx context.Context) (domain.AccountSnapshot, error) {
	m.mu.Lock()
	m.fetchCount++
	m.mu.Unlock()
	return m.fetchFunc(ctx)
}

func (m *raceMockProvider) ProviderName() string {
	return m.name
}

// TestSyncManagerRaceStartStopRefresh tests concurrent Start/Stop/Refresh calls.
func TestSyncManagerRaceStartStopRefresh(t *testing.T) {
	provider := &raceMockProvider{
		name: "test",
		fetchFunc: func(ctx context.Context) (domain.AccountSnapshot, error) {
			return domain.AccountSnapshot{Platform: "test", Version: 1}, nil
		},
	}

	cfg := &config.Config{
		Providers: map[string]config.ProviderConfig{
			"test": {Name: "test", RefreshInterval: 10 * time.Millisecond, BackoffInitial: 5 * time.Millisecond, BackoffMax: 50 * time.Millisecond},
		},
	}

	store := store.New()
	sm := New([]providertype.Provider{provider}, store, cfg)

	var wg sync.WaitGroup
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	wg.Add(1)
	go func() {
		defer wg.Done()
		sm.Start(ctx)
	}()

	// Concurrent refresh calls
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 20; j++ {
				sm.Refresh("test")
				time.Sleep(5 * time.Millisecond)
			}
		}()
	}

	// Single stop after a delay (avoid multiple stops causing panic)
	time.Sleep(100 * time.Millisecond)
	sm.Stop()

	wg.Wait()
}

// TestSyncManagerRaceMultipleProviders tests multiple providers fetching concurrently.
func TestSyncManagerRaceMultipleProviders(t *testing.T) {
	providers := []providertype.Provider{
		&raceMockProvider{name: "prov1", fetchFunc: func(ctx context.Context) (domain.AccountSnapshot, error) {
			return domain.AccountSnapshot{Platform: "prov1", Version: 1}, nil
		}},
		&raceMockProvider{name: "prov2", fetchFunc: func(ctx context.Context) (domain.AccountSnapshot, error) {
			return domain.AccountSnapshot{Platform: "prov2", Version: 1}, nil
		}},
		&raceMockProvider{name: "prov3", fetchFunc: func(ctx context.Context) (domain.AccountSnapshot, error) {
			return domain.AccountSnapshot{Platform: "prov3", Version: 1}, nil
		}},
		&raceMockProvider{name: "prov4", fetchFunc: func(ctx context.Context) (domain.AccountSnapshot, error) {
			return domain.AccountSnapshot{Platform: "prov4", Version: 1}, nil
		}},
		&raceMockProvider{name: "prov5", fetchFunc: func(ctx context.Context) (domain.AccountSnapshot, error) {
			return domain.AccountSnapshot{Platform: "prov5", Version: 1}, nil
		}},
	}

	cfg := &config.Config{
		Providers: map[string]config.ProviderConfig{
			"prov1": {Name: "prov1", RefreshInterval: 20 * time.Millisecond, BackoffInitial: 5 * time.Millisecond, BackoffMax: 50 * time.Millisecond},
			"prov2": {Name: "prov2", RefreshInterval: 20 * time.Millisecond, BackoffInitial: 5 * time.Millisecond, BackoffMax: 50 * time.Millisecond},
			"prov3": {Name: "prov3", RefreshInterval: 20 * time.Millisecond, BackoffInitial: 5 * time.Millisecond, BackoffMax: 50 * time.Millisecond},
			"prov4": {Name: "prov4", RefreshInterval: 20 * time.Millisecond, BackoffInitial: 5 * time.Millisecond, BackoffMax: 50 * time.Millisecond},
			"prov5": {Name: "prov5", RefreshInterval: 20 * time.Millisecond, BackoffInitial: 5 * time.Millisecond, BackoffMax: 50 * time.Millisecond},
		},
	}

	store := store.New()
	sm := New(providers, store, cfg)

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		sm.Start(ctx)
	}()

	// Concurrent refresh calls to all providers
	for _, name := range []string{"prov1", "prov2", "prov3", "prov4", "prov5"} {
		pname := name
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 10; i++ {
				sm.Refresh(pname)
				time.Sleep(10 * time.Millisecond)
			}
		}()
	}

	wg.Wait()

	// Verify all providers have data
	for _, name := range []string{"prov1", "prov2", "prov3", "prov4", "prov5"} {
		snap, ok := store.Get(name)
		if !ok {
			t.Errorf("expected snapshot for %s", name)
			continue
		}
		if snap.Version == 0 {
			t.Errorf("expected version > 0 for %s", name)
		}
	}
}

// TestSyncManagerRaceRefreshDuringStop tests refresh calls during shutdown.
func TestSyncManagerRaceRefreshDuringStop(t *testing.T) {
	fetchCount := 0
	var countMu sync.Mutex

	provider := &raceMockProvider{
		name: "test",
		fetchFunc: func(ctx context.Context) (domain.AccountSnapshot, error) {
			countMu.Lock()
			fetchCount++
			countMu.Unlock()
			return domain.AccountSnapshot{Platform: "test", Version: 1}, nil
		},
	}

	cfg := &config.Config{
		Providers: map[string]config.ProviderConfig{
			"test": {Name: "test", RefreshInterval: 50 * time.Millisecond, BackoffInitial: 10 * time.Millisecond, BackoffMax: 100 * time.Millisecond},
		},
	}

	store := store.New()
	sm := New([]providertype.Provider{provider}, store, cfg)

	var wg sync.WaitGroup

	// Starter
	wg.Add(1)
	go func() {
		defer wg.Done()
		ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
		defer cancel()
		sm.Start(ctx)
	}()

	// Refreshes during run
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 20; i++ {
			sm.Refresh("test")
			time.Sleep(10 * time.Millisecond)
		}
	}()

	// Stop during operation
	wg.Add(1)
	go func() {
		defer wg.Done()
		time.Sleep(50 * time.Millisecond)
		sm.Stop()
	}()

	wg.Wait()

	t.Logf("Total fetches: %d", fetchCount)
}

// TestSyncManagerRaceConcurrentStart tests concurrent Start calls (should not panic).
func TestSyncManagerRaceConcurrentStart(t *testing.T) {
	provider := &raceMockProvider{
		name: "test",
		fetchFunc: func(ctx context.Context) (domain.AccountSnapshot, error) {
			return domain.AccountSnapshot{Platform: "test", Version: 1}, nil
		},
	}

	cfg := &config.Config{
		Providers: map[string]config.ProviderConfig{
			"test": {Name: "test", RefreshInterval: 100 * time.Millisecond, BackoffInitial: 50 * time.Millisecond, BackoffMax: 200 * time.Millisecond},
		},
	}

	store := store.New()
	sm := New([]providertype.Provider{provider}, store, cfg)

	var wg sync.WaitGroup

	// Multiple concurrent starts
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			defer cancel()
			sm.Start(ctx)
		}()
	}

	wg.Wait()
}

// TestSyncManagerLeakGoroutines verifies no goroutine leaks after shutdown.
func TestSyncManagerLeakGoroutines(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping leak test in short mode")
	}

	before := runtime.NumGoroutine()

	providers := []providertype.Provider{
		&raceMockProvider{name: "prov1", fetchFunc: func(ctx context.Context) (domain.AccountSnapshot, error) {
			return domain.AccountSnapshot{Platform: "prov1", Version: 1}, nil
		}},
		&raceMockProvider{name: "prov2", fetchFunc: func(ctx context.Context) (domain.AccountSnapshot, error) {
			return domain.AccountSnapshot{Platform: "prov2", Version: 1}, nil
		}},
		&raceMockProvider{name: "prov3", fetchFunc: func(ctx context.Context) (domain.AccountSnapshot, error) {
			return domain.AccountSnapshot{Platform: "prov3", Version: 1}, nil
		}},
	}

	cfg := &config.Config{
		Providers: map[string]config.ProviderConfig{
			"prov1": {Name: "prov1", RefreshInterval: 50 * time.Millisecond, BackoffInitial: 10 * time.Millisecond, BackoffMax: 100 * time.Millisecond},
			"prov2": {Name: "prov2", RefreshInterval: 50 * time.Millisecond, BackoffInitial: 10 * time.Millisecond, BackoffMax: 100 * time.Millisecond},
			"prov3": {Name: "prov3", RefreshInterval: 50 * time.Millisecond, BackoffInitial: 10 * time.Millisecond, BackoffMax: 100 * time.Millisecond},
		},
	}

	store := store.New()
	sm := New(providers, store, cfg)

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	go sm.Start(ctx)

	// Some refreshes
	time.Sleep(50 * time.Millisecond)
	sm.Refresh("prov1")
	sm.Refresh("prov2")
	sm.Refresh("prov3")

	sm.Stop()

	// Allow cleanup time
	time.Sleep(100 * time.Millisecond)

	after := runtime.NumGoroutine()
	leaked := after - before

	if leaked > 5 {
		t.Errorf("goroutine leak detected: before=%d, after=%d, leaked=%d", before, after, leaked)
	}
}
