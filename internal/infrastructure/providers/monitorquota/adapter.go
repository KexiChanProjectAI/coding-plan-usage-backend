// Package monitorquota provides a shared adapter for Z.ai and Zhipu MonitorQuota endpoints.
package monitorquota

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/quotahub/ucpqa/internal/domain"
	"github.com/quotahub/ucpqa/internal/domain/provider"
)

// Adapter is the shared MonitorQuota adapter for Z.ai and Zhipu.
type Adapter struct {
	name       string
	baseURL    string
	token      string
	httpClient *http.Client
}

// NewZAI creates a new Z.ai MonitorQuota provider adapter.
func NewZAI(name, baseURL, token string) *Adapter {
	return &Adapter{
		name:       name,
		baseURL:    baseURL,
		token:      token,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// NewZhipu creates a new Zhipu MonitorQuota provider adapter.
func NewZhipu(name, baseURL, token string) *Adapter {
	return &Adapter{
		name:       name,
		baseURL:    baseURL,
		token:      token,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// NewZAIWithClient creates a new Z.ai MonitorQuota provider adapter with a custom HTTP client.
func NewZAIWithClient(name, baseURL, token string, client *http.Client) *Adapter {
	return &Adapter{
		name:       name,
		baseURL:    baseURL,
		token:      token,
		httpClient: client,
	}
}

// NewZhipuWithClient creates a new Zhipu MonitorQuota provider adapter with a custom HTTP client.
func NewZhipuWithClient(name, baseURL, token string, client *http.Client) *Adapter {
	return &Adapter{
		name:       name,
		baseURL:    baseURL,
		token:      token,
		httpClient: client,
	}
}

// ProviderName returns the provider name.
func (a *Adapter) ProviderName() string {
	return a.name
}

// Fetch retrieves the current account quota snapshot from the upstream provider.
func (a *Adapter) Fetch(ctx context.Context) (domain.AccountSnapshot, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, a.baseURL+"/api/monitor/usage/quota/limit", nil)
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

	// Validate success envelope: success==true && code==200
	if !result.Success || result.Code != 200 {
		return domain.AccountSnapshot{}, provider.NewErrUpstreamRejection(a.name, result.Code, result.Msg, nil)
	}

	snapshot := domain.NewAccountSnapshot(a.name, "default", 0)

	for _, limit := range result.Data.Limits {
		tier := mapUnitToTier(limit.Unit)
		if tier == "" {
			// Unsupported tier, skip silently
			continue
		}

		var used int64
		if limit.Usage != nil {
			// Use usage field directly if present
			used = *limit.Usage
		} else if limit.CurrentValue != nil && limit.Remaining != nil {
			// Compute used from currentValue - remaining
			used = *limit.CurrentValue - *limit.Remaining
		} else {
			// Cannot compute used without either usage or (currentValue and remaining)
			continue
		}

		var total int64
		if limit.CurrentValue != nil {
			total = *limit.CurrentValue
		} else {
			continue
		}

		var resetAt time.Time
		if limit.NextResetTime != nil {
			resetAt = time.UnixMilli(*limit.NextResetTime)
		}

		snapshot.AddQuota(tier, domain.QuotaTier{
			Used:   used,
			Total:  total,
			ResetAt: resetAt,
		})
	}

	return *snapshot, nil
}

// mapUnitToTier maps the Z.ai/Zhipu unit code to a domain tier.
// unit=3 maps to 5H (hour), unit=5 maps to 1M (month).
// Returns empty string for unsupported units.
func mapUnitToTier(unit *int) domain.Tier {
	if unit == nil {
		return ""
	}
	switch *unit {
	case 3:
		return domain.Tier5H
	case 5:
		return domain.Tier1M
	default:
		return ""
	}
}

// APIResponse represents the Z.ai/Zhipu API response envelope.
type APIResponse struct {
	Code    int         `json:"code"`
	Msg     string      `json:"msg"`
	Data    ResponseData `json:"data"`
	Success bool        `json:"success"`
}

// ResponseData represents the data field of the quota limit response.
type ResponseData struct {
	Limits []QuotaLimit `json:"limits"`
	Level  string       `json:"level"`
}

// QuotaLimit represents a single quota limit entry.
type QuotaLimit struct {
	Type          string   `json:"type"`
	Unit          *int     `json:"unit"`
	Number        *int     `json:"number"`
	Usage         *int64   `json:"usage"`
	CurrentValue  *int64   `json:"currentValue"`
	Remaining     *int64   `json:"remaining"`
	Percentage    *int     `json:"percentage"`
	NextResetTime *int64   `json:"nextResetTime"`
	UsageDetails  []UsageDetail `json:"usageDetails"`
}

// UsageDetail represents a single usage detail entry.
type UsageDetail struct {
	ModelCode string `json:"modelCode"`
	Usage     int64  `json:"usage"`
}

// Compile-time check that Adapter implements provider.Provider interface.
var _ provider.Provider = (*Adapter)(nil)