package domain

import (
	"testing"
	"time"
)

func TestNormalizeToPercent(t *testing.T) {
	tests := []struct {
		name     string
		used     int64
		total    int64
		expected int64
	}{
		{"normal 25%", 250, 1000, 25},
		{"normal 50%", 50, 100, 50},
		{"normal 100%", 100, 100, 100},
		{"zero total", 50, 0, 0},
		{"negative total", 50, -10, 0},
		{"negative used", -50, 100, 0},
		{"both negative", -50, -10, 0},
		{"used greater than total clamps to 100", 150, 100, 100},
		{"used way greater than total clamps to 100", 200, 100, 100},
		{"zero used", 0, 100, 0},
		{"zero used zero total", 0, 0, 0},
		{"large numbers", 999999, 1000000, 99},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NormalizeToPercent(tt.used, tt.total)
			if result != tt.expected {
				t.Errorf("NormalizeToPercent(%d, %d) = %d; want %d", tt.used, tt.total, result, tt.expected)
			}
		})
	}
}

func TestBackfillCanonicalTiers(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name          string
		input         map[Tier]QuotaTier
		expectedTiers []Tier
		expectMissing bool
	}{
		{
			name:          "empty map gets all tiers",
			input:         map[Tier]QuotaTier{},
			expectedTiers: []Tier{Tier5H, Tier1W, Tier1M},
			expectMissing: false,
		},
		{
			name: "partial tiers get filled",
			input: map[Tier]QuotaTier{
				Tier5H: {Used: 10, Total: 100, ResetAt: now},
			},
			expectedTiers: []Tier{Tier5H, Tier1W, Tier1M},
			expectMissing: false,
		},
		{
			name: "all tiers present unchanged",
			input: map[Tier]QuotaTier{
				Tier5H: {Used: 10, Total: 100, ResetAt: now},
				Tier1W: {Used: 20, Total: 200, ResetAt: now},
				Tier1M: {Used: 30, Total: 300, ResetAt: now},
			},
			expectedTiers: []Tier{Tier5H, Tier1W, Tier1M},
			expectMissing: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := BackfillCanonicalTiers(tt.input)

			for _, tier := range tt.expectedTiers {
				quota, exists := result[tier]
				if !exists {
					t.Errorf("BackfillCanonicalTiers missing tier %v", tier)
					continue
				}
				if tt.expectMissing {
					original, hadOriginal := tt.input[tier]
					if !hadOriginal {
						if quota.Used != 0 || quota.Total != 100 || !quota.ResetAt.IsZero() {
							t.Errorf("tier %v backfill value incorrect: got Used=%d Total=%d ResetAt=%v; want Used=0 Total=100 ResetAt=zero",
								tier, quota.Used, quota.Total, quota.ResetAt)
						}
					} else {
						if quota != original {
							t.Errorf("tier %v changed unexpectedly: got %+v; want %+v", tier, quota, original)
						}
					}
				}
			}

			if len(result) != len(tt.expectedTiers) {
				t.Errorf("BackfillCanonicalTiers result length = %d; want %d", len(result), len(tt.expectedTiers))
			}
		})
	}
}

func TestBackfillCanonicalTiers_PreservesExisting(t *testing.T) {
	now := time.Now()
	existing := map[Tier]QuotaTier{
		Tier5H: {Used: 10, Total: 100, ResetAt: now},
		Tier1W: {Used: 20, Total: 200, ResetAt: now},
	}

	result := BackfillCanonicalTiers(existing)

	if result[Tier5H] != existing[Tier5H] {
		t.Errorf("Tier5H was modified: got %+v; want %+v", result[Tier5H], existing[Tier5H])
	}
	if result[Tier1W] != existing[Tier1W] {
		t.Errorf("Tier1W was modified: got %+v; want %+v", result[Tier1W], existing[Tier1W])
	}

	if result[Tier1M].Used != 0 || result[Tier1M].Total != 100 || !result[Tier1M].ResetAt.IsZero() {
		t.Errorf("Tier1M backfill incorrect: got Used=%d Total=%d ResetAt=%v; want Used=0 Total=100 ResetAt=zero",
			result[Tier1M].Used, result[Tier1M].Total, result[Tier1M].ResetAt)
	}
}

func TestBackfillCanonicalTiers_ZeroResetAt(t *testing.T) {
	result := BackfillCanonicalTiers(map[Tier]QuotaTier{})

	for _, tier := range AllSupportedTiers {
		if !result[tier].ResetAt.IsZero() {
			t.Errorf("tier %v ResetAt should be zero but got %v", tier, result[tier].ResetAt)
		}
	}
}