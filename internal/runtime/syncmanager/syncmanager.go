// Package syncmanager provides the polling runtime that manages provider sync goroutines.
package syncmanager

import (
	"context"
	"log"
	"math/rand"
	"sync"
	"time"

	"github.com/quotahub/ucpqa/internal/config"
	"github.com/quotahub/ucpqa/internal/domain/provider"
	"github.com/quotahub/ucpqa/internal/infrastructure/store"
)

// SyncManager manages polling goroutines for each provider, applying jitter,
// exponential backoff on failure, and coalescing manual refresh requests.
type SyncManager struct {
	providers []provider.Provider
	store     *store.Store
	config    *config.Config

	// Per-provider state
	mu             sync.RWMutex
	providerStates map[string]*providerState
	startMu        sync.Mutex
	wg             sync.WaitGroup
	stopCh         chan struct{}
	stopOnce       sync.Once
}

type providerState struct {
	name      string
	backoff   time.Duration
	refreshCh chan struct{} // buffered channel size 1 for coalescing
}

// New creates a new SyncManager with the given providers, store, and config.
func New(providers []provider.Provider, store *store.Store, cfg *config.Config) *SyncManager {
	sm := &SyncManager{
		providers:      providers,
		store:          store,
		config:         cfg,
		providerStates: make(map[string]*providerState),
		stopCh:         make(chan struct{}),
	}

	for _, p := range providers {
		name := p.ProviderName()
		provCfg, ok := cfg.Providers[name]
		if !ok {
			// Fall back to zero values if provider not in config
			provCfg = config.ProviderConfig{Name: name}
		}

		sm.providerStates[name] = &providerState{
			name:      name,
			backoff:   provCfg.BackoffInitial,
			refreshCh: make(chan struct{}, 1),
		}
	}

	return sm
}

// Start launches one polling goroutine per provider and blocks until ctx is cancelled.
func (sm *SyncManager) Start(ctx context.Context) {
	sm.startMu.Lock()
	defer sm.startMu.Unlock()

	for _, p := range sm.providers {
		sm.wg.Add(1)
		go sm.poll(ctx, p)
	}

	sm.wg.Wait()
}

// Stop cancels all polling goroutines and waits for them to finish.
func (sm *SyncManager) Stop() {
	sm.stopOnce.Do(func() {
		close(sm.stopCh)
	})
	sm.wg.Wait()
}

// Refresh enqueues a manual refresh request for the given provider.
// If the provider's refresh channel is already full (coalescing), the request is dropped.
func (sm *SyncManager) Refresh(providerName string) {
	sm.mu.RLock()
	state, ok := sm.providerStates[providerName]
	sm.mu.RUnlock()

	if !ok {
		log.Printf("[syncmanager] Refresh: unknown provider %q", providerName)
		return
	}

	select {
	case state.refreshCh <- struct{}{}:
	default:
		log.Printf("[syncmanager] Refresh: coalescing dropped refresh for provider %q", providerName)
	}
}

// poll runs the polling loop for a single provider.
func (sm *SyncManager) poll(ctx context.Context, p provider.Provider) {
	defer sm.wg.Done()

	name := p.ProviderName()

	sm.mu.RLock()
	state := sm.providerStates[name]
	provCfg := sm.config.Providers[name]
	sm.mu.RUnlock()

	backoff := provCfg.BackoffInitial
	if backoff <= 0 {
		backoff = 1 * time.Second
	}

	for {
		interval := provCfg.RefreshInterval
		if interval <= 0 {
			interval = 1 * time.Minute
		}
		jitter := applyJitter(interval, provCfg.JitterPercent)

		select {
		case <-ctx.Done():
			log.Printf("[syncmanager] poll: context cancelled for provider %q", name)
			return
		case <-sm.stopCh:
			log.Printf("[syncmanager] poll: stop requested for provider %q", name)
			return
		case <-state.refreshCh:
			log.Printf("[syncmanager] poll: manual refresh triggered for provider %q", name)
		case <-time.After(jitter):
		}

		snapshot, err := p.Fetch(ctx)
		if err != nil {
			log.Printf("[syncmanager] poll: fetch error for provider %q: %v", name, err)
			backoff = min(backoff*2, provCfg.BackoffMax)
			if provCfg.BackoffMax > 0 && backoff <= 0 {
				backoff = provCfg.BackoffMax
			}
			log.Printf("[syncmanager] poll: backoff increased to %v for provider %q", backoff, name)

			select {
			case <-ctx.Done():
				return
			case <-sm.stopCh:
				return
			case <-time.After(backoff):
			}
			continue
		}

		sm.store.Update(snapshot)
		backoff = provCfg.BackoffInitial
		if backoff <= 0 {
			backoff = 1 * time.Second
		}
		log.Printf("[syncmanager] poll: success for provider %q, backoff reset to %v", name, backoff)
	}
}

// applyJitter applies random jitter to a duration based on the given percentage.
// The jitter can be positive or negative, up to the configured percent.
func applyJitter(d time.Duration, jitterPercent int) time.Duration {
	if jitterPercent <= 0 {
		return d
	}
	fraction := float64(jitterPercent) / 100.0
	maxJitter := float64(d) * fraction
	jitter := (rand.Float64()*2 - 1) * maxJitter
	return time.Duration(float64(d) + jitter)
}
