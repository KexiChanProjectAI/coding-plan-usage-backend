// Package cpa provides the CLIProxyAPI Codex Pool Aggregator (CPA) provider adapter.
//
// The adapter talks to a running CLIProxyAPI management API to discover OpenAI Codex
// OAuth accounts, queries each account's upstream usage endpoint through the relay,
// and returns a single pooled snapshot whose remaining quota is the equal-weighted
// average remaining percentage of all successfully queried accounts.
package cpa

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/quotahub/ucpqa/internal/domain"
	"github.com/quotahub/ucpqa/internal/domain/provider"
)

const (
	// defaultStagger is the default sleep between consecutive per-account usage queries.
	defaultStagger = 1 * time.Second

	// upstreamUsageURL is the OpenAI endpoint relayed through CLIProxyAPI's api-call.
	upstreamUsageURL = "https://chatgpt.com/backend-api/wham/usage"
)

// Adapter is the CPA provider adapter.
type Adapter struct {
	name       string
	baseURL    string
	token      string
	stagger    time.Duration
	httpClient *http.Client
}

// New creates a new CPA provider adapter with the default request stagger.
func New(name, baseURL, token string) *Adapter {
	return &Adapter{
		name:    name,
		baseURL: baseURL,
		token:   token,
		stagger: defaultStagger,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// NewWithStagger creates a new CPA provider adapter with a custom per-account stagger.
// A stagger of zero or negative falls back to the default stagger.
func NewWithStagger(name, baseURL, token string, stagger time.Duration) *Adapter {
	a := New(name, baseURL, token)
	if stagger > 0 {
		a.stagger = stagger
	}
	return a
}

// NewWithClient creates a new CPA provider adapter with a custom HTTP client.
func NewWithClient(name, baseURL, token string, client *http.Client) *Adapter {
	return &Adapter{
		name:       name,
		baseURL:    baseURL,
		token:      token,
		stagger:    defaultStagger,
		httpClient: client,
	}
}

func (a *Adapter) ProviderName() string {
	return a.name
}

// Fetch retrieves the current pooled quota snapshot for all OpenAI Codex accounts.
// It lists accounts via the management API, queries each Codex account through the
// relay with staggered timing, and returns the equal-weighted average remaining
// percentage of the successfully queried accounts.
func (a *Adapter) Fetch(ctx context.Context) (domain.AccountSnapshot, error) {
	accounts, err := a.listCodexAccounts(ctx)
	if err != nil {
		return domain.AccountSnapshot{}, err
	}
	if len(accounts) == 0 {
		return domain.AccountSnapshot{}, provider.NewErrFetchFailure(a.name, fmt.Errorf("no queryable OpenAI Codex accounts found"))
	}

	var successful []int
	for i, acc := range accounts {
		if i > 0 {
			if err := sleepCtx(ctx, a.stagger); err != nil {
				return domain.AccountSnapshot{}, provider.NewErrFetchFailure(a.name, err)
			}
		}

		remaining, err := a.queryAccountRemaining(ctx, acc)
		if err != nil {
			log.Printf("[cpa] %s: skipping account %q: %v", a.name, acc.Label, err)
			continue
		}
		successful = append(successful, remaining)
	}

	if len(successful) == 0 {
		return domain.AccountSnapshot{}, provider.NewErrFetchFailure(a.name, fmt.Errorf("all %d Codex account queries failed", len(accounts)))
	}

	sum := 0
	for _, v := range successful {
		sum += v
	}
	avgRemaining := sum / len(successful)

	snapshot := domain.NewAccountSnapshot(a.name, "openai-pool", 0)
	now := time.Now()
	used := int64(100 - avgRemaining)
	for _, tier := range domain.AllSupportedTiers {
		snapshot.AddQuota(tier, domain.QuotaTier{
			Used:    used,
			Total:   100,
			ResetAt: now,
		})
	}

	return *snapshot, nil
}

// codexAccount holds the fields needed to query a single Codex account.
type codexAccount struct {
	AuthIndex        string
	ChatGPTAccountID string
	Label            string
}

// listCodexAccounts calls the management /auth-files endpoint and returns only
// queryable OpenAI Codex accounts.
func (a *Adapter) listCodexAccounts(ctx context.Context) ([]codexAccount, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, a.baseURL+"/auth-files", nil)
	if err != nil {
		return nil, provider.NewErrFetchFailure(a.name, err)
	}
	req.Header.Set("Authorization", "Bearer "+a.token)

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return nil, provider.NewErrFetchFailure(a.name, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, provider.NewErrUpstreamRejection(a.name, resp.StatusCode, "auth-files request failed", nil)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, provider.NewErrParseFailure(a.name, err)
	}

	var result authFilesResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, provider.NewErrParseFailure(a.name, err)
	}

	var accounts []codexAccount
	for _, f := range result.Files {
		if f.Type != "codex" {
			continue
		}
		if f.Disabled || f.Unavailable || f.Status != "active" {
			continue
		}
		if f.AuthIndex == "" {
			log.Printf("[cpa] %s: skipping account %q: missing auth_index", a.name, accountLabel(f))
			continue
		}
		if f.IDToken.ChatGPTAccountID == "" {
			log.Printf("[cpa] %s: skipping account %q: missing chatgpt_account_id", a.name, accountLabel(f))
			continue
		}
		accounts = append(accounts, codexAccount{
			AuthIndex:        f.AuthIndex,
			ChatGPTAccountID: f.IDToken.ChatGPTAccountID,
			Label:            accountLabel(f),
		})
	}

	return accounts, nil
}

