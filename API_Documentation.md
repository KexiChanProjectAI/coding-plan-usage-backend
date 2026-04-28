# QuotaHub API Documentation

This document describes the API request/response formats used by QuotaHub to fetch usage/quota data from each supported provider.

Providers covered:
- [OpenAI Codex](#openai-codex)
- [Kimi Coding Plan](#kimi-coding-plan)
- [MiniMax Coding Plan](#minimax-coding-plan)
- [Z.ai](#zai)
- [Zhipu BigModel](#zhipu-bigmodel)

---

## OpenAI Codex

### Base URL
```
https://chatgpt.com/backend-api/
```

### Endpoint
```
GET wham/usage
```

### Request Headers
| Header | Required | Description |
|--------|----------|-------------|
| `Authorization` | Yes | Bearer token or session authorization |
| `ChatGPT-Account-Id` | Optional | Account identifier |

### Response Body (JSON)
```json
{
  "plan_type": "string | null",
  "rate_limit": {
    "allowed": "boolean | null",
    "limit_reached": "boolean | null",
    "primary_window": {
      "used_percent": "number | null",
      "limit_window_seconds": "long | null",
      "reset_after_seconds": "long | null",
      "reset_at": "long (seconds) | null"
    },
    "secondary_window": {
      "used_percent": "number | null",
      "limit_window_seconds": "long | null",
      "reset_after_seconds": "long | null",
      "reset_at": "long (seconds) | null"
    }
  },
  "additional_rate_limits": [
    {
      "limit_name": "string | null",
      "metered_feature": "string",
      "rate_limit": { /* same shape as rate_limit above */ }
    }
  ],
  "rate_limit_reached_type": {
    "type": "string | null"
  },
  "credits": {
    "has_credits": "boolean | null",
    "unlimited": "boolean | null",
    "balance": "string | null"
  }
}
```

### Notes
- `reset_at` is in **seconds** (not milliseconds).
- `used_percent` is a 0–100 percentage.
- `additional_rate_limits` contains per-feature rate limits.

---

## Kimi Coding Plan

### Base URL
```
https://api.kimi.com/
```

### Endpoint
```
GET coding/v1/usages
```

### Request Headers
| Header | Required | Description |
|--------|----------|-------------|
| `Authorization` | Yes | Bearer token |

### Response Body (JSON)
```json
{
  "user": {
    "userId": "string | null",
    "region": "string | null",
    "membership": {
      "level": "string | null"
    },
    "businessId": "string | null"
  },
  "usage": {
    "limit": "string (numeric)",
    "remaining": "string (numeric)",
    "resetTime": "string (ISO-8601 instant)"
  },
  "limits": [
    {
      "window": {
        "duration": "long",
        "timeUnit": "string (e.g. TIME_UNIT_DAY, TIME_UNIT_MONTH)"
      },
      "detail": {
        "limit": "string (numeric)",
        "remaining": "string (numeric)",
        "resetTime": "string (ISO-8601 instant)"
      }
    }
  ],
  "parallel": {
    "limit": "string (numeric)"
  },
  "totalQuota": {
    "limit": "string (numeric)",
    "remaining": "string (numeric)"
  },
  "authentication": {
    "method": "string | null",
    "scope": "string | null"
  },
  "subType": "string | null"
}
```

### Notes
- `usage.limit`, `usage.remaining`, and parallel/limit values are **strings** representing numbers.
- `resetTime` uses ISO-8601 format (e.g. `2025-04-27T12:00:00Z`).
- `timeUnit` values observed: `TIME_UNIT_MINUTE`, `TIME_UNIT_HOUR`, `TIME_UNIT_DAY`, `TIME_UNIT_MONTH`.

---

## MiniMax Coding Plan

### Base URL
```
https://www.minimaxi.com/
```

### Endpoint
```
GET v1/api/openplatform/coding_plan/remains
```

### Request Headers
| Header | Required | Description |
|--------|----------|-------------|
| `Authorization` | Yes | Bearer token |

### Response Body (JSON)
```json
{
  "model_remains": [
    {
      "start_time": "long (epoch ms)",
      "end_time": "long (epoch ms)",
      "remains_time": "long (epoch ms)",
      "current_interval_total_count": "int",
      "current_interval_usage_count": "int",
      "model_name": "string",
      "current_weekly_total_count": "int",
      "current_weekly_usage_count": "int",
      "weekly_start_time": "long (epoch ms)",
      "weekly_end_time": "long (epoch ms)",
      "weekly_remains_time": "long (epoch ms)"
    }
  ],
  "base_resp": {
    "status_code": "int (0 = success)",
    "status_msg": "string"
  }
}
```

### Notes
- `base_resp.status_code` must be `0` for success.
- `current_interval_usage_count` actually represents the **remaining** count (naming quirk in the API).
- `model_name` examples: `MiniMax-M*`, `coding-plan-vlm`, `coding-plan-search`.
- Weekly fields may be `0` when there is no weekly quota for that model.

---

## Z.ai

### Base URL
Dynamic / configurable; default is the Z.ai console domain (e.g. `https://<tenant>.z.ai/`).

### Endpoints

#### 1. Model Usage
```
GET api/monitor/usage/model-usage
```

#### 2. Tool Usage
```
GET api/monitor/usage/tool-usage
```

#### 3. Quota Limit
```
GET api/monitor/usage/quota/limit
```

### Request Headers
| Header | Required | Description |
|--------|----------|-------------|
| `Authorization` | Yes | Bearer token |
| `Accept-Language` | No | `en-US,en` (sent by client) |
| `Content-Type` | No | `application/json` (sent by client) |

### Query Parameters (model-usage & tool-usage)
| Parameter | Required | Description |
|-----------|----------|-------------|
| `startTime` | Yes | Start of window, format `yyyy-MM-dd HH:mm:ss` |
| `endTime` | Yes | End of window, format `yyyy-MM-dd HH:mm:ss` |

### Response Envelope (all endpoints)
```json
{
  "code": "int | null",
  "msg": "string | null",
  "data": "<endpoint-specific>",
  "success": "boolean | null"
}
```

### Model Usage Data (`data` field)
```json
{
  "x_time": ["yyyy-MM-dd HH:mm", ...],
  "modelCallCount": ["long", ...],
  "tokensUsage": ["long", ...],
  "totalUsage": {
    "totalModelCallCount": "long",
    "totalTokensUsage": "long",
    "modelSummaryList": [
      { "modelName": "string", "totalTokens": "long", "sortOrder": "int | null" }
    ]
  },
  "modelDataList": [
    { "modelName": "string", "sortOrder": "int | null", "tokensUsage": ["long", ...], "totalTokens": "long" }
  ],
  "modelSummaryList": [
    { "modelName": "string", "totalTokens": "long", "sortOrder": "int | null" }
  ],
  "granularity": "string | null"
}
```

### Tool Usage Data (`data` field)
```json
{
  "x_time": ["yyyy-MM-dd HH:mm", ...],
  "networkSearchCount": ["long", ...],
  "webReadMcpCount": ["long", ...],
  "zreadMcpCount": ["long", ...],
  "totalUsage": {
    "totalNetworkSearchCount": "long",
    "totalWebReadMcpCount": "long",
    "totalZreadMcpCount": "long",
    "totalSearchMcpCount": "long",
    "toolDetails": [
      { "toolName": "string | null", "modelCode": "string | null", "usage": "long", "sortOrder": "int | null" }
    ],
    "toolSummaryList": [
      { "toolName": "string | null", "modelCode": "string | null", "usage": "long", "sortOrder": "int | null" }
    ]
  },
  "toolDataList": [
    { "toolName": "string | null", "sortOrder": "int | null", "usageCount": ["long", ...], "totalUsage": "long" }
  ],
  "toolSummaryList": [
    { "toolName": "string | null", "modelCode": "string | null", "usage": "long", "sortOrder": "int | null" }
  ],
  "granularity": "string | null"
}
```

### Quota Limit Data (`data` field)
```json
{
  "limits": [
    {
      "type": "string (e.g. TOKENS_LIMIT, TIME_LIMIT)",
      "unit": "int | null (3 = hour, 5 = month)",
      "number": "int | null (duration count)",
      "usage": "long | null",
      "currentValue": "long | null",
      "remaining": "long | null",
      "percentage": "int | null (0-100)",
      "nextResetTime": "long (epoch ms) | null",
      "usageDetails": [
        { "modelCode": "string", "usage": "long" }
      ]
    }
  ],
  "level": "string | null"
}
```

### Notes
- `success` should be `true` and `code` should be `200` for a successful call.
- `x_time` uses `yyyy-MM-dd HH:mm` granularity.
- `unit` mapping: `3` = hour, `5` = month.

---

## Zhipu BigModel

### Base URL
Dynamic / configurable; default is the Zhipu console domain (e.g. `https://<tenant>.bigmodel.cn/`).

### Endpoints

#### 1. Model Usage
```
GET api/monitor/usage/model-usage
```

#### 2. Tool Usage
```
GET api/monitor/usage/tool-usage
```

#### 3. Quota Limit
```
GET api/monitor/usage/quota/limit
```

### Request Headers
| Header | Required | Description |
|--------|----------|-------------|
| `Authorization` | Yes | Bearer token |
| `Accept-Language` | No | `en-US,en` (sent by client) |
| `Content-Type` | No | `application/json` (sent by client) |

### Query Parameters (model-usage & tool-usage)
| Parameter | Required | Description |
|-----------|----------|-------------|
| `startTime` | Yes | Start of window, format `yyyy-MM-dd HH:mm:ss` |
| `endTime` | Yes | End of window, format `yyyy-MM-dd HH:mm:ss` |

### Response Envelope (all endpoints)
```json
{
  "code": "int | null",
  "msg": "string | null",
  "data": "<endpoint-specific>",
  "success": "boolean | null"
}
```

### Model Usage Data (`data` field)
```json
{
  "x_time": ["yyyy-MM-dd HH:mm", ...],
  "modelCallCount": ["long", ...],
  "tokensUsage": ["long", ...],
  "totalUsage": {
    "totalModelCallCount": "long",
    "totalTokensUsage": "long",
    "modelSummaryList": [
      { "modelName": "string", "totalTokens": "long", "sortOrder": "int | null" }
    ]
  },
  "modelDataList": [
    { "modelName": "string", "sortOrder": "int | null", "tokensUsage": ["long", ...], "totalTokens": "long" }
  ],
  "modelSummaryList": [
    { "modelName": "string", "totalTokens": "long", "sortOrder": "int | null" }
  ],
  "granularity": "string | null"
}
```

### Tool Usage Data (`data` field)
```json
{
  "x_time": ["yyyy-MM-dd HH:mm", ...],
  "networkSearchCount": ["long", ...],
  "webReadMcpCount": ["long", ...],
  "zreadMcpCount": ["long", ...],
  "totalUsage": {
    "totalNetworkSearchCount": "long",
    "totalWebReadMcpCount": "long",
    "totalZreadMcpCount": "long",
    "totalSearchMcpCount": "long",
    "toolDetails": [
      { "toolName": "string | null", "modelCode": "string | null", "usage": "long", "sortOrder": "int | null" }
    ],
    "toolSummaryList": [
      { "toolName": "string | null", "modelCode": "string | null", "usage": "long", "sortOrder": "int | null" }
    ]
  },
  "toolDataList": [
    { "toolName": "string | null", "sortOrder": "int | null", "usageCount": ["long", ...], "totalUsage": "long" }
  ],
  "toolSummaryList": [
    { "toolName": "string | null", "modelCode": "string | null", "usage": "long", "sortOrder": "int | null" }
  ],
  "granularity": "string | null"
}
```

### Quota Limit Data (`data` field)
```json
{
  "limits": [
    {
      "type": "string (e.g. TOKENS_LIMIT, TIME_LIMIT)",
      "unit": "int | null (3 = hour, 5 = month)",
      "number": "int | null (duration count)",
      "usage": "long | null",
      "currentValue": "long | null",
      "remaining": "long | null",
      "percentage": "int | null (0-100)",
      "nextResetTime": "long (epoch ms) | null",
      "usageDetails": [
        { "modelCode": "string", "usage": "long" }
      ]
    }
  ],
  "level": "string | null"
}
```

### Notes
- `success` should be `true` and `code` should be `200` for a successful call.
- `x_time` uses `yyyy-MM-dd HH:mm` granularity.
- `unit` mapping: `3` = hour, `5` = month.
- Zhipu and Z.ai share the same **MonitorQuota** envelope pattern, but they target different base URLs and brands.

---

## Common Client Configuration

All providers use the following HTTP client defaults in QuotaHub:
- **Connect timeout:** 30 seconds
- **Read timeout:** 30 seconds
- **Write timeout:** 30 seconds
- **JSON parser:** Kotlinx Serialization with `ignoreUnknownKeys = true` and `coerceInputValues = true`
- **Logging:** `HttpLoggingInterceptor.Level.BASIC`

---

## Error Handling Summary

| Provider | Error Indicator |
|----------|-----------------|
| **Codex** | HTTP error codes / missing fields |
| **Kimi** | HTTP error codes / parse failures |
| **MiniMax** | `base_resp.status_code != 0` |
| **Z.ai** | `success == false` or `code != 200` |
| **Zhipu** | `success == false` or `code != 200` |

---

*Generated from QuotaHub source code analysis.*
