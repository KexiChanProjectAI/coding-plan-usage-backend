# QuotaHub

Real-time aggregation of coding plan quota data from multiple AI providers into a unified API.

## Features

- Multi-provider polling (OpenAI Codex, Kimi, MiniMax, Z.ai, Zhipu)
- Canonical normalization to 3 tiers: 5H, 1W, 1M
- Real-time updates via SSE and WebSocket
- Derived account status (healthy / degraded / initializing)
- Prometheus business and HTTP metrics
- Jitter and exponential backoff for provider polling
- Graceful shutdown with configurable timeout
- Thread-safe in-memory store (no database dependency)

## Architecture

```
cmd/api/main.go          → Entry point, config loading, provider construction
internal/
  app/                   → Composition root, wires all components
  config/                → YAML config + env var overrides
  domain/                 → Core types: AccountSnapshot, Tier, Status, DeriveStatus, Provider interface
  infrastructure/
    store/                → Thread-safe versioned in-memory store
    metrics/              → Prometheus gauges + HTTP middleware
    providers/
      codex/              → OpenAI Codex WHAM usage adapter
      kimi/               → Kimi coding plan adapter
      minimax/            → MiniMax coding plan adapter
      monitorquota/       → Shared adapter for Z.ai and Zhipu
  runtime/
    syncmanager/          → Per-provider polling with jitter, backoff, refresh coalescing
  transport/
    http/api/             → REST usage handler
    sse/                  → SSE broker + handler
    ws/                   → WebSocket hub + handler
```

## Quick Start

```bash
# Build
go build -o ucpqa ./cmd/api

# Configure (see Configuration section)
cp configs/config.yaml config.yaml
# Edit config.yaml with your provider tokens

# Run
./ucpqa
# Starts API server on :8080, metrics on :9090
```

## Configuration

Reference `configs/config.yaml` for a full example. Every provider requires a valid token to start.

```yaml
server:
  api_port: 8080        # API + SSE + WS server port
  metrics_port: 9090    # Prometheus metrics server port

global:
  max_stale_duration: 10m  # Tiers with reset_at older than this are omitted

providers:
  codex:
    type: codex              # Provider adapter type (codex, kimi, minimax, zai, zhipu)
    name: codex              # Unique alias for this account
    base_url: "https://api.codex.example.com/v1"
    token: ""                    # REQUIRED — Bearer token
    refresh_interval: 5m         # Poll interval
    jitter_percent: 15           # ±15% random jitter on interval
    backoff_initial: 1s          # Initial backoff on fetch failure
    backoff_max: 60s             # Maximum backoff duration
  kimi:
    type: kimi
    name: kimi
    base_url: "https://api.moonshot.cn/v1"
    token: ""
    refresh_interval: 5m
    jitter_percent: 10
    backoff_initial: 2s
    backoff_max: 120s
  minimax-work:
    type: minimax
    name: minimax-work
    base_url: "https://api.minimax.chat/v1"
    token: ""
    refresh_interval: 5m
    jitter_percent: 10
    backoff_initial: 1s
    backoff_max: 60s
  minimax-personal:
    type: minimax
    name: minimax-personal
    base_url: "https://api.minimax.chat/v1"
    token: ""
    refresh_interval: 5m
    jitter_percent: 10
    backoff_initial: 1s
    backoff_max: 60s
  zai:
    type: zai
    name: zai
    base_url: "https://your-zai-instance.com"
    token: ""
    refresh_interval: 5m
    jitter_percent: 10
    backoff_initial: 1s
    backoff_max: 60s
  zhipu:
    type: zhipu
    name: zhipu
    base_url: "https://your-zhipu-instance.com"
    token: ""
    refresh_interval: 5m
    jitter_percent: 10
    backoff_initial: 1s
    backoff_max: 60s
```

### Provider Configuration Fields

- `type` (required) — Provider adapter type: `codex`, `kimi`, `minimax`, `zai`, or `zhipu`. This determines which adapter is used.
- `name` — Human-readable alias for this account. Defaults to the config key if not specified.
- `token` — REQUIRED. Bearer token for the provider API.
- `max_stale_duration` — Tiers whose `reset_at` is more than this duration in the past are omitted from snapshots
- Config path defaults to `config.yaml`, override with `UCPQA_CONFIG_PATH` env var

### Multi-Account Support

You can configure multiple accounts of the same provider type by using different config keys and the same `type` value:

```yaml
providers:
  minimax-work:
    type: minimax
    name: minimax-work
    base_url: "https://www.minimaxi.com/"
    token: ""                    # REQUIRED
    refresh_interval: 5m
    jitter_percent: 10
    backoff_initial: 1s
    backoff_max: 60s

  minimax-personal:
    type: minimax                # Same type as minimax-work
    name: minimax-personal
    base_url: "https://www.minimaxi.com/"
    token: ""                    # REQUIRED
    refresh_interval: 5m
    jitter_percent: 10
    backoff_initial: 1s
    backoff_max: 60s
```

