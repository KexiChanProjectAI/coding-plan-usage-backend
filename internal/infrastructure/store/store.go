// Package store provides a thread-safe, versioned in-memory store for account snapshots.
package store

import (
	"sync"
	"time"

	"github.com/quotahub/ucpqa/internal/domain"
)

// Store is a thread-safe, versioned in-memory store for account snapshots.
// It uses sync.RWMutex to allow concurrent readers while ensuring exclusive writer access.
// Updates atomically replace the snapshot and increment the version.
type Store struct {
	mu               sync.RWMutex
	snapshots        map[string]*domain.AccountSnapshot // key: platform
	maxStaleDuration time.Duration
	OnUpdate         func(snapshot domain.AccountSnapshot)
}

// New creates a new Store with default behavior.
func New() *Store {
	return NewWithConfig(0)
}

// NewWithConfig creates a new Store with the given stale omission duration.
func NewWithConfig(maxStaleDuration time.Duration) *Store {
	return &Store{
		snapshots:        make(map[string]*domain.AccountSnapshot),
		maxStaleDuration: maxStaleDuration,
	}
}

// Get retrieves the account snapshot for the given platform.
// It returns the snapshot and a boolean indicating whether it was found.
// The returned snapshot is immutable and must not be modified by callers.
func (s *Store) Get(platform string) (domain.AccountSnapshot, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	snap, ok := s.snapshots[platform]
	if !ok {
		return domain.AccountSnapshot{}, false
	}

	return cloneSnapshot(*snap), true
}

// Update atomically updates the snapshot for the given platform.
// It sets LastSync to the current time and increments the Version.
// If the platform does not exist, a new snapshot is created.
// Returns the updated snapshot.
func (s *Store) Update(snapshot domain.AccountSnapshot) domain.AccountSnapshot {
	now := time.Now()
	callback := s.OnUpdate

	s.mu.Lock()
	existing, ok := s.snapshots[snapshot.Platform]
	if ok {
		snapshot.Version = existing.Version + 1
	} else {
		snapshot.Version = 1
	}
	snapshot.LastSync = now
	snapshot = cloneSnapshot(snapshot)
	snapshot = s.omitStaleQuotas(snapshot, now)

	snapCopy := snapshot
	s.snapshots[snapshot.Platform] = &snapCopy
	s.mu.Unlock()

	result := cloneSnapshot(snapshot)
	if callback != nil {
		callback(result)
	}

	return result
}

func (s *Store) omitStaleQuotas(snapshot domain.AccountSnapshot, now time.Time) domain.AccountSnapshot {
	if s.maxStaleDuration <= 0 {
		return snapshot
	}

	for tier, quota := range snapshot.Quotas {
		if now.Sub(quota.ResetAt) > s.maxStaleDuration {
			delete(snapshot.Quotas, tier)
		}
	}

	return snapshot
}

func cloneSnapshot(snapshot domain.AccountSnapshot) domain.AccountSnapshot {
	quotasCopy := make(map[domain.Tier]domain.QuotaTier, len(snapshot.Quotas))
	for tier, quota := range snapshot.Quotas {
		quotasCopy[tier] = quota
	}
	snapshot.Quotas = quotasCopy
	return snapshot
}

// Delete removes the snapshot for the given platform.
func (s *Store) Delete(platform string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.snapshots, platform)
}

// Len returns the number of snapshots in the store.
func (s *Store) Len() int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return len(s.snapshots)
}

// Platforms returns a list of all platforms in the store.
func (s *Store) Platforms() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	platforms := make([]string, 0, len(s.snapshots))
	for p := range s.snapshots {
		platforms = append(platforms, p)
	}
	return platforms
}
