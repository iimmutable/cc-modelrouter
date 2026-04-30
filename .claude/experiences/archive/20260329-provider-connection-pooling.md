# Provider Connection Pooling Pitfalls

**Date:** 2026-03-29
**Tags:** #provider #http #connection-pooling #bigmodel

## Problem

BigModel (GLM) API returns error code 1213 when HTTP keep-alive connections are reused. This causes intermittent failures that look like API errors but are actually transport-level issues.

## Root Cause

BigModel's API gateway doesn't properly support HTTP keep-alive. When Go's default `http.Transport` reuses a TCP connection for a second request, BigModel responds with error 1213.

## Solution

Add `disableKeepAlives: true` to the provider configuration:

```json
{
  "bigmodel": {
    "apiKey": "${BIGMODEL_API_KEY}",
    "baseURL": "https://open.bigmodel.cn/api/anthropic",
    "models": ["glm-4.7"],
    "disableKeepAlives": true
  }
}
```

This sets `Transport.DisableKeepAlives = true` in the HTTP client, forcing a new connection for each request.

## When to Use

- BigModel/GLM providers — always enable
- Aliyun DashScope — may also benefit
- OpenRouter, Anthropic, Gemini — not needed (proper keep-alive support)

## Diagnosis

Error 1213 in logs, appearing intermittently on the second or later request to the same provider.
