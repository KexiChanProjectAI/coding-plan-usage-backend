package domain

import "time"

// Status represents the derived health status of an account.
type Status string

const (
	StatusHealthy      Status = "healthy"
	StatusDegraded     Status = "degraded"
	StatusInitializing Status = "initializing"
)

// DeriveStatus computes the account status based on snapshot state and staleness rules.
// It returns:
//   - StatusInitializing if snapshot.Version == 0
//   - StatusDegraded if any supported tier is stale (LastSync - tier reset > maxStaleDuration)
//   - StatusHealthy otherwise
func DeriveStatus(snapshot AccountSnapshot, maxStaleDuration time.Duration) Status {
	if snapshot.Version == 0 {
		return StatusInitializing
	}

	freshness := Freshness(snapshot, maxStaleDuration)
	for _, tier := range AllSupportedTiers {
		staleness := freshness[tier]
		if staleness.IsStaleFor(maxStaleDuration) {
			return StatusDegraded
		}
	}

	return StatusHealthy
}