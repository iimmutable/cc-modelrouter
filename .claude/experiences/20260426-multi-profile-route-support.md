# Multi-Profile Route Support with Hot-Reload

**Date:** 2026-04-26
**Tags:** #config #profiles #hot-reload #cli #tui #backward-compat #architecture

## Problem

Users needed to switch between different routing strategies (e.g., "fast" for quick responses, "quality" for complex reasoning) without restarting the router. The original config only supported a single set of routes.

## Solution

### Architecture

Profiles are named route configurations embedded in `RouterConfig`:

```go
type ProfileConfig struct {
    Name        string            `json:"name"`
    Description string            `json:"description,omitempty"`
    Routes      map[string]string `json:"routes"`
}

type RouterConfig struct {
    Routes   map[string]string            `json:"routes,omitempty"`   // legacy
    Profiles map[string]ProfileConfig     `json:"profiles,omitempty"` // new
}
```

### Key Design Decisions

1. **Config vs Runtime State** — Profiles are defined in config files. Active profile is ephemeral runtime state (not persisted). Startup prefers "default" profile, falls back to first alphabetically.

2. **Backward Compatibility** — Legacy `routes` at root level still work. On load, if old `cfg.Profiles` exists at root, silently migrates to `cfg.Router.Profiles`. Legacy route selection logic falls back gracefully.

3. **Hot-Reload via Admin API** — Profile switching uses localhost-only admin endpoints (`/_admin/profiles/switch`). No restart needed. Updates handler, router engine, and instance metadata atomically.

4. **Default Profile Locking** — The "default" profile cannot be renamed in the TUI, ensuring predictable startup behavior.

### Migration Patterns

```go
// Silent promotion during config loading
if len(cfg.Profiles) > 0 && len(cfg.Router.Profiles) == 0 {
    cfg.Router.Profiles = cfg.Profiles
    cfg.Profiles = nil // cleared on save
}
```

### CLI Flow

1. `profile list` — queries config file or running instance
2. `profile switch <name>` — calls admin API on running instance, updates instance metadata on success
3. `profile status` — shows active profile from instance metadata

### TUI Integration

- Full-screen editing (converted from modal for better UX)
- Tab navigation with [+] for profile creation
- Auto-migration of legacy routes to "default" profile on wizard startup
- Locked "default" profile name

## Pitfalls

- **Admin token security** — 32-char random token stored in instance metadata. Never logged. Localhost-only access.
- **Profile switch atomicity** — Switch updates handler + router + metadata. Failure mid-process could leave inconsistent state. Mitigation: metadata updated only after successful API call.
- **Legacy fallback complexity** — `GetActiveRoutes()` must check both profiles and legacy routes. Don't simplify this away — existing users rely on it.
- **Config schema conflicts** — Old configs had profiles at root level, new location is nested. Migration handles both but don't save back to old location.

## Files

- `internal/config/types.go` — ProfileConfig, RouterConfig types
- `internal/config/loader.go` — Migration logic
- `internal/cli/profile.go` — CLI commands
- `internal/proxy/admin_handler.go` — Admin API endpoints
- `internal/proxy/handler.go` — `UpdateActiveProfile()` for hot-reload
- `internal/router/engine.go` — `SetActiveProfile()`, `GetRoutes()`
- `internal/configwizard/wizard.go` — TUI profile editing
- `internal/daemon/instance.go` — Instance metadata tracking
