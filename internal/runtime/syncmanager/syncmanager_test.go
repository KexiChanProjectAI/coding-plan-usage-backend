package syncmanager

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/quotahub/ucpqa/internal/config"
	"github.com/quotahub/ucpqa/internal/domain"
	providertype "github.com/quotahub/ucpqa/internal/domain/provider"
	"github.com/quotahub/ucpqa/internal/infrastructure/store"
)

// mockProvider is a test double for provider.Provider.
type mockProvider struct {
	name       string
	fetchFunc  func(ctx context.Context) (domain.AccountSnapshot, error)
	fetchCount int
	mu         sync.Mutex
}

func (m *mockProvider) Fetch(ctx context.Context) (domain.AccountSnapshot, error) {
	m.mu.Lock()
	m.fetchCount++
	m.mu.Unlock()
	return m.fetchFunc(ctx)
}

func (m *mockProvider) ProviderName() string {
	return m.name
}

func TestNew(t *testing.T) {
	providers := []providertype.Provider{
		&mockProvider{name: "test1", fetchFunc: func(ctx context.Context) (domain.AccountSnapshot, error) {
			return domain.AccountSnapshot{Platform: "test1"}, nil
		}},
		&mockProvider{name: "test2", fetchFunc: func(ctx context.Context) (domain.AccountSnapshot, error) {
			return domain.AccountSnapshot{Platform: "test2"}, nil
		}},
	}

	cfg := &config.Config{
		Providers: map[string]config.ProviderConfig{
			"test1": {Name: "test1", RefreshInterval: 10 * time.Second, BackoffInitial: 1 * time.Second, BackoffMax: 30 * time.Second},
			"test2": {Name: "test2", RefreshInterval: 20 * time.Second, BackoffInitial: 2 * time.Second, BackoffMax: 60 * time.Second},
		},
	}

	store := store.New()
	sm := New(providers, store, cfg)

	if sm == nil {
		t.Fatal("New returned nil")
	}
	if len(sm.providerStates) != 2 {
		t.Errorf("expected 2 provider states, got %d", len(sm.providerStates))
	}
}

func TestSyncManagerStartStop(t *testing.T) {
	fetchDone := make(chan struct{})
	provider := &mockProvider{
		name: "test",
		fetchFunc: func(ctx context.Context) (domain.AccountSnapshot, error) {
			select {
			case fetchDone <- struct{}{}:
			default:
			}
			return domain.AccountSnapshot{Platform: "test"}, nil
		},
	}

	cfg := &config.Config{
		Providers: map[string]config.ProviderConfig{
			"test": {Name: "test", RefreshInterval: 50 * time.Millisecond, BackoffInitial: 10 * time.Millisecond, BackoffMax: 100 * time.Millisecond},
		},
	}

	store := store.New()
	sm := New([]providertype.Provider{provider}, store, cfg)

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	go func() {
		<-fetchDone
		sm.Stop()
	}()

	sm.Start(ctx)
}

func TestSyncManagerRefresh(t *testing.T) {
	fetchCount := 0
	var fetchMu sync.Mutex

	provider := &mockProvider{
		name: "test",
		fetchFunc: func(ctx context.Context) (domain.AccountSnapshot, error) {
			fetchMu.Lock()
			fetchCount++
			fetchMu.Unlock()
			return domain.AccountSnapshot{Platform: "test"}, nil
		},
	}

	cfg := &config.Config{
		Providers: map[string]config.ProviderConfig{
			"test": {Name: "test", RefreshInterval: 1 * time.Hour, BackoffInitial: 1 * time.Second, BackoffMax: 30 * time.Second},
		},
	}

	store := store.New()
	sm := New([]providertype.Provider{provider}, store, cfg)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	go func() {
		time.Sleep(20 * time.Millisecond)
		sm.Refresh("test")
		sm.Refresh("test")
		time.Sleep(20 * time.Millisecond)
		sm.Stop()
	}()

	sm.Start(ctx)

	fetchMu.Lock()
	count := fetchCount
	fetchMu.Unlock()

	if count < 1 {
		t.Errorf("expected at least 1 fetch, got %d", count)
	}
}

func TestSyncManagerRefreshUnknownProvider(t *testing.T) {
	provider := &mockProvider{
		name: "test",
		fetchFunc: func(ctx context.Context) (domain.AccountSnapshot, error) {
			return domain.AccountSnapshot{Platform: "test"}, nil
		},
	}

	cfg := &config.Config{
		Providers: map[string]config.ProviderConfig{
			"test": {Name: "test", RefreshInterval: 1 * time.Hour, BackoffInitial: 1 * time.Second, BackoffMax: 30 * time.Second},
		},
	}

	store := store.New()
	sm := New([]providertype.Provider{provider}, store, cfg)

	sm.Refresh("unknown")
}