The config key (`minimax-work`, `minimax-personal`) is the unique identifier/alias. The `type` field determines which adapter is instantiated. Each account appears separately in the API response with its own platform name.

## Environment Variable Overrides

All configuration values can be overridden via environment variables:

- `UCPQA_CONFIG_PATH` — Config file path
- `UCPQA_API_PORT` — Override server.api_port
- `UCPQA_METRICS_PORT` — Override server.metrics_port
- `UCPQA_MAX_STALE_DURATION` — Override global.max_stale_duration (Go duration format, e.g. "10m")
- `UCPQA_PROVIDER_{NAME}_BASE_URL` — Override provider base_url
- `UCPQA_PROVIDER_{NAME}_TOKEN` — Override provider token
- `UCPQA_PROVIDER_{NAME}_REFRESH_INTERVAL` — Override provider refresh_interval
- `UCPQA_PROVIDER_{NAME}_JITTER_PERCENT` — Override provider jitter_percent
- `UCPQA_PROVIDER_{NAME}_BACKOFF_INITIAL` — Override provider backoff_initial
- `UCPQA_PROVIDER_{NAME}_BACKOFF_MAX` — Override provider backoff_max

{Name} is the provider key uppercased (e.g. CODEX, KIMI, ZAI, ZHIPU).

## API Reference

### GET /api/v1/usage

Returns array of all account snapshots with derived status.

```json
[
  {
    "platform": "codex",
    "account_alias": "",
    "quotas": {
      "5H": { "used": 42, "total": 100, "reset_at": "2025-04-28T12:00:00Z" },
      "1W": { "used": 300, "total": 1000, "reset_at": "2025-05-02T00:00:00Z" },
      "1M": { "used": 1200, "total": 5000, "reset_at": "2025-05-28T00:00:00Z" }
    },
    "last_sync": "2025-04-28T10:30:00Z",
    "version": 42,
    "status": "healthy"
  }
]
```

Status values:
- `healthy` — all tiers fresh
- `degraded` — any tier stale beyond max_stale_duration
- `initializing` — no data yet (version=0)

Empty store returns:
```json
[{ "platform": "", "version": 0, "status": "initializing" }]
```

### GET /api/v1/stream (SSE)

Server-Sent Events stream. Each event wraps a versioned snapshot:

```
data: {"version":42,"snapshot":{"platform":"codex","account_alias":"","quotas":{"5H":{"used":42,"total":100,"reset_at":"..."}},...}}
```

- Content-Type: text/event-stream
- Auto-reconnect: client should reconnect on disconnect
- Version field enables client-side diff detection

### GET /ws (WebSocket)

WebSocket endpoint for real-time updates.

- Server broadcasts raw `AccountSnapshot` JSON to all connected clients on each update
- Clients can send `{"type":"refresh"}` to trigger an immediate poll of all providers
- Ping/pong keepalive: server pings every 54s, expects pong within 60s
- Max message size from client: 512 bytes

### GET /metrics (port 9090)

Prometheus metrics endpoint.

## Metrics

- `coding_plan_usage_value{platform,account,tier,type}` — gauge, type is "used" or "total"
- `coding_plan_reset_timestamp{platform,account,tier}` — gauge, Unix timestamp of next reset
- `http_requests_total{method,path,status}` — counter
- `http_request_duration_seconds{method,path}` — histogram

## Domain Model

- **Tier**: `5H` (5-hour window), `1W` (1-week window), `1M` (1-month window) — only these 3 are supported; all other tiers are omitted
- **QuotaTier**: `{used, total, reset_at}`
- **AccountSnapshot**: `{platform, account_alias, quotas, last_sync, version}` — immutable once stored
- **Status**: derived by `DeriveStatus()` based on version and staleness
- **Provider interface**: `Fetch(ctx) → AccountSnapshot, error` + `ProviderName() → string`

## Status Derivation Rules

- `version == 0` → `initializing`
- Any supported tier's `reset_at` is older than `max_stale_duration` → `degraded`
- Otherwise → `healthy`

## Error Types

- `ErrFetchFailure` — network or transport-level error (connection timeout, DNS failure, TLS handshake error)
- `ErrParseFailure` — JSON decode or schema mismatch in provider response
- `ErrUpstreamRejection` — HTTP error status or rejection envelope from provider
- `ErrStaleExpiry` — fetched data too old to be useful

## Development

```bash
make test          # Run all tests
make test-race     # Run tests with race detector
make test-bench    # Run benchmarks
make lint          # Run golangci-lint
make build         # Build binary
make tidy          # Tidy dependencies
make verify        # Verify dependencies
```

## Graceful Shutdown

On SIGINT or SIGTERM, the app:

1. Cancels context — SyncManager stops polling
2. Closes SSE broker — closes all subscriber channels
3. Closes WebSocket hub — disconnects all clients
4. Shuts down HTTP servers with timeout (2× max_stale_duration, minimum 30s)

All Stop() methods are idempotent via sync.Once.