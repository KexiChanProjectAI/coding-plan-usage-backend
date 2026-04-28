package domain

import "time"

// NormalizeToPercent returns the percentage of used/total as an int64.
// It clamps the result to the range [0, 100].
// Returns 0 if total <= 0 or if used is negative.
func NormalizeToPercent(used, total int64) int64 {
	if total <= 0 {
		return 0
	}
	if used < 0 {
		return 0
	}
	percent := (used * 100) / total
	if percent > 100 {
		return 100
	}
	return percent
}

// BackfillCanonicalTiers copies the provided quotas map and adds any missing
// canonical tiers (5H, 1W, 1M) with default values: Used=0, Total=100, ResetAt=time.Time{}.
func BackfillCanonicalTiers(quotas map[Tier]QuotaTier) map[Tier]QuotaTier {
	result := make(map[Tier]QuotaTier, len(quotas))
	for tier, quota := range quotas {
		result[tier] = quota
	}
	for _, tier := range AllSupportedTiers {
		if _, exists := result[tier]; !exists {
			result[tier] = QuotaTier{
				Used:    0,
				Total:   100,
				ResetAt: time.Time{},
			}
		}
	}
	return result
}