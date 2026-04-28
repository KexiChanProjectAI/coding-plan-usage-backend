package provider

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/quotahub/ucpqa/internal/domain"
)

type mockProvider struct {
	name    string
	snapshot *domain.AccountSnapshot
	fetchErr error
}

func (m *mockProvider) Fetch(ctx context.Context) (domain.AccountSnapshot, error) {
	if m.fetchErr != nil {
		return domain.AccountSnapshot{}, m.fetchErr
	}
	return *m.snapshot, nil
}

func (m *mockProvider) ProviderName() string {
	return m.name
}

func TestProviderInterface(t *testing.T) {
	snapshot := domain.NewAccountSnapshot("test-platform", "test-account", 1)
	snapshot.LastSync = time.Now()

	prov := &mockProvider{
		name:     "test-provider",
		snapshot: snapshot,
	}

	ctx := context.Background()
	result, err := prov.Fetch(ctx)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Platform != "test-platform" {
		t.Errorf("expected platform test-platform, got %s", result.Platform)
	}
	if prov.ProviderName() != "test-provider" {
		t.Errorf("expected provider name test-provider, got %s", prov.ProviderName())
	}
}

func TestProviderErrorsCategorizeFailures(t *testing.T) {
	underlying := errors.New("connection reset")

	tests := []struct {
		name       string
		err        error
		wantType   string
		checkType  func(error) bool
		checkUnwrap func(error) bool
	}{
		{
			name:      "ErrFetchFailure",
			err:       NewErrFetchFailure("codex", underlying),
			wantType:  "ErrFetchFailure",
			checkType: IsFetchFailure,
			checkUnwrap: func(e error) bool {
				return errors.Is(e, underlying)
			},
		},
		{
			name:      "ErrParseFailure",
			err:       NewErrParseFailure("kimi", underlying),
			wantType:  "ErrParseFailure",
			checkType: IsParseFailure,
			checkUnwrap: func(e error) bool {
				return errors.Is(e, underlying)
			},
		},
		{
			name:      "ErrUpstreamRejection",
			err:       NewErrUpstreamRejection("minimax", 401, "invalid token", underlying),
			wantType:  "ErrUpstreamRejection",
			checkType: IsUpstreamRejection,
			checkUnwrap: func(e error) bool {
				return errors.Is(e, underlying)
			},
		},
		{
			name:      "ErrStaleExpiry",
			err:       NewErrStaleExpiry("zhipu", "2024-01-01T00:00:00Z"),
			wantType:  "ErrStaleExpiry",
			checkType: IsStaleExpiry,
			checkUnwrap: func(e error) bool {
				return errors.Unwrap(e) == nil
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !tt.checkType(tt.err) {
				t.Errorf("expected %s to be detected as %s", tt.err.Error(), tt.wantType)
			}
			if !tt.checkUnwrap(tt.err) {
				t.Errorf("expected unwrap check to pass for %s", tt.err.Error())
			}
		})
	}
}

func TestErrorTypeDistinctness(t *testing.T) {
	underlying := errors.New("network error")

	fetchErr := NewErrFetchFailure("codex", underlying)
	parseErr := NewErrParseFailure("kimi", underlying)
	upstreamErr := NewErrUpstreamRejection("minimax", 403, "forbidden", underlying)
	staleErr := NewErrStaleExpiry("zhipu", "2024-01-01T00:00:00Z")

	if errors.As(fetchErr, &parseErr) {
		t.Error("ErrFetchFailure should not match ErrParseFailure type")
	}
	if errors.As(fetchErr, &upstreamErr) {
		t.Error("ErrFetchFailure should not match ErrUpstreamRejection type")
	}
	if errors.As(fetchErr, &staleErr) {
		t.Error("ErrFetchFailure should not match ErrStaleExpiry type")
	}

	if errors.As(parseErr, &fetchErr) {
		t.Error("ErrParseFailure should not match ErrFetchFailure type")
	}
	if errors.As(parseErr, &upstreamErr) {
		t.Error("ErrParseFailure should not match ErrUpstreamRejection type")
	}
	if errors.As(parseErr, &staleErr) {
		t.Error("ErrParseFailure should not match ErrStaleExpiry type")
	}

	if errors.As(upstreamErr, &fetchErr) {
		t.Error("ErrUpstreamRejection should not match ErrFetchFailure type")
	}
	if errors.As(upstreamErr, &parseErr) {
		t.Error("ErrUpstreamRejection should not match ErrParseFailure type")
	}
	if errors.As(upstreamErr, &staleErr) {
		t.Error("ErrUpstreamRejection should not match ErrStaleExpiry type")
	}

	if errors.As(staleErr, &fetchErr) {
		t.Error("ErrStaleExpiry should not match ErrFetchFailure type")
	}
	if errors.As(staleErr, &parseErr) {
		t.Error("ErrStaleExpiry should not match ErrParseFailure type")
	}
	if errors.As(staleErr, &upstreamErr) {
		t.Error("ErrStaleExpiry should not match ErrUpstreamRejection type")
	}
}

func TestFetchResult(t *testing.T) {
	snapshot := domain.NewAccountSnapshot("test", "acc", 1)

	t.Run("success result", func(t *testing.T) {
		result := FetchResult{Snapshot: snapshot, Error: nil}
		if !result.IsSuccess() {
			t.Error("expected IsSuccess() to be true for successful result")
		}
	})

	t.Run("failure result", func(t *testing.T) {
		result := FetchResult{Snapshot: nil, Error: NewErrFetchFailure("test", errors.New("fail"))}
		if result.IsSuccess() {
			t.Error("expected IsSuccess() to be false for failed result")
		}
	})
}

func TestProviderReturnsCorrectErrorTypes(t *testing.T) {
	underlying := errors.New("timeout")

	providers := []struct {
		name  string
		err   error
		check func(error) bool
	}{
		{name: "codex-fetch", err: NewErrFetchFailure("codex", underlying), check: IsFetchFailure},
		{name: "kimi-parse", err: NewErrParseFailure("kimi", underlying), check: IsParseFailure},
		{name: "minimax-upstream", err: NewErrUpstreamRejection("minimax", 500, "server error", underlying), check: IsUpstreamRejection},
		{name: "zhipu-stale", err: NewErrStaleExpiry("zhipu", "2024-01-01T00:00:00Z"), check: IsStaleExpiry},
	}

	for _, p := range providers {
		t.Run(p.name, func(t *testing.T) {
			if !p.check(p.err) {
				t.Errorf("expected error to be detected as correct type: %s", p.err.Error())
			}
		})
	}
}