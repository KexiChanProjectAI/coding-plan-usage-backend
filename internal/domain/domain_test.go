package domain

import (
	"encoding/json"
	"testing"
	"time"
)

func TestDeriveStatusDegradedWhenAnyTierStale(t *testing.T) {
	now := time.Now()
	maxStale := 30 * time.Minute

	snapshot := AccountSnapshot{
		Platform:     "test-provider",
		AccountAlias: "test-account",
		Version:      1,
		LastSync:     now,
		Quotas: map[Tier]QuotaTier{
			Tier5H: {
				Used:    50,
				Total:   100,
				ResetAt: now.Add(-1 * time.Hour),
			},
			Tier1W: {
				Used:    10,
				Total:   50,
				ResetAt: now.Add(-2 * time.Hour),
			},
		},
	}

	status := DeriveStatus(snapshot, maxStale)
	if status != StatusDegraded {
		t.Errorf("expected StatusDegraded when any tier is stale, got %s", status)
	}
}

func TestDeriveStatusHealthyWhenAllTiersFresh(t *testing.T) {
	now := time.Now()
	maxStale := 30 * time.Minute

	snapshot := AccountSnapshot{
		Platform:     "test-provider",
		AccountAlias: "test-account",
		Version:      1,
		LastSync:     now,
		Quotas: map[Tier]QuotaTier{
			Tier5H: {
				Used:    50,
				Total:   100,
				ResetAt: now.Add(-10 * time.Minute),
			},
			Tier1W: {
				Used:    10,
				Total:   50,
				ResetAt: now.Add(-20 * time.Minute),
			},
		},
	}

	status := DeriveStatus(snapshot, maxStale)
	if status != StatusHealthy {
		t.Errorf("expected StatusHealthy when all tiers are fresh, got %s", status)
	}
}

func TestDeriveStatusInitializingWhenVersionZero(t *testing.T) {
	now := time.Now()
	maxStale := 30 * time.Minute

	snapshot := AccountSnapshot{
		Platform:     "test-provider",
		AccountAlias: "test-account",
		Version:      0,
		LastSync:     now,
		Quotas:       make(map[Tier]QuotaTier),
	}

	status := DeriveStatus(snapshot, maxStale)
	if status != StatusInitializing {
		t.Errorf("expected StatusInitializing when version is 0, got %s", status)
	}
}

func TestSnapshotOmitsUnsupportedTiers(t *testing.T) {
	snapshot := AccountSnapshot{
		Platform:     "test-provider",
		AccountAlias: "test-account",
		Version:      1,
		LastSync:     time.Now(),
		Quotas: map[Tier]QuotaTier{
			Tier5H: {
				Used:    50,
				Total:   100,
				ResetAt: time.Now(),
			},
		},
	}

	data, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	var decoded map[string]interface{}
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	quotas, ok := decoded["quotas"].(map[string]interface{})
	if !ok {
		t.Fatal("quotas field missing or not a map")
	}

	if _, exists := quotas["5H"]; !exists {
		t.Error("5H tier should be present in JSON")
	}

	if _, exists := quotas["1W"]; exists {
		t.Error("1W tier should be omitted from JSON")
	}

	if _, exists := quotas["1M"]; exists {
		t.Error("1M tier should be omitted from JSON")
	}
}

func TestSnapshotOmitsUnsupportedTierFromUnknownProvider(t *testing.T) {
	snapshot := NewAccountSnapshot("unknown-provider", "account", 1)
	snapshot.AddQuota(Tier("3H"), QuotaTier{Used: 10, Total: 100, ResetAt: time.Now()})
	snapshot.AddQuota(Tier5H, QuotaTier{Used: 50, Total: 100, ResetAt: time.Now()})
	snapshot.AddQuota(Tier("2D"), QuotaTier{Used: 5, Total: 50, ResetAt: time.Now()})

	data, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	var decoded map[string]interface{}
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	quotas, ok := decoded["quotas"].(map[string]interface{})
	if !ok {
		t.Fatal("quotas field missing or not a map")
	}

	if _, exists := quotas["5H"]; !exists {
		t.Error("5H tier should be present in JSON")
	}

	if _, exists := quotas["3H"]; exists {
		t.Error("3H (unsupported) tier should be omitted from JSON")
	}

	if _, exists := quotas["2D"]; exists {
		t.Error("2D (unsupported) tier should be omitted from JSON")
	}
}

func TestQuotaTierSerializesResetAtAsRFC3339(t *testing.T) {
	resetTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
	quota := QuotaTier{
		Used:    50,
		Total:   100,
		ResetAt: resetTime,
	}

	data, err := json.Marshal(quota)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	var decoded map[string]interface{}
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	resetAtStr, ok := decoded["reset_at"].(string)
	if !ok {
		t.Fatal("reset_at is not a string in JSON")
	}

	expected := "2024-01-15T10:30:00Z"
	if resetAtStr != expected {
		t.Errorf("reset_at should be RFC3339 format, got %s expected %s", resetAtStr, expected)
	}
}