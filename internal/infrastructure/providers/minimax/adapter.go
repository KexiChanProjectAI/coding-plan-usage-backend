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

	for _, model := range result.ModelRemains {
		intervalTier := domain.QuotaTier{
			Used:   int64(model.CurrentIntervalTotalCount - model.CurrentIntervalUsageCount),
			Total:  int64(model.CurrentIntervalTotalCount),
			ResetAt: time.UnixMilli(model.EndTime),
		}
		snapshot.AddQuota(domain.Tier5H, intervalTier)

		if model.CurrentWeeklyTotalCount > 0 {
			weeklyTier := domain.QuotaTier{
				Used:   int64(model.CurrentWeeklyTotalCount - model.CurrentWeeklyUsageCount),
				Total:  int64(model.CurrentWeeklyTotalCount),
				ResetAt: time.UnixMilli(model.WeeklyEndTime),
			}
			snapshot.AddQuota(domain.Tier1W, weeklyTier)
		}
	}

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
