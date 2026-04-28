package store

import (
	"sync"
	"testing"
	"time"

	"github.com/quotahub/ucpqa/internal/domain"
)

func TestNewStore(t *testing.T) {
	s := New()
	if s == nil {
		t.Fatal("New() returned nil")
	}
	if s.Len() != 0 {
		t.Errorf("expected empty store, got length %d", s.Len())
	}
}

func TestStoreGetNotFound(t *testing.T) {
	s := New()
	_, ok := s.Get("nonexistent")
	if ok {
		t.Error("expected not found for nonexistent platform")
	}
}

func TestStoreUpdateNewSnapshot(t *testing.T) {
	s := New()

	snap := domain.AccountSnapshot{
		Platform:     "minimax",
		AccountAlias: "test-account",
		Quotas:       make(map[domain.Tier]domain.QuotaTier),
		Version:      0,
	}
	snap.AddQuota(domain.Tier5H, domain.QuotaTier{
		Used:    10,
		Total:   100,
		ResetAt: time.Now().Add(5 * time.Hour),
	})

	updated := s.Update(snap)

	if updated.Version != 1 {
		t.Errorf("expected version 1 for new snapshot, got %d", updated.Version)
	}
	if updated.LastSync.IsZero() {
		t.Error("expected LastSync to be set")
	}

	retrieved, ok := s.Get("minimax")
	if !ok {
		t.Fatal("expected to retrieve minimax snapshot")
	}
	if retrieved.Version != 1 {
		t.Errorf("expected retrieved version 1, got %d", retrieved.Version)
	}
	if retrieved.AccountAlias != "test-account" {
		t.Errorf("expected account alias test-account, got %s", retrieved.AccountAlias)
	}
}

func TestStoreUpdateIncrementsVersion(t *testing.T) {
	s := New()

	snap := domain.AccountSnapshot{
		Platform:     "minimax",
		AccountAlias: "test-account",
		Quotas:       make(map[domain.Tier]domain.QuotaTier),
	}
	snap.AddQuota(domain.Tier5H, domain.QuotaTier{Used: 10, Total: 100, ResetAt: time.Now()})

	v1 := s.Update(snap)
	if v1.Version != 1 {
		t.Errorf("expected version 1, got %d", v1.Version)
	}

	v2 := s.Update(snap)
	if v2.Version != 2 {
		t.Errorf("expected version 2, got %d", v2.Version)
	}

	v3 := s.Update(snap)
	if v3.Version != 3 {
		t.Errorf("expected version 3, got %d", v3.Version)
	}
}

func TestStoreUpdateSetsLastSync(t *testing.T) {
	s := New()

	before := time.Now().Add(-time.Second)

	snap := domain.AccountSnapshot{
		Platform:     "minimax",
		AccountAlias: "test",
		Quotas:       make(map[domain.Tier]domain.QuotaTier),
	}
	snap.AddQuota(domain.Tier5H, domain.QuotaTier{Used: 5, Total: 50, ResetAt: time.Now()})

	updated := s.Update(snap)

	if updated.LastSync.Before(before) {
		t.Error("LastSync was not updated to current time")
	}
}

func TestStoreDelete(t *testing.T) {
	s := New()

	snap := domain.AccountSnapshot{
		Platform:     "minimax",
		AccountAlias: "test",
		Quotas:       make(map[domain.Tier]domain.QuotaTier),
	}
	s.Update(snap)

	s.Delete("minimax")

	_, ok := s.Get("minimax")
	if ok {
		t.Error("expected snapshot to be deleted")
	}
	if s.Len() != 0 {
		t.Errorf("expected empty store, got length %d", s.Len())
	}
}

