package store

import (
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/quotahub/ucpqa/internal/domain"
)

// TestStoreRaceConcurrentReadWrite tests for data races between concurrent readers and writers.
func TestStoreRaceConcurrentReadWrite(t *testing.T) {
	s := New()

	// Pre-populate store
	snap := domain.AccountSnapshot{
		Platform:     "minimax",
		AccountAlias: "test",
		Quotas:       make(map[domain.Tier]domain.QuotaTier),
	}
	snap.AddQuota(domain.Tier5H, domain.QuotaTier{Used: 10, Total: 100, ResetAt: time.Now()})
	s.Update(snap)

	duration := 1 * time.Second
	stopCh := make(chan struct{})

	var wg sync.WaitGroup

	// 10 reader goroutines
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ticker := time.NewTicker(1 * time.Millisecond)
			defer ticker.Stop()
			for {
				select {
				case <-stopCh:
					return
				case <-ticker.C:
					_, ok := s.Get("minimax")
					_ = ok
					_, _ = s.Get("nonexistent")
				}
			}
		}()
	}

	// 10 writer goroutines
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			ticker := time.NewTicker(1 * time.Millisecond)
			defer ticker.Stop()
			i := 0
			for {
				select {
				case <-stopCh:
					return
				case <-ticker.C:
					snap := domain.AccountSnapshot{
						Platform:     "minimax",
						AccountAlias: "test",
						Quotas:       make(map[domain.Tier]domain.QuotaTier),
					}
					snap.AddQuota(domain.Tier5H, domain.QuotaTier{Used: int64(i), Total: 100, ResetAt: time.Now()})
					s.Update(snap)
					i++
				}
			}
		}(i)
	}

	time.Sleep(duration)
	close(stopCh)
	wg.Wait()
}

// TestStoreRaceConcurrentUpdates tests concurrent updates to the same platform.
func TestStoreRaceConcurrentUpdates(t *testing.T) {
	s := New()

	duration := 1 * time.Second
	stopCh := make(chan struct{})
	updateCount := 0
	var countMu sync.Mutex

	var wg sync.WaitGroup

	// Multiple goroutines updating the same platform
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			ticker := time.NewTicker(1 * time.Millisecond)
			defer ticker.Stop()
			i := 0
			for {
				select {
				case <-stopCh:
					return
				case <-ticker.C:
					snap := domain.AccountSnapshot{
						Platform:     "shared-platform",
						AccountAlias: "test",
						Quotas:       make(map[domain.Tier]domain.QuotaTier),
					}
					snap.AddQuota(domain.Tier5H, domain.QuotaTier{Used: int64(idx*1000 + i), Total: 100, ResetAt: time.Now()})
					s.Update(snap)
					countMu.Lock()
					updateCount++
					countMu.Unlock()
					i++
				}
			}
		}(i)
	}

	time.Sleep(duration)
	close(stopCh)
	wg.Wait()

	// Verify final state is consistent
	snap, ok := s.Get("shared-platform")
	if !ok {
		t.Fatal("expected snapshot to exist after concurrent updates")
	}
	if snap.Version == 0 {
		t.Error("expected version > 0 after updates")
	}

	t.Logf("Total updates performed: %d", updateCount)
}

// TestStoreRaceMultiplePlatforms tests concurrent access to multiple platforms.
func TestStoreRaceMultiplePlatforms(t *testing.T) {
	s := New()

	platforms := []string{"codex", "kimi", "minimax", "zai", "zhipu"}
	duration := 500 * time.Millisecond
	stopCh := make(chan struct{})

	var wg sync.WaitGroup

	// Writers for each platform
	for _, platform := range platforms {
		p := platform
		wg.Add(1)
		go func() {
			defer wg.Done()
			ticker := time.NewTicker(2 * time.Millisecond)
			defer ticker.Stop()
			i := 0
			for {
				select {
				case <-stopCh:
					return
				case <-ticker.C:
					snap := domain.AccountSnapshot{
						Platform:     p,
						AccountAlias: p + "-account",
						Quotas:       make(map[domain.Tier]domain.QuotaTier),
					}
					snap.AddQuota(domain.Tier5H, domain.QuotaTier{Used: int64(i), Total: 100, ResetAt: time.Now()})
					s.Update(snap)
					i++
				}
			}
		}()
	}

	// Readers for each platform
	for _, platform := range platforms {
		p := platform
		wg.Add(1)
		go func() {
			defer wg.Done()
			ticker := time.NewTicker(1 * time.Millisecond)
			defer ticker.Stop()
			for {
				select {
				case <-stopCh:
					return
				case <-ticker.C:
					_, _ = s.Get(p)
				}
			}
		}()
	}

	time.Sleep(duration)
	close(stopCh)
	wg.Wait()

	// Verify all platforms have data
	for _, platform := range platforms {
		snap, ok := s.Get(platform)
		if !ok {
			t.Errorf("expected snapshot for %s", platform)
			continue
		}
		if snap.Version == 0 {
			t.Errorf("expected version > 0 for %s", platform)
		}
	}
}

// TestStoreLeakGoroutines verifies no goroutine leaks after shutdown.
func TestStoreLeakGoroutines(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping leak test in short mode")
	}

	before := runtime.NumGoroutine()

	s := New()

	platforms := []string{"codex", "kimi", "minimax", "zai", "zhipu"}
	var wg sync.WaitGroup

	// Start concurrent readers and writers
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				platform := platforms[idx%len(platforms)]
				snap := domain.AccountSnapshot{
					Platform:     platform,
					AccountAlias: "test",
					Quotas:       make(map[domain.Tier]domain.QuotaTier),
				}
				snap.AddQuota(domain.Tier5H, domain.QuotaTier{Used: int64(j), Total: 100, ResetAt: time.Now()})
				s.Update(snap)
				_, _ = s.Get(platform)
			}
		}(i)
	}

	wg.Wait()

	// Allow cleanup time
	time.Sleep(100 * time.Millisecond)

	after := runtime.NumGoroutine()
	leaked := after - before

	// Allow some variance due to runtime behavior
	if leaked > 5 {
		t.Errorf("goroutine leak detected: before=%d, after=%d, leaked=%d", before, after, leaked)
	}
}
