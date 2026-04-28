package domain

import "time"

// QuotaTier represents usage and limits for a single quota tier window.
type QuotaTier struct {
	Used   int64     `json:"used"`
	Total  int64     `json:"total"`
	ResetAt time.Time `json:"reset_at"`
}

// AccountSnapshot is the canonical representation of an account's quota state
// at a point in time. It is immutable once created.
type AccountSnapshot struct {
	Platform     string            `json:"platform"`
	AccountAlias string            `json:"account_alias"`
	Quotas       map[Tier]QuotaTier `json:"quotas,omitempty"`
	LastSync     time.Time         `json:"last_sync"`
	Version      int64             `json:"version"`
}

// NewAccountSnapshot creates a new account snapshot with the given fields.
// Quotas map is initialized empty and must be populated by the caller.
func NewAccountSnapshot(platform, accountAlias string, version int64) *AccountSnapshot {
	return &AccountSnapshot{
		Platform:     platform,
		AccountAlias: accountAlias,
		Quotas:       make(map[Tier]QuotaTier),
		LastSync:     time.Time{},
		Version:      version,
	}
}

// AddQuota adds or updates a quota tier in the snapshot.
// If the tier is not supported, it is silently ignored.
func (s *AccountSnapshot) AddQuota(tier Tier, quota QuotaTier) {
	if !tier.IsSupported() {
		return
	}
	s.Quotas[tier] = quota
}