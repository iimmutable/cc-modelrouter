# Go Header Logging Security Pattern

**Date:** 2026-03-29
**Tags:** #security #go #logging #api-keys

## Problem

Using Go's `%v` format verb with `http.Header` prints ALL headers including secrets (API keys, Bearer tokens). This leaks credentials into log files.

## The Dangerous Pattern

```go
// WRONG — leaks API keys into logs
logging.Debugf("[PROXY REQUEST] Headers: %v", req.Header)
logging.Debugf("[PROXY REQUEST] Headers: %s", req.Header)
```

Go's `http.Header.String()` and default formatting include the full value of every header.

## The Safe Pattern

Always use the sanitization library:

```go
import "github.com/iimmutable/cc-modelrouter/internal/logging"

// CORRECT — secrets are redacted
logging.Debugf("[PROXY REQUEST] Headers: %s", logging.SanitizeHeadersString(req.Header))
```

## Headers Automatically Redacted

- `Authorization` (Bearer tokens)
- `X-Api-Key` (Anthropic API keys)
- `X-Auth-Token`
- `Cookie` / `Set-Cookie`
- `Proxy-Authorization`
- `X-BigModel-Api-Key`
- `X-OpenRouter-Api-Key`

## Redaction Format

```
X-Api-Key:[sk-ant-**************** [REDACTED]]
Authorization:[Bearer**************** [REDACTED]]
```

## Lesson

This was not caught until 18 days after the logging system was built. Security review of logging code should happen at implementation time, not weeks later. Any header logging must use sanitization from day one.

## Verification

```bash
go test -v ./test/security
```