func TestSyncManagerBackoffResetOnSuccess(t *testing.T) {
	fetchCount := 0
	var fetchMu sync.Mutex

	provider := &mockProvider{
		name: "test",
		fetchFunc: func(ctx context.Context) (domain.AccountSnapshot, error) {
			fetchMu.Lock()
			fetchCount++
			fetchMu.Unlock()
			return domain.AccountSnapshot{Platform: "test", Version: int64(fetchCount)}, nil
		},
	}

	cfg := &config.Config{
		Providers: map[string]config.ProviderConfig{
			"test": {Name: "test", RefreshInterval: 50 * time.Millisecond, BackoffInitial: 10 * time.Millisecond, BackoffMax: 100 * time.Millisecond},
		},
	}

	store := store.New()
	sm := New([]providertype.Provider{provider}, store, cfg)

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()

	go func() {
		time.Sleep(200 * time.Millisecond)
		sm.Stop()
	}()

	sm.Start(ctx)

	snap, ok := store.Get("test")
	if !ok {
		t.Fatal("expected snapshot in store")
	}
	if snap.Version == 0 {
		t.Error("expected version > 0")
	}
}

func TestSyncManagerBackoffIncreaseOnFailure(t *testing.T) {
	failCount := 0
	var failMu sync.Mutex

	provider := &mockProvider{
		name: "test",
		fetchFunc: func(ctx context.Context) (domain.AccountSnapshot, error) {
			failMu.Lock()
			failCount++
			count := failCount
			failMu.Unlock()
			if count < 3 {
				return domain.AccountSnapshot{}, providertype.NewErrFetchFailure("test", context.DeadlineExceeded)
			}
			return domain.AccountSnapshot{Platform: "test", Version: 1}, nil
		},
	}

	cfg := &config.Config{
		Providers: map[string]config.ProviderConfig{
			"test": {Name: "test", RefreshInterval: 20 * time.Millisecond, BackoffInitial: 10 * time.Millisecond, BackoffMax: 50 * time.Millisecond},
		},
	}

	store := store.New()
	sm := New([]providertype.Provider{provider}, store, cfg)

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	go func() {
		time.Sleep(400 * time.Millisecond)
		sm.Stop()
	}()

	sm.Start(ctx)

	failMu.Lock()
	count := failCount
	failMu.Unlock()

	if count < 3 {
		t.Errorf("expected at least 3 fetch attempts, got %d", count)
	}
}

func TestApplyJitter(t *testing.T) {
	baseInterval := 100 * time.Millisecond

	for _, pct := range []int{0, 10, 50} {
		for i := 0; i < 100; i++ {
			result := applyJitter(baseInterval, pct)
			if pct == 0 {
				if result != baseInterval {
					t.Errorf("with 0%% jitter, expected exact interval, got %v", result)
				}
			} else {
				minExpected := time.Duration(float64(baseInterval) * (1 - float64(pct)/100))
				maxExpected := time.Duration(float64(baseInterval) * (1 + float64(pct)/100))
				if result < minExpected || result > maxExpected {
					t.Errorf("jitter %d%%: result %v outside expected range [%v, %v]", pct, result, minExpected, maxExpected)
				}
			}
		}
	}
}

func TestMultipleProviders(t *testing.T) {
	providers := []providertype.Provider{
		&mockProvider{name: "prov1", fetchFunc: func(ctx context.Context) (domain.AccountSnapshot, error) {
			return domain.AccountSnapshot{Platform: "prov1", Version: 1}, nil
		}},
		&mockProvider{name: "prov2", fetchFunc: func(ctx context.Context) (domain.AccountSnapshot, error) {
			return domain.AccountSnapshot{Platform: "prov2", Version: 1}, nil
		}},
		&mockProvider{name: "prov3", fetchFunc: func(ctx context.Context) (domain.AccountSnapshot, error) {
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

	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()

	go func() {
		time.Sleep(100 * time.Millisecond)
		sm.Stop()
	}()

	sm.Start(ctx)

	for _, name := range []string{"prov1", "prov2", "prov3"} {
		if _, ok := store.Get(name); !ok {
			t.Errorf("expected snapshot for %s in store", name)
		}
	}
}

func TestContextCancellation(t *testing.T) {
	provider := &mockProvider{
		name: "test",
		fetchFunc: func(ctx context.Context) (domain.AccountSnapshot, error) {
			select {
			case <-ctx.Done():
				return domain.AccountSnapshot{}, ctx.Err()
			case <-time.After(1 * time.Hour):
				return domain.AccountSnapshot{Platform: "test"}, nil
			}
		},
	}

	cfg := &config.Config{
		Providers: map[string]config.ProviderConfig{
			"test": {Name: "test", RefreshInterval: 1 * time.Hour, BackoffInitial: 1 * time.Second, BackoffMax: 30 * time.Second},
		},
	}

	store := store.New()
	sm := New([]providertype.Provider{provider}, store, cfg)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	done := make(chan struct{})
	go func() {
		sm.Start(ctx)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(200 * time.Millisecond):
		t.Error("Start did not return after context cancellation")
	}
}