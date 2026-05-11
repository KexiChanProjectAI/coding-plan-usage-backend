package sub2api

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/quotahub/ucpqa/internal/domain"
	"github.com/quotahub/ucpqa/internal/domain/provider"
)

// Adapter fetches model quota usage from a sub2api-compatible endpoint.
type Adapter struct {
	name       string
	baseURL    string
	token      string
	httpClient *http.Client
}

// New creates a new sub2api provider adapter.
func New(name, baseURL, token string) *Adapter {
	return &Adapter{
		name:    name,
		baseURL: baseURL,
		token:   token,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// NewWithClient creates a new sub2api provider adapter with a custom HTTP client.
func NewWithClient(name, baseURL, token string, client *http.Client) *Adapter {
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
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, a.baseURL+"/v1/model-quotas", nil)
	if err != nil {
		return domain.AccountSnapshot{}, provider.NewErrFetchFailure(a.name, err)
	}

	req.Header.Set("Authorization", "Bearer "+a.token)

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return domain.AccountSnapshot{}, provider.NewErrFetchFailure(a.name, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return domain.AccountSnapshot{}, provider.NewErrParseFailure(a.name, err)
	}

	if resp.StatusCode != http.StatusOK {
		var upstreamErr ErrorResponse
		if err := json.Unmarshal(body, &upstreamErr); err == nil && upstreamErr.Error.Message != "" {
			return domain.AccountSnapshot{}, provider.NewErrUpstreamRejection(a.name, resp.StatusCode, upstreamErr.Error.Message, nil)
		}
		return domain.AccountSnapshot{}, provider.NewErrUpstreamRejection(a.name, resp.StatusCode, "", nil)
	}

	var result APIResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return domain.AccountSnapshot{}, provider.NewErrParseFailure(a.name, err)
	}

	snapshot := domain.NewAccountSnapshot(a.name, "default", 0)

	var highest5H int64
	var highest1W int64
	var has5H bool
	var has1W bool

	// sub2api reports quota windows per model, while the current domain model only
	// stores one account-level tier per window. We conservatively keep the most
	// constrained model by taking the highest usage percentage seen in each window.
	for _, model := range result.Data {
		if pct, ok := usagePercent(model.FiveHour); ok {
			if !has5H || pct > highest5H {
				highest5H = pct
			}
			has5H = true
		}

		if pct, ok := usagePercent(model.SevenDay); ok {
			if !has1W || pct > highest1W {
				highest1W = pct
			}
			has1W = true
		}
	}

	if has5H {
		snapshot.AddQuota(domain.Tier5H, domain.QuotaTier{Used: highest5H, Total: 100, ResetAt: time.Time{}})
	}
	if has1W {
		snapshot.AddQuota(domain.Tier1W, domain.QuotaTier{Used: highest1W, Total: 100, ResetAt: time.Time{}})
	}

	snapshot.Quotas = domain.BackfillCanonicalTiers(snapshot.Quotas)

	return *snapshot, nil
}

func usagePercent(window *QuotaWindow) (int64, bool) {
	if window == nil || window.TotalUSD == nil {
		return 0, false
	}

	total := *window.TotalUSD
	if total <= 0 {
		return 0, false
	}

	used := 0.0
	if window.UsedUSD != nil {
		used = *window.UsedUSD
	} else if window.RemainingUSD != nil {
		used = total - *window.RemainingUSD
	}

	if used < 0 {
		used = 0
	}
	if used > total {
		used = total
	}

	return domain.NormalizeToPercent(int64(used*10000), int64(total*10000)), true
}

type APIResponse struct {
	Object string       `json:"object"`
	Data   []ModelQuota `json:"data"`
}

type ModelQuota struct {
	ID        string       `json:"id"`
	Object    string       `json:"object"`
	QuotaPool string       `json:"quota_pool"`
	FiveHour  *QuotaWindow `json:"five_hour"`
	SevenDay  *QuotaWindow `json:"seven_day"`
}

type QuotaWindow struct {
	TotalUSD             *float64 `json:"total_usd"`
	UsedUSD              *float64 `json:"used_usd"`
	RemainingUSD         *float64 `json:"remaining_usd"`
	AccountsCount        *int     `json:"accounts_count"`
	UnknownAccountsCount *int     `json:"unknown_accounts_count"`
}

type ErrorResponse struct {
	Error ErrorBody `json:"error"`
}

type ErrorBody struct {
	Type    string `json:"type"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

var _ provider.Provider = (*Adapter)(nil)
