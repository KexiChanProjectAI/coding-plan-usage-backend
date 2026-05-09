# QuotaHub Frontend PRD

## 1. Product Overview

QuotaHub Frontend is a lightweight dashboard for operators to monitor coding-plan quota usage across configured AI providers. It presents normalized quota tiers from the backend API in a responsive, read-only interface so users can quickly identify provider health, quota consumption, and data freshness.

## 2. Goals

- Provide a single-page view of all provider quota snapshots returned by QuotaHub.
- Show quota usage for the canonical `5H`, `1W`, and `1M` tiers.
- Make provider health and stale/error states easy to scan.
- Support manual refresh and periodic automatic refresh without requiring a full page reload.
- Work on both desktop and mobile browser widths.

## 3. Non-Goals

- Editing provider configuration or credentials.
- Authenticating users or managing sessions.
- Persisting frontend state outside the browser runtime.
- Replacing API, SSE, WebSocket, or Prometheus interfaces.
- Providing historical analytics, charts, or alert configuration.

## 4. Target Users

- Developers and operators running QuotaHub locally or in an internal environment.
- Maintainers who need a quick visual check of provider quota state.
- Support/debug users validating whether configured providers are syncing successfully.

## 5. User Problems

- Raw API responses are hard to scan across multiple providers and quota windows.
- Users need to know which provider is healthy, degraded, empty, or failing at a glance.
- Users need current usage without repeatedly refreshing the browser manually.
- Mobile users need an interface that remains readable on narrow screens.

## 6. Current Product Behavior

The existing frontend is an embedded single HTML document served by the backend root route `/`. It loads React from an external ESM CDN and Babel Standalone from a CDN at runtime. The app fetches quota data from `/api/v1/usage` when hosted on the expected origin, otherwise from `http://100.64.1.38:8070/api/v1/usage`.

The UI renders:

- Page title: `Coding Plans`.
- Last updated text based on the latest successful fetch time.
- Manual refresh button with a spinning refresh icon while loading.
- Error banner with retry action when fetching fails.
- Skeleton cards during initial loading.
- Empty state when the API returns no providers.
- One provider card per snapshot.
- Provider avatar, provider name, account alias, status chip, quota blocks, last sync, and version.

## 7. Functional Requirements

### 7.1 Data Loading

- The dashboard must request quota snapshots from the backend usage API.
- The initial request must run when the page loads.
- The dashboard must refresh data automatically every 30 seconds.
- The dashboard must provide a manual refresh action.
- Manual refresh must show a visible loading/spinning state.
- Failed requests must not crash the page.
- Failed requests must display a human-readable error message and retry action.
- Empty API responses must display a dedicated empty state.

### 7.2 Provider Cards

Each provider snapshot must render as a card containing:

- Provider/platform name.
- Account alias, defaulting to `default` when absent.
- Deterministic avatar using the provider name initial and a stable color palette.
- Status chip derived from provider status.
- Quota blocks for `5H`, `1W`, and `1M`.
- Last sync relative time.
- Snapshot version.

### 7.3 Status Display

- `healthy` and `active` statuses map to `Healthy`.
- `warning` and `limited` statuses map to `Warning`.
- `critical`, `error`, and `down` statuses map to `Critical`.
- Unknown or missing statuses map to `Unknown`.
- Each status must use a visually distinct chip treatment.

### 7.4 Quota Display

For each canonical quota tier:

- Show percentage used as `used / total * 100`, clamped between 0 and 100.
- Show a progress bar matching the percentage.
- Show raw used and total values.
- Show reset time formatted as a localized short date/time.
- Show unavailable state when quota data or total quota is missing.
- Mark usage as critical at 90% or above.
- Mark usage as warning at 70% or above.
- Otherwise use the tier-specific normal progress color.

### 7.5 Responsive Layout

- The page must be readable on mobile and desktop widths.
- Provider quota blocks must collapse to a single column on small screens.
- Provider footer content must wrap or stack on small screens.
- Touch targets for refresh and retry must remain accessible on mobile.

### 7.6 Accessibility

- Provider cards must expose accessible labels.
- Status chips must expose status text to assistive technology.
- Error banners must use alert semantics.
- Loading skeletons must be hidden from assistive technology.
- Refresh actions must have accessible labels.

## 8. API Contract

The frontend expects `GET /api/v1/usage` to return an array of provider snapshots. Each item should provide the following fields used by the UI:

```json
{
  "platform": "codex",
  "account_alias": "default",
  "status": "healthy",
  "last_sync": "2026-05-09T12:00:00Z",
  "version": 1,
  "quotas": {
    "5H": { "used": 10, "total": 100, "reset_at": "2026-05-09T17:00:00Z" },
    "1W": { "used": 20, "total": 200, "reset_at": "2026-05-16T12:00:00Z" },
    "1M": { "used": 30, "total": 300, "reset_at": "2026-06-09T12:00:00Z" }
  }
}
```

## 9. UX Requirements

- Visual style should remain compact, dark-themed, and dashboard-oriented.
- Cards should prioritize scanability over dense tabular layout.
- Quota severity should be conveyed by both text and color.
- Relative freshness should be visible near the page header and per provider.
- Loading, empty, and error states should be visually distinct.

## 10. Technical Requirements

- The dashboard must be servable by the Go backend without a separate frontend build step if embedded delivery remains desired.
- If a dedicated frontend build is introduced later, the backend API contract must remain unchanged.
- Runtime dependencies loaded from third-party CDNs should be reviewed before production use.
- The API base URL must be environment-safe and should avoid hard-coded internal IP addresses in production deployments.
- The implementation should avoid storing secrets or provider tokens in the browser.

## 11. Success Metrics

- Users can identify provider health and high quota usage within 5 seconds of opening the dashboard.
- Initial dashboard render succeeds when the backend API is reachable.
- Manual refresh updates data without a full browser reload.
- The dashboard remains usable at mobile widths below 720px.
- Empty and error states are clear without reading browser console logs.

## 12. Open Questions

- Should the frontend consume SSE or WebSocket updates instead of polling every 30 seconds?
- Should the dashboard support multiple backend origins through configuration rather than source changes?
- Should historical quota usage or trend charts be added?
- Should status mapping be fully aligned with backend domain statuses only?
- Should the dashboard be split into a standalone frontend package for production deployments?
