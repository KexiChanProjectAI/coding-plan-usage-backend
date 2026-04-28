package store

import (
	"sync"
	"testing"
	"time"

	"github.com/quotahub/ucpqa/internal/domain"
)

// BenchmarkStoreRead benchmarks store.Get() under concurrent read load.
// Target: < 1000 ns/op
func BenchmarkStoreRead(b *testing.B) {
	s := New()

	// Pre-populate store with 5 platforms
	platforms := []string{"codex", "kimi", "minimax", "zai", "zhipu"}
	for _, platform := range platforms {
		snap := domain.AccountSnapshot{
			Platform:     platform,
			AccountAlias: platform + "-account",
			Quotas:       make(map[domain.Tier]domain.QuotaTier),
		}
		snap.AddQuota(domain.Tier5H, domain.QuotaTier{Used: 10, Total: 100, ResetAt: time.Now().Add(5 * time.Hour)})
		snap.AddQuota(domain.Tier1W, domain.QuotaTier{Used: 20, Total: 200, ResetAt: time.Now().Add(7 * 24 * time.Hour)})
		snap.AddQuota(domain.Tier1M, domain.QuotaTier{Used: 30, Total: 300, ResetAt: time.Now().Add(30 * 24 * time.Hour)})
		s.Update(snap)
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, _ = s.Get("minimax")
		}
	})
}

// BenchmarkStoreReadWide benchmarks store.Get() across multiple platforms in parallel.
func BenchmarkStoreReadWide(b *testing.B) {
	s := New()

	platforms := []string{"codex", "kimi", "minimax", "zai", "zhipu"}
	for _, platform := range platforms {
		snap := domain.AccountSnapshot{
			Platform:     platform,
			AccountAlias: platform + "-account",
			Quotas:       make(map[domain.Tier]domain.QuotaTier),
		}
		snap.AddQuota(domain.Tier5H, domain.QuotaTier{Used: 10, Total: 100, ResetAt: time.Now().Add(5 * time.Hour)})
		s.Update(snap)
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			for _, platform := range platforms {
				_, _ = s.Get(platform)
			}
		}
	})
}

// BenchmarkStoreWrite benchmarks store.Update() under concurrent write load.
func BenchmarkStoreWrite(b *testing.B) {
	s := New()

	platforms := []string{"codex", "kimi", "minimax", "zai", "zhipu"}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			platform := platforms[i%len(platforms)]
			i++
			snap := domain.AccountSnapshot{
				Platform:     platform,
				AccountAlias: platform + "-account",
				Quotas:       make(map[domain.Tier]domain.QuotaTier),
			}
			snap.AddQuota(domain.Tier5H, domain.QuotaTier{Used: int64(i), Total: 100, ResetAt: time.Now()})
			s.Update(snap)
		}
	})
}

// BenchmarkStoreReadWriteMixed benchmarks concurrent reads and writes.
func BenchmarkStoreReadWriteMixed(b *testing.B) {
	s := New()

	platforms := []string{"codex", "kimi", "minimax", "zai", "zhipu"}
	for _, platform := range platforms {
		snap := domain.AccountSnapshot{
			Platform:     platform,
			AccountAlias: platform + "-account",
			Quotas:       make(map[domain.Tier]domain.QuotaTier),
		}
		snap.AddQuota(domain.Tier5H, domain.QuotaTier{Used: 10, Total: 100, ResetAt: time.Now()})
		s.Update(snap)
	}

	var wg sync.WaitGroup
	readers := 4
	writers := 2

	b.ResetTimer()

	wg.Add(readers + writers)

	// Readers
	for i := 0; i < readers; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < b.N/readers; j++ {
				_, _ = s.Get("minimax")
			}
		}()
	}

	// Writers
	for i := 0; i < writers; i++ {
		go func(idx int) {
			defer wg.Done()
			for j := 0; j < b.N/writers; j++ {
				snap := domain.AccountSnapshot{
					Platform:     platforms[idx%len(platforms)],
					AccountAlias: "test",
					Quotas:       make(map[domain.Tier]domain.QuotaTier),
				}
				snap.AddQuota(domain.Tier5H, domain.QuotaTier{Used: int64(j), Total: 100, ResetAt: time.Now()})
				s.Update(snap)
			}
		}(i)
	}

	wg.Wait()
}
