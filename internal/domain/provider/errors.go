package provider

import (
	"errors"
	"fmt"
)

// ErrFetchFailure represents a network or transport-level error when communicating
// with the upstream provider. This includes connection timeouts, DNS failures,
// TLS handshake errors, and unexpected connection drops.
type ErrFetchFailure struct {
	ProviderName string
	Cause        error
}

func (e *ErrFetchFailure) Error() string {
	return fmt.Sprintf("fetch failure for provider %q: %v", e.ProviderName, e.Cause)
}

func (e *ErrFetchFailure) Unwrap() error {
	return e.Cause
}

// ErrParseFailure represents a JSON parsing error or schema mismatch when decoding
// the upstream provider's response. The response was received but could not be
// interpreted as expected.
type ErrParseFailure struct {
	ProviderName string
	Cause        error
}

func (e *ErrParseFailure) Error() string {
	return fmt.Sprintf("parse failure for provider %q: %v", e.ProviderName, e.Cause)
}

func (e *ErrParseFailure) Unwrap() error {
	return e.Cause
}

// ErrUpstreamRejection represents an error response from the upstream provider,
// such as an HTTP error status code, a rejection envelope, or an explicit
// error message indicating the request was denied or invalid.
type ErrUpstreamRejection struct {
	ProviderName string
	StatusCode   int
	Message      string
	Cause        error
}

func (e *ErrUpstreamRejection) Error() string {
	if e.Message != "" {
		return fmt.Sprintf("upstream rejection for provider %q (status %d): %s", e.ProviderName, e.StatusCode, e.Message)
	}
	return fmt.Sprintf("upstream rejection for provider %q (status %d)", e.ProviderName, e.StatusCode)
}

func (e *ErrUpstreamRejection) Unwrap() error {
	return e.Cause
}

// ErrStaleExpiry indicates that the fetched data is too old to be useful.
// This is used by the sync manager to determine when to retry a fetch
// or mark an account as degraded due to stale quota information.
type ErrStaleExpiry struct {
	ProviderName string
	Since        string
}

func (e *ErrStaleExpiry) Error() string {
	return fmt.Sprintf("stale expiry for provider %q (data since %s)", e.ProviderName, e.Since)
}

// NewErrFetchFailure creates a new fetch failure error.
func NewErrFetchFailure(providerName string, cause error) *ErrFetchFailure {
	return &ErrFetchFailure{ProviderName: providerName, Cause: cause}
}

// NewErrParseFailure creates a new parse failure error.
func NewErrParseFailure(providerName string, cause error) *ErrParseFailure {
	return &ErrParseFailure{ProviderName: providerName, Cause: cause}
}

// NewErrUpstreamRejection creates a new upstream rejection error.
func NewErrUpstreamRejection(providerName string, statusCode int, message string, cause error) *ErrUpstreamRejection {
	return &ErrUpstreamRejection{ProviderName: providerName, StatusCode: statusCode, Message: message, Cause: cause}
}

// NewErrStaleExpiry creates a new stale expiry error.
func NewErrStaleExpiry(providerName string, since string) *ErrStaleExpiry {
	return &ErrStaleExpiry{ProviderName: providerName, Since: since}
}

// IsFetchFailure returns true if the error is a fetch failure.
func IsFetchFailure(err error) bool {
	var fetchErr *ErrFetchFailure
	return errors.As(err, &fetchErr)
}

// IsParseFailure returns true if the error is a parse failure.
func IsParseFailure(err error) bool {
	var parseErr *ErrParseFailure
	return errors.As(err, &parseErr)
}

// IsUpstreamRejection returns true if the error is an upstream rejection.
func IsUpstreamRejection(err error) bool {
	var upstreamErr *ErrUpstreamRejection
	return errors.As(err, &upstreamErr)
}

// IsStaleExpiry returns true if the error is a stale expiry.
func IsStaleExpiry(err error) bool {
	var staleErr *ErrStaleExpiry
	return errors.As(err, &staleErr)
}