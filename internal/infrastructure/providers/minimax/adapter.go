// Package minimax provides the MiniMax provider adapter.
package minimax

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/quotahub/ucpqa/internal/domain"
	"github.com/quotahub/ucpqa/internal/domain/provider"
)

// Adapter is the MiniMax provider adapter.
type Adapter struct {
	name      string
	baseURL   string
	token     string
	httpClient *http.Client
}

// New creates a new MiniMax provider adapter.
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

// NewWithClient creates a new MiniMax provider adapter with a custom HTTP client.
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
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, a.baseURL+"/v1/api/openplatform/coding_plan/remains", nil)
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

	if result.BaseResp.StatusCode != 0 {
		return domain.AccountSnapshot{}, provider.NewErrUpstreamRejection(a.name, result.BaseResp.StatusCode, result.BaseResp.StatusMsg, nil)
	}

	snapshot := domain.NewAccountSnapshot(a.name, "default", 0)

	var highest5H, highest1W int64
	var resetAt5H, resetAt1W time.Time

	for _, model := range result.ModelRemains {
		if model.CurrentIntervalTotalCount > 0 {
			used5H := int64(model.CurrentIntervalTotalCount - model.CurrentIntervalUsageCount)
			if used5H < 0 {
				used5H = 0
			}
			percent5H := domain.NormalizeToPercent(used5H, int64(model.CurrentIntervalTotalCount))
			if percent5H > highest5H {
				highest5H = percent5H
			}
			// Preserve ResetAt from any valid tier, even if usage is 0%
			if resetAt5H.IsZero() {
				resetAt5H = time.UnixMilli(model.EndTime)
			}
		}

		if model.CurrentWeeklyTotalCount > 0 {
			used1W := int64(model.CurrentWeeklyTotalCount - model.CurrentWeeklyUsageCount)
			if used1W < 0 {
				used1W = 0
			}
			percent1W := domain.NormalizeToPercent(used1W, int64(model.CurrentWeeklyTotalCount))
			if percent1W > highest1W {
				highest1W = percent1W
			}
			// Preserve ResetAt from any valid tier, even if usage is 0%
			if resetAt1W.IsZero() {
				resetAt1W = time.UnixMilli(model.WeeklyEndTime)
			}
		}
	}

	if highest5H > 0 || !resetAt5H.IsZero() {
		snapshot.AddQuota(domain.Tier5H, domain.QuotaTier{
			Used:    highest5H,
			Total:   100,
			ResetAt: resetAt5H,
		})
	}
	if highest1W > 0 || !resetAt1W.IsZero() {
		snapshot.AddQuota(domain.Tier1W, domain.QuotaTier{
			Used:    highest1W,
			Total:   100,
			ResetAt: resetAt1W,
		})
	}

	snapshot.Quotas = domain.BackfillCanonicalTiers(snapshot.Quotas)

	return *snapshot, nil
}

// APIResponse represents the MiniMax API response structure.
type APIResponse struct {
	ModelRemains []ModelRemain `json:"model_remains"`
	BaseResp     BaseResp      `json:"base_resp"`
}

// ModelRemain represents a single model's quota information.
type ModelRemain struct {
	StartTime                int64  `json:"start_time"`
	EndTime                  int64  `json:"end_time"`
	RemainsTime              int64  `json:"remains_time"`
	CurrentIntervalTotalCount int   `json:"current_interval_total_count"`
	CurrentIntervalUsageCount int   `json:"current_interval_usage_count"`
	ModelName                string `json:"model_name"`
	CurrentWeeklyTotalCount   int   `json:"current_weekly_total_count"`
	CurrentWeeklyUsageCount   int   `json:"current_weekly_usage_count"`
	WeeklyStartTime          int64  `json:"weekly_start_time"`
	WeeklyEndTime            int64  `json:"weekly_end_time"`
	WeeklyRemainsTime        int64  `json:"weekly_remains_time"`
}

// BaseResp represents the base response envelope.
type BaseResp struct {
	StatusCode int    `json:"status_code"`
	StatusMsg  string `json:"status_msg"`
}

// Compile-time check that Adapter implements provider.Provider interface.
var _ provider.Provider = (*Adapter)(nil)
