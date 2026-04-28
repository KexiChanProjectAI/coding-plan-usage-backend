package provider

import (
	"context"

	"github.com/quotahub/ucpqa/internal/domain"
)

// Provider defines the interface for fetching account quota snapshots from an upstream provider.
// Implementations must be stateless and return immutable snapshot fragments or typed errors.
// No transport-layer coupling (e.g., Gin, HTTP handlers) is allowed in implementations.
type Provider interface {
	// Fetch retrieves the current account quota snapshot from the upstream provider.
	// It returns an AccountSnapshot on success or a typed error on failure.
	Fetch(ctx context.Context) (domain.AccountSnapshot, error)

	// ProviderName returns a human-readable identifier for this provider.
	// This is used for logging, metrics, and error messages.
	ProviderName() string
}

// FetchResult represents the outcome of a Fetch operation with richer context than a plain return.
// It can be used when callers need to distinguish between success with warnings vs pure success.
type FetchResult struct {
	// Snapshot is the fetched account snapshot. It is nil if Fetch failed.
	Snapshot *domain.AccountSnapshot

	// Error is the typed error that occurred during fetch, if any.
	// If Error is non-nil, Snapshot must be nil.
	Error error
}

// IsSuccess returns true if the fetch completed without error.
func (r FetchResult) IsSuccess() bool {
	return r.Error == nil && r.Snapshot != nil
}