func TestStorePlatforms(t *testing.T) {
	s := New()

	s.Update(domain.AccountSnapshot{
		Platform:     "codex",
		AccountAlias: "c1",
		Quotas:       make(map[domain.Tier]domain.QuotaTier),
	})
	s.Update(domain.AccountSnapshot{
		Platform:     "kimi",
		AccountAlias: "k1",
		Quotas:       make(map[domain.Tier]domain.QuotaTier),
	})
	s.Update(domain.AccountSnapshot{
		Platform:     "minimax",
		AccountAlias: "m1",
		Quotas:       make(map[domain.Tier]domain.QuotaTier),
	})

	platforms := s.Platforms()
	if len(platforms) != 3 {
		t.Errorf("expected 3 platforms, got %d", len(platforms))
	}

	platformSet := make(map[string]bool)
	for _, p := range platforms {
		platformSet[p] = true
	}
	for _, expected := range []string{"codex", "kimi", "minimax"} {
		if !platformSet[expected] {
			t.Errorf("expected platform %s not found", expected)
		}
	}
}

func TestStoreGetReturnsCopy(t *testing.T) {
	s := New()

	snap := domain.AccountSnapshot{
		Platform:     "minimax",
		AccountAlias: "test",
		Quotas:       make(map[domain.Tier]domain.QuotaTier),
	}
	snap.AddQuota(domain.Tier5H, domain.QuotaTier{Used: 10, Total: 100, ResetAt: time.Now()})
	s.Update(snap)

	retrieved, _ := s.Get("minimax")
	retrieved.AccountAlias = "modified"

	original, _ := s.Get("minimax")
	if original.AccountAlias != "test" {
		t.Error("Get should return a copy; modifying returned value should not affect stored value")
	}
}

func TestStoreConcurrentReads(t *testing.T) {
	s := New()

	snap := domain.AccountSnapshot{
		Platform:     "minimax",
		AccountAlias: "test",
		Quotas:       make(map[domain.Tier]domain.QuotaTier),
	}
	for i := 0; i < 100; i++ {
		snap.AddQuota(domain.Tier5H, domain.QuotaTier{Used: int64(i), Total: 100, ResetAt: time.Now()})
		s.Update(snap)
	}

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				_, _ = s.Get("minimax")
			}
		}()
	}

	wg.Wait()
}

func TestStoreConcurrentUpdates(t *testing.T) {
	s := New()

	platforms := []string{"codex", "kimi", "minimax", "zai", "zhipu"}

	var wg sync.WaitGroup
	for _, platform := range platforms {
		p := platform
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 50; i++ {
				snap := domain.AccountSnapshot{
					Platform:     p,
					AccountAlias: p + "-account",
					Quotas:       make(map[domain.Tier]domain.QuotaTier),
				}
				snap.AddQuota(domain.Tier5H, domain.QuotaTier{Used: int64(i), Total: 100, ResetAt: time.Now()})
				s.Update(snap)
			}
		}()
	}

	wg.Wait()

	if s.Len() != len(platforms) {
		t.Errorf("expected %d platforms, got %d", len(platforms), s.Len())
	}

	for _, platform := range platforms {
		snap, ok := s.Get(platform)
		if !ok {
			t.Errorf("platform %s not found", platform)
			continue
		}
		if snap.Version != 50 {
			t.Errorf("platform %s: expected version 50, got %d", platform, snap.Version)
		}
	}
}

func TestStoreImmutability(t *testing.T) {
	s := New()

	snap := domain.AccountSnapshot{
		Platform:     "minimax",
		AccountAlias: "original",
		Quotas:       make(map[domain.Tier]domain.QuotaTier),
	}
	snap.AddQuota(domain.Tier5H, domain.QuotaTier{Used: 10, Total: 100, ResetAt: time.Now()})
	snap.AddQuota(domain.Tier1W, domain.QuotaTier{Used: 20, Total: 200, ResetAt: time.Now()})

	original := s.Update(snap)

	original.Quotas[domain.Tier1M] = domain.QuotaTier{Used: 30, Total: 300, ResetAt: time.Now()}

	retrieved, _ := s.Get("minimax")
	if _, has1M := retrieved.Quotas[domain.Tier1M]; has1M {
		t.Error("stored snapshot should not have 1M tier after modification of returned copy")
	}

	original.Quotas[domain.Tier5H] = domain.QuotaTier{Used: 999, Total: 999, ResetAt: time.Now()}

	retrieved2, _ := s.Get("minimax")
	if retrieved2.Quotas[domain.Tier5H].Used == 999 {
		t.Error("stored snapshot should not have modified used value")
	}
}