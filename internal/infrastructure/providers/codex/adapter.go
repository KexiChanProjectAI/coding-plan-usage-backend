package codex

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/quotahub/ucpqa/internal/domain"
	"github.com/quotahub/ucpqa/internal/domain/provider"
)

const (
	seconds5H = 18000
	seconds1W = 604800
	seconds1M = 2592000
)

type Adapter struct {
	name      string
	baseURL   string
	token     string
	httpClient *http.Client
}

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
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, a.baseURL+"/wham/usage", nil)
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

	if result.RateLimit.PrimaryWindow != nil {
		tier := a.windowToTier(result.RateLimit.PrimaryWindow, domain.Tier5H)
		if tier != nil {
			snapshot.AddQuota(domain.Tier5H, *tier)
		}
	}

	if result.RateLimit.SecondaryWindow != nil {
		tier := a.windowToTier(result.RateLimit.SecondaryWindow, domain.Tier1W)
		if tier != nil {
			snapshot.AddQuota(domain.Tier1W, *tier)
		}
	}

	for _, addLimit := range result.AdditionalRateLimits {
		if addLimit.RateLimit == nil {
			continue
		}
		tier := a.windowToTier(addLimit.RateLimit, "")
		if tier == nil {
			continue
		}
		snapshot.AddQuota(domain.Tier1M, *tier)
	}

	return *snapshot, nil
}

func (a *Adapter) windowToTier(window *RateLimitWindow, tierHint domain.Tier) *domain.QuotaTier {
	if window == nil {
		return nil
	}

	if tierHint == "" {
		if window.LimitWindowSeconds == nil {
			return nil
		}
		switch *window.LimitWindowSeconds {
		case seconds5H:
			tierHint = domain.Tier5H
		case seconds1W:
			tierHint = domain.Tier1W
		case seconds1M:
			tierHint = domain.Tier1M
		default:
			return nil
		}
	}

	_ = tierHint // tierHint used by caller to select which tier to populate

	var used, total int64
	if window.UsedPercent != nil {
		if window.LimitWindowSeconds != nil {
			total = 100
			used = total * int64(*window.UsedPercent) / 100
		} else {
			used = 0
			total = 0
		}
	} else {
		used = 0
		total = 0
	}

	resetAt := time.Time{}
	if window.ResetAt != nil {
		resetAt = time.Unix(*window.ResetAt, 0)
	}

	return &domain.QuotaTier{
		Used:   used,
		Total:  total,
		ResetAt: resetAt,
	}
}

type APIResponse struct {
	PlanType             string                 `json:"plan_type"`
	RateLimit            RateLimit              `json:"rate_limit"`
	AdditionalRateLimits []AdditionalRateLimit  `json:"additional_rate_limits"`
	RateLimitReachedType *RateLimitReachedType `json:"rate_limit_reached_type"`
	Credits              *Credits              `json:"credits"`
}

type RateLimit struct {
	Allowed        *bool           `json:"allowed"`
	LimitReached   *bool           `json:"limit_reached"`
	PrimaryWindow   *RateLimitWindow `json:"primary_window"`
	SecondaryWindow *RateLimitWindow `json:"secondary_window"`
}

type RateLimitWindow struct {
	UsedPercent        *float64 `json:"used_percent"`
	LimitWindowSeconds *int64   `json:"limit_window_seconds"`
	ResetAfterSeconds  *int64   `json:"reset_after_seconds"`
	ResetAt            *int64   `json:"reset_at"`
}

type AdditionalRateLimit struct {
	LimitName     *string          `json:"limit_name"`
	MeteredFeature string          `json:"metered_feature"`
	RateLimit     *RateLimitWindow `json:"rate_limit"`
}

type RateLimitReachedType struct {
	Type *string `json:"type"`
}

type Credits struct {
	HasCredits *bool   `json:"has_credits"`
	Unlimited  *bool   `json:"unlimited"`
	Balance    *string `json:"balance"`
}

var _ provider.Provider = (*Adapter)(nil)