// queryAccountRemaining queries a single Codex account through the api-call relay
// and returns its effective remaining percentage (0-100).
func (a *Adapter) queryAccountRemaining(ctx context.Context, acc codexAccount) (int, error) {
	payload := apiCallRequest{
		AuthIndex: acc.AuthIndex,
		Method:    "GET",
		URL:       upstreamUsageURL,
		Header: map[string]string{
			"Authorization":      "Bearer $TOKEN$",
			"ChatGPT-Account-Id": acc.ChatGPTAccountID,
			"Accept":             "application/json",
			"User-Agent":         "codex_cli_rs/0.101.0 (Mac OS 26.0.1; arm64)",
		},
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return 0, provider.NewErrParseFailure(a.name, err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.baseURL+"/api-call", bytes.NewReader(payloadBytes))
	if err != nil {
		return 0, provider.NewErrFetchFailure(a.name, err)
	}
	req.Header.Set("Authorization", "Bearer "+a.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return 0, provider.NewErrFetchFailure(a.name, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return 0, provider.NewErrUpstreamRejection(a.name, resp.StatusCode, string(body), nil)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, provider.NewErrParseFailure(a.name, err)
	}

	var envelope apiCallResponse
	if err := json.Unmarshal(body, &envelope); err != nil {
		return 0, provider.NewErrParseFailure(a.name, err)
	}

	if envelope.StatusCode == 0 {
		// The relay always sets status_code when it returns HTTP 200. If it is missing,
		// the relay response is malformed.
		return 0, provider.NewErrParseFailure(a.name, fmt.Errorf("relay response missing status_code"))
	}

	if envelope.StatusCode != http.StatusOK {
		msg := envelope.Body
		if len(msg) > 200 {
			msg = msg[:200]
		}
		return 0, provider.NewErrUpstreamRejection(a.name, envelope.StatusCode, msg, nil)
	}

	var usage usageResponse
	if err := json.Unmarshal([]byte(envelope.Body), &usage); err != nil {
		return 0, provider.NewErrParseFailure(a.name, err)
	}

	return effectiveRemaining(&usage), nil
}

// effectiveRemaining computes the effective remaining percentage (0-100) from an
// upstream usage response following the parsing rules in the CLIProxyAPI docs.
func effectiveRemaining(u *usageResponse) int {
	if u == nil || u.RateLimit == nil {
		// No rate-limit info means unmetered or credits-only; treat as full remaining.
		return 100
	}

	rl := u.RateLimit
	maxUsed := 0
	hasWindow := false

	for _, w := range []*window{rl.Primary, rl.Secondary} {
		if w == nil {
			continue
		}
		hasWindow = true
		used := clampPercent(w.UsedPercent)
		if used > maxUsed {
			maxUsed = used
		}
	}

	for _, extra := range u.AdditionalRateLimits {
		if extra.RateLimit == nil {
			continue
		}
		for _, w := range []*window{extra.RateLimit.Primary, extra.RateLimit.Secondary} {
			if w == nil {
				continue
			}
			hasWindow = true
			used := clampPercent(w.UsedPercent)
			if used > maxUsed {
				maxUsed = used
			}
		}
	}

	blocked := !rl.Allowed || rl.LimitReached
	if u.Credits != nil && (u.Credits.HasCredits || u.Credits.Unlimited) {
		blocked = false
	}
	if u.SpendControl != nil && u.SpendControl.Reached {
		blocked = true
	}

	if blocked {
		return 0
	}
	if !hasWindow {
		return 100
	}
	return 100 - maxUsed
}

// clampPercent clamps a percentage value to [0, 100].
func clampPercent(v int) int {
	if v < 0 {
		return 0
	}
	if v > 100 {
		return 100
	}
	return v
}

// sleepCtx sleeps for d or returns ctx.Err if the context is cancelled first.
func sleepCtx(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

// authFilesResponse is the response shape from GET /v0/management/auth-files.
type authFilesResponse struct {
	Files []authFile `json:"files"`
}

// authFile is a single entry in the auth-files list.
type authFile struct {
	AuthIndex   string  `json:"auth_index"`
	Name        string  `json:"name"`
	Type        string  `json:"type"`
	Label       string  `json:"label"`
	Email       string  `json:"email"`
	Status      string  `json:"status"`
	Disabled    bool    `json:"disabled"`
	Unavailable bool    `json:"unavailable"`
	IDToken     idToken `json:"id_token"`
}

// accountLabel returns a human-readable label for an auth file entry.
func accountLabel(f authFile) string {
	if f.Label != "" {
		return f.Label
	}
	if f.Email != "" {
		return f.Email
	}
	return f.Name
}

// idToken holds the parsed JWT id_token claims for a Codex account.
type idToken struct {
	ChatGPTAccountID string `json:"chatgpt_account_id"`
	PlanType         string `json:"plan_type"`
}

// apiCallRequest is the body for POST /v0/management/api-call.
type apiCallRequest struct {
	AuthIndex string            `json:"auth_index"`
	Method    string            `json:"method"`
	URL       string            `json:"url"`
	Header    map[string]string `json:"header"`
}

// apiCallResponse is the relay envelope returned by api-call.
type apiCallResponse struct {
	StatusCode int    `json:"status_code"`
	Body       string `json:"body"`
}

// usageResponse mirrors the upstream OpenAI usage endpoint payload.
type usageResponse struct {
	PlanType             string                `json:"plan_type"`
	RateLimit            *rateLimit            `json:"rate_limit"`
	Credits              *credits              `json:"credits"`
	AdditionalRateLimits []additionalRateLimit `json:"additional_rate_limits"`
	SpendControl         *spendControl         `json:"spend_control"`
}

// rateLimit is the top-level or nested rate limit object.
type rateLimit struct {
	Allowed      bool    `json:"allowed"`
	LimitReached bool    `json:"limit_reached"`
	Primary      *window `json:"primary_window"`
	Secondary    *window `json:"secondary_window"`
}

// window is a single quota window.
type window struct {
	UsedPercent        int `json:"used_percent"`
	LimitWindowSeconds int `json:"limit_window_seconds"`
	ResetAfterSeconds  int `json:"reset_after_seconds"`
	ResetAt            int `json:"reset_at"`
}

// additionalRateLimit is an extra limit set keyed by limit_name.
type additionalRateLimit struct {
	LimitName string     `json:"limit_name"`
	RateLimit *rateLimit `json:"rate_limit"`
}

// credits represents purchased/rollover credits.
type credits struct {
	HasCredits bool `json:"has_credits"`
	Unlimited  bool `json:"unlimited"`
}

// spendControl represents workspace spend caps.
type spendControl struct {
	Reached bool `json:"reached"`
}

// Compile-time check that Adapter implements provider.Provider interface.
var _ provider.Provider = (*Adapter)(nil)
