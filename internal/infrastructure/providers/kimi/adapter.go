// Package kimi provides the Kimi provider adapter.
package kimi

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/quotahub/ucpqa/internal/domain"
	"github.com/quotahub/ucpqa/internal/domain/provider"
)

// Adapter is the Kimi provider adapter.
type Adapter struct {
	name      string
	baseURL   string
	token     string
	httpClient *http.Client
}

// New creates a new Kimi provider adapter.
func New(name, baseURL, token string) *Adapter {
	return &Adapter{
		name:      name,
		baseURL:   baseURL,
		token:     token,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// NewWithClient creates a new Kimi provider adapter with a custom HTTP client.
func NewWithClient(name, baseURL, token string, client *http.Client) *Adapter {
	return &Adapter{
		name:      name,
		baseURL:   baseURL,
		token:     token,
		httpClient: client,
	}
}

func (a *Adapter) ProviderName() string {
	return a.name
}

func (a *Adapter) Fetch(ctx context.Context) (domain.AccountSnapshot, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, a.baseURL+"/coding/v1/usages", nil)
	if err != nil {
		return domain.AccountSnapshot{}, provider.NewErrFetchFailure(a.name, err)
	}

	req.Header.Set("Authorization", "Bearer "+a.token)

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return domain.AccountSnapshot{}, provider.NewErrFetchFailure(a.name, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return domain.AccountSnapshot{}, provider.NewErrUpstreamRejection(a.name, resp.StatusCode, "", nil)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return domain.AccountSnapshot{}, provider.NewErrParseFailure(a.name, err)
	}

	var result APIResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return domain.AccountSnapshot{}, provider.NewErrParseFailure(a.name, err)
	}

	snapshot := domain.NewAccountSnapshot(a.name, "default", 0)

	// Process usage section if present with valid resetTime
	if result.Usage.ResetTime != "" {
		limit, err := parseInt64(result.Usage.Limit)
		if err != nil {
			return domain.AccountSnapshot{}, provider.NewErrParseFailure(a.name, err)
		}
		remaining, err := parseInt64(result.Usage.Remaining)
		if err != nil {
			return domain.AccountSnapshot{}, provider.NewErrParseFailure(a.name, err)
		}
		resetAt, err := time.Parse(time.RFC3339, result.Usage.ResetTime)
		if err != nil {
			return domain.AccountSnapshot{}, provider.NewErrParseFailure(a.name, err)
		}

		// Determine tier based on reset time
		tier := inferTierFromResetTime(resetAt)
		snapshot.AddQuota(tier, domain.QuotaTier{
			Used:   limit - remaining,
			Total:  limit,
			ResetAt: resetAt,
		})
	}

	// Process limits array
	for _, limit := range result.Limits {
		durationSeconds := windowDurationToSeconds(limit.Window.Duration, limit.Window.TimeUnit)
		if durationSeconds == 0 {
			continue
		}

		tier := mapDurationToTier(durationSeconds)
		if tier == "" {
			continue
		}

		limitVal, err := parseInt64(limit.Detail.Limit)
		if err != nil {
			return domain.AccountSnapshot{}, provider.NewErrParseFailure(a.name, err)
		}
		remainingVal, err := parseInt64(limit.Detail.Remaining)
		if err != nil {
			return domain.AccountSnapshot{}, provider.NewErrParseFailure(a.name, err)
		}

		var resetAt time.Time
		if limit.Detail.ResetTime != "" {
			resetAt, err = time.Parse(time.RFC3339, limit.Detail.ResetTime)
			if err != nil {
				return domain.AccountSnapshot{}, provider.NewErrParseFailure(a.name, err)
			}
		}

		snapshot.AddQuota(tier, domain.QuotaTier{
			Used:   limitVal - remainingVal,
			Total:  limitVal,
			ResetAt: resetAt,
		})
	}

	return *snapshot, nil
}

// parseInt64 parses a string to int64, returning ErrParseFailure on error.
func parseInt64(s string) (int64, error) {
	if s == "" {
		return 0, nil
	}
	val, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, err
	}
	return val, nil
}

// windowDurationToSeconds converts a window duration with timeUnit to seconds.
func windowDurationToSeconds(duration int64, timeUnit string) int64 {
	switch timeUnit {
	case "TIME_UNIT_MINUTE":
		return duration * 60
	case "TIME_UNIT_HOUR":
		return duration * 3600
	case "TIME_UNIT_DAY":
		return duration * 86400
	case "TIME_UNIT_MONTH":
		return duration * 2592000 // approximate seconds in a month
	default:
		return 0
	}
}

// mapDurationToTier maps a duration in seconds to a canonical tier.
// Returns empty string if the duration is not supported.
func mapDurationToTier(durationSeconds int64) domain.Tier {
	// Any limit with duration <= 5 hours (18000 seconds) maps to 5H
	if durationSeconds <= 5*3600 {
		return domain.Tier5H
	}
	// Any limit with duration ~1 week (7 days = 604800 seconds) maps to 1W
	if durationSeconds >= 6*86400 && durationSeconds <= 8*86400 {
		return domain.Tier1W
	}
	// Any limit with duration ~1 month (~2592000 seconds) maps to 1M
	if durationSeconds >= 28*86400 {
		return domain.Tier1M
	}
	return ""
}

// inferTierFromResetTime infers the canonical tier from a reset time.
func inferTierFromResetTime(resetAt time.Time) domain.Tier {
	now := time.Now()
	duration := resetAt.Sub(now)

	// If reset is within ~5 hours, it's likely a 5H tier
	if duration <= 5*time.Hour {
		return domain.Tier5H
	}
	// If reset is within ~1 week, it's likely a 1W tier
	if duration <= 8*24*time.Hour {
		return domain.Tier1W
	}
	// Otherwise, assume 1M tier
	return domain.Tier1M
}

// APIResponse represents the Kimi API response structure.
type APIResponse struct {
	User       *User       `json:"user,omitempty"`
	Usage      Usage       `json:"usage,omitempty"`
	Limits     []Limit     `json:"limits,omitempty"`
	Parallel   Parallel    `json:"parallel,omitempty"`
	TotalQuota TotalQuota `json:"totalQuota,omitempty"`
}

// User represents user information in the API response.
type User struct {
	UserID     *string   `json:"userId,omitempty"`
	Region     *string   `json:"region,omitempty"`
	Membership *string   `json:"membership,omitempty"`
	BusinessID *string   `json:"businessId,omitempty"`
}

// Usage represents usage quota information.
type Usage struct {
	Limit      string `json:"limit,omitempty"`
	Remaining  string `json:"remaining,omitempty"`
	ResetTime  string `json:"resetTime,omitempty"`
}

// Limit represents a windowed limit entry.
type Limit struct {
	Window Window  `json:"window,omitempty"`
	Detail Detail  `json:"detail,omitempty"`
}

// Window represents the time window for a limit.
type Window struct {
	Duration int64  `json:"duration,omitempty"`
	TimeUnit string `json:"timeUnit,omitempty"`
}

// Detail represents the detail of a limit.
type Detail struct {
	Limit     string `json:"limit,omitempty"`
	Remaining string `json:"remaining,omitempty"`
	ResetTime string `json:"resetTime,omitempty"`
}

// Parallel represents parallel/concurrency information.
type Parallel struct {
	Limit string `json:"limit,omitempty"`
}

// TotalQuota represents total quota information.
type TotalQuota struct {
	Limit     string `json:"limit,omitempty"`
	Remaining string `json:"remaining,omitempty"`
}

// Compile-time check that Adapter implements provider.Provider interface.
var _ provider.Provider = (*Adapter)(nil)