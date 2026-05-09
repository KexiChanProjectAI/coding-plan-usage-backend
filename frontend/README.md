# QuotaHub Dashboard

A fast, accessible, read-only quota monitoring dashboard built with React, TypeScript, and Vite.

## Prerequisites

- Node.js 18+
- npm

## Quick Start

```bash
npm ci
npm run dev
```

The dev server starts at http://localhost:5173 with API requests proxied to the backend.

## Environment Variables

| Variable | Default | Description |
|---|---|---|
| `VITE_QUOTAHUB_API_URL` | (empty) | Override the API base URL. When empty, requests go to `/api/v1/usage` (same-origin or Vite proxy). |

## npm Commands

| Command | Description |
|---|---|
| `npm run dev` | Start Vite dev server with hot reload |
| `npm run build` | Type-check and build for production |
| `npm run preview` | Preview the production build locally |
| `npm run typecheck` | Run TypeScript type checking |
| `npm test` | Run Vitest unit/component tests |
| `npm test -- --run` | Run tests once (no watch) |
| `npm run test:e2e` | Run Playwright browser tests (Chromium) |

## Vite Dev Proxy

During development, the Vite dev server proxies `/api` requests to `http://100.64.1.38:8070`. This allows local development against the real backend without CORS configuration.

## Cloudflare Pages Deployment

### Build Settings

- **Root directory**: `frontend`
- **Build command**: `npm ci && npm run build`
- **Output directory**: `dist`
- **Node.js version**: 18+ (set in environment)

### Backend CORS

The frontend makes requests to `/api/v1/usage`. When deployed on Cloudflare Pages, the backend must be configured to accept requests from the Pages domain:

```yaml
# In the backend config (e.g., configs/config.yaml)
server:
  cors_enabled: true
  cors_allowed_origins:
    - "https://your-project.pages.dev"
  cors_allowed_methods:
    - "GET"
    - "OPTIONS"
  cors_allowed_headers:
    - "Content-Type"
```

Or via environment variables:

```bash
UCPQA_CORS_ENABLED=true
UCPQA_CORS_ALLOWED_ORIGINS=https://your-project.pages.dev
UCPQA_CORS_ALLOWED_METHODS=GET,OPTIONS
UCPQA_CORS_ALLOWED_HEADERS=Content-Type
```

### Private API Access

If the backend is on a private network (e.g., Tailnet IP `100.64.1.38`), Cloudflare Pages cannot reach it directly. You will need one of:

- **Cloudflare Tunnel** — Expose the backend through a public hostname
- **Cloudflare Workers** — Proxy API requests from the Pages domain
- **VPN/Zero Trust** — Network-level access for the backend

**Note**: Implementation of these networking solutions is out of scope for this project. This document only describes the prerequisites.

## Security

- **No secrets**: The frontend does not store or transmit API keys, provider tokens, or credentials.
- **Public env var**: Only `VITE_QUOTAHUB_API_URL` is used, which is embedded at build time and contains no sensitive data.
- **Read-only**: The dashboard only reads quota data; it cannot modify provider configurations.
