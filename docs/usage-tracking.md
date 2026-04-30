# Usage Tracking Design

**Date:** 2025-02-23
**Status:** Design Complete

## Overview

This document describes the design of the usage tracking feature for `cc-modelrouter`. The feature enables monitoring of token usage statistics per model, per route, and per instance through a CLI command.

## Requirements

- Track token usage for each proxy request
- Store records with instance ID, route, model, tokens, and fallbacks
- Provide CLI command to query and display usage statistics
- Support filtering by instance ID and time period
- No external dependencies (pure Go SQLite)

## CLI Interface

Usage statistics are viewed through the live terminal UI:

```bash
ccrouter monitor
```

See [CLI Reference - monitor](cli-reference.md#ccrouter-monitor) for details and flags.

### Output Format

The monitor dashboard displays:

```
Usage Summary (TODAY, all instances)
  Requests: 1,234  |  Tokens: 45.6M  |  Fallbacks: 12

By Route:
  Route                Requests    Tokens      Fallbacks
  ────────────────────────────────────────────────────────
  /think               800         30.2M       8
  /thinkMore           300         12.0M       3
  /ultrathink          134         3.4M        1

By Model:
  Model                Requests    Tokens
  ────────────────────────────────────────────────
  gpt-4o               600         25.0M
  claude-sonnet-4      400         15.0M
  gemini-2.0-flash     234         5.6M
```

**Note:** Fallbacks are route-level metrics only (cannot attribute to specific models).

## Storage Design

### Database

- **Engine:** SQLite via `modernc.org/sqlite` (pure Go, no CGO)
- **Location:** `~/.cc-modelrouter/usage.db`
- **Approach:** Hybrid in-memory buffer + persistent storage

### Schema

```sql
CREATE TABLE usage_records (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    instance_id TEXT NOT NULL,
    route TEXT NOT NULL,
    model TEXT NOT NULL,
    tokens INTEGER NOT NULL,
    fallbacks INTEGER NOT NULL DEFAULT 0,
    timestamp DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_instance_route ON usage_records(instance_id, route);
CREATE INDEX idx_timestamp ON usage_records(timestamp);
```

### Flush Strategy

| Setting | Value |
|---------|-------|
| Buffer size | 500 records |
| Flush timeout | 3 seconds |
| Flush method | Batch transaction |

Records are buffered in memory and flushed to SQLite when either:
1. The buffer reaches 500 records
2. 3 seconds have passed since the last flush

## Implementation Architecture

```
┌─────────────────┐     ┌─────────────────┐     ┌─────────────────┐
│  Proxy Handler  │────▶│ Usage Tracker   │────▶│  SQLite DB      │
│                 │     │ (in-memory buf) │     │ (~/.cc-model..)│
└─────────────────┘     └─────────────────┘     └─────────────────┘
                                │
                                ▼
                         ┌─────────────────┐
                         │ Usage Command   │
                         │  CLI/Stats/     │
                         │  Formatter      │
                         └─────────────────┘
```

### Components

| File | Responsibility |
|------|----------------|
| `internal/usage/tracker.go` | In-memory buffer, flush coordination |
| `internal/usage/db.go` | SQLite operations (init, insert, query) |
| `internal/usage/stats.go` | Aggregation logic (by route, model, period) |
| `internal/usage/formatter.go` | Output formatting, token number formatting |
| `internal/usage/period.go` | Period parsing to time range |

## Token Number Formatting

| Value | Display |
|-------|---------|
| < 1,000 | `20` |
| < 1,000,000 | `680` (thousands separator) |
| < 10,000,000 | `1.2M` |
| < 100,000,000 | `3.0M` |
| >= 100,000,000 | `200.0M` |

## Integration Points

### Proxy Handler

When a proxy request completes:
1. Extract instance ID, route, model, tokens, fallbacks
2. Call `tracker.Record(instanceID, route, model, tokens, fallbacks)`

### CLI Root Command

Register the `usage` subcommand in `internal/cli/root.go`.

## Design Constraints

1. **No CGO:** Uses `modernc.org/sqlite` for single-binary distribution
2. **Fallbacks are route-level:** Cannot attribute fallbacks to specific models
3. **Optional instance-id:** Omitting shows aggregate across all instances
4. **Positional period:** Period is a positional argument, not a flag

## Future Considerations

- Configurable buffer size and flush timeout
- Export to CSV/JSON
- Per-model cost calculation (if pricing data available)
- Rate limiting based on usage

---

## Future Considerations
