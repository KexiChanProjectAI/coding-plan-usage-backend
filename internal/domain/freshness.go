package domain

import "time"

// Staleness represents whether a quota tier's data is considered stale.
type Staleness struct {
	IsStale bool
	Since   time.Time
}

// IsStaleFor returns true if the staleness duration exceeds maxStaleDuration.
func (s Staleness) IsStaleFor(maxStaleDuration time.Duration) bool {
	if !s.IsStale {
		return false
	}
	return time.Since(s.Since) > maxStaleDuration
}

// Freshness computes staleness for each tier in the snapshot based on LastSync.
func Freshness(snapshot AccountSnapshot, maxStaleDuration time.Duration) map[Tier]Staleness {
	result := make(map[Tier]Staleness)

	if snapshot.LastSync.IsZero() {
		for _, tier := range AllSupportedTiers {
			result[tier] = Staleness{IsStale: false}
		}
		return result
	}

	lastSync := snapshot.LastSync
	for _, tier := range AllSupportedTiers {
		quota, exists := snapshot.Quotas[tier]
		if !exists {
			result[tier] = Staleness{IsStale: false}
			continue
		}
		tierIsStale := lastSync.Sub(quota.ResetAt) > maxStaleDuration
		result[tier] = Staleness{
			IsStale: tierIsStale,
			Since:   quota.ResetAt,
		}
	}

	return result
}