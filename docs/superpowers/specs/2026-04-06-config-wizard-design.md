# ccrouter Configuration Wizard — Design Specification

**Date:** 2026-04-06
**Author:** Claude
**Status:** Approved

---

## 1. Overview

A **hybrid menu-driven + step-by-step wizard** TUI application for managing ccrouter configuration. Built with **charmbracelet/lipgloss** and **bubbles** libraries (already in use by the monitor module).

**Launch command:**
```bash
ccrouter config wizard    # or: ccrouter config edit
```

---

## 2. Navigation Architecture

```
Main Menu
├── [1] Providers      → Provider list → Add/Edit/View → Test
├── [2] Routes         → Route list → Add/Edit chain
├── [3] Server         → Host/Port config
├── [4] Logging        → Level/Destination config
├── [5] View Config    → Read-only display
├── [6] Save & Exit    → Write to disk
```

**Navigation keys:**
- `↑/↓` — Navigate menu
- `Enter` — Select/Confirm
- `Esc` — Go back/Cancel
- `Ctrl+C` — Exit (with unsaved changes warning)

---

## 3. Screen Designs

### 3.1 Main Menu

```
┌─────────────────────────────────────────────────────────────┐
│            🛠️  ccrouter Configuration Wizard               │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│   Current Config: ~/.cc-modelrouter/config.json           │
│                                                             │
│   [1] 🏢 Providers      Manage API providers (3 configured) │
│   [2] 🛣️  Routes       Configure routing rules            │
│   [3] ⚙️  Server       Host: localhost, Port: 8081        │
│   [4] 📝 Logging       Level: info, Destination: stdout   │
│   [5] 👁️  View Config  Browse current configuration        │
│   ─────────────────────────────────────────────────────    │
│   [6] 💾 Save & Exit    Write changes to disk              │
│                                                             │
│   [↑/↓] Navigate   [Enter] Select   [Esc] Quit           │
└─────────────────────────────────────────────────────────────┘
```

### 3.2 Providers Screen

```
┌─────────────────────────────────────────────────────────────┐
│  Providers                                    [Esc] Back   │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│   ✅ anthropic          [Test] ✓ Connected                 │
│      ├─ Base: https://api.anthropic.com                   │
│      └─ Models: claude-opus-4, claude-sonnet-4            │
│                                                             │
│   ✅ openrouter         [Test] ✓ Connected                 │
│      ├─ Base: https://openrouter.ai/api                   │
│      └─ Models: anthropic/claude-opus-4                   │
│                                                             │
│   ❌ bigmodel           [Test] ✗ Failed                    │
│      ├─ Base: https://open.bigmodel.cn/api                 │
│      └─ Models: glm-4.7                                   │
│                                                             │
│   [+ Add Provider]                                         │
│                                                             │
│   [↑/↓] Navigate   [Enter] Edit   [T] Test   [Del] Delete │
└─────────────────────────────────────────────────────────────┘
```

### 3.3 Add Provider Wizard (2 Steps)

**Step 1: Provider Details**
```
┌─────────────────────────────────────────────────────────────┐
│  Add Provider (1/2)                          [Esc] Cancel   │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│   Provider Name: [anthropic________________]              │
│                                                             │
│   Base URL:       [https://api.anthropic.com____]         │
│   ┌─────────────────────────────────────────────────────┐  │
│   │ [Anthropic] [OpenRouter] [BigModel] [Gemini] [Custom]│  │
│   └─────────────────────────────────────────────────────┘  │
│                                                             │
│   Transformer: [anthropic                        ▼]       │
│                                                             │
│   Models (one per line):                                    │
│   ┌─────────────────────────────────────────────────────┐  │
│   │ claude-sonnet-4-20250514                           │  │
│   │ claude-opus-4-20250514                             │  │
│   └─────────────────────────────────────────────────────┘  │
│                                                             │
│            [Cancel]              [Next: Setup API Key →]   │
└─────────────────────────────────────────────────────────────┘
```

**Step 2: Environment Variable Setup**
```
┌─────────────────────────────────────────────────────────────┐
│  Environment Setup (2/2)                      [Esc] Back   │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│   Provider: anthropic                                        │
│                                                             │
│   ─────────────────────────────────────────────────────    │
│                                                             │
│   Environment Variable:                                     │
│                                                             │
│   Name: ANTHROPIC_API_KEY (auto-generated)                 │
│                                                             │
│   Enter API Key: [_________________________________]       │
│                    [  Test Connection  ]                    │
│                                                             │
│   ─────────────────────────────────────────────────────    │
│                                                             │
│   Shell Configuration:                                      │
│                                                             │
│   [✓] Add to shell config (~/.zshrc)                      │
│   [✓] Source environment now (export to current shell)    │
│                                                             │
│   Preview (~/.zshrc):                                       │
│   ┌─────────────────────────────────────────────────────┐  │
│   │ # ccrouter - anthropic                              │  │
│   │ export ANTHROPIC_API_KEY="sk-ant-..."               │  │
│   └─────────────────────────────────────────────────────┘  │
│                                                             │
│            [← Back]              [Save Provider]          │
└─────────────────────────────────────────────────────────────┘
```

### 3.4 Test Connectivity Modal

On testing:
```
┌─────────────────────────────────────────────────────────────┐
│  Testing: anthropic → claude-opus-4-20250514               │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│   ┌─────────────────────────────────────────────────────┐  │
│   │  ⟳ Sending test request...                         │  │
│   └─────────────────────────────────────────────────────┘  │
│                                                             │
│   [ Cancel ]                                               │
└─────────────────────────────────────────────────────────────┘
```

On success:
```
┌─────────────────────────────────────────────────────────────┐
│  ✅ Connection Successful                      [Esc] Close │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│   Provider: anthropic                                       │
│   Model: claude-opus-4-20250514                            │
│   Latency: 1.2s                                             │
│                                                             │
│   Tokens:                                                  │
│   ├─ Input:  10                                            │
│   └─ Output: 45                                            │
│                                                             │
│   Cost estimate: $0.0039                                   │
│                                                             │
│              [ Close ]                                     │
└─────────────────────────────────────────────────────────────┘
```

On failure:
```
┌─────────────────────────────────────────────────────────────┐
│  ❌ Connection Failed                          [Esc] Close │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│   Provider: anthropic                                       │
│   Model: claude-opus-4-20250514                            │
│                                                             │
│   Error: Invalid API key                                    │
│   (Check your ANTHROPIC_API_KEY environment variable)      │
│                                                             │
│              [ Close ]                                     │
└─────────────────────────────────────────────────────────────┘
```

### 3.5 Routes Screen

```
┌─────────────────────────────────────────────────────────────┐
│  Routes                                         [Esc] Back  │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│   Route              │ Chain                               │
│   ─────────────────────────────────────────────────────    │
│   default            │ anthropic:claude-sonnet-4           │
│   background         │ anthropic:claude-haiku-3-5          │
│   think              │ anthropic:claude-sonnet-4           │
│   thinkMore          │ openrouter:anthropic/claude-3.5-s   │
│   ultrathink         │ openrouter:anthropic/claude-opus-4  │
│   longContext        │ anthropic:claude-sonnet-4           │
│   image              │ openrouter:anthropic/claude-3.5-s   │
│   webSearch          │ anthropic:claude-sonnet-4           │
│                                                             │
│   [+ Add Route]  [Enter] Edit Chain                        │
│                                                             │
│   [↑/↓] Navigate   [Enter] Edit   [Del] Delete             │
└─────────────────────────────────────────────────────────────┘
```

### 3.6 Add/Edit Route

```
┌─────────────────────────────────────────────────────────────┐
│  Add/Edit Route                                  [Esc] Back │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│   Route Name: [ultrathink                      ▼]         │
│                 ├─ default                                  │
│                 ├─ background                               │
│                 ├─ think                                    │
│                 ├─ thinkMore                                │
│                 ├─ ultrathink ← selected                    │
│                 ├─ longContext                              │
│                 ├─ image                                    │
│                 └─ webSearch                               │
│                                                             │
│   ─────────────────────────────────────────────────────    │
│                                                             │
│   Failover Chain:                                          │
│   ┌─────────────────────────────────────────────────────┐ │
│   │ [1] anthropic:claude-opus-4-20250514        [↑] [↓] │ │
│   │ [2] openrouter:anthropic/claude-opus-4      [↑] [↓] │ │
│   │ [3] bigmodel:glm-4.7                      [↑] [↓] │ │
│   └─────────────────────────────────────────────────────┘ │
│                                                             │
│   Add to chain:                                             │
│   Provider: [anthropic                    ▼]               │
│   Model:    [claude-opus-4-20250514    ▼]                  │
│                        [Add to Chain]                      │
│                                                             │
│   [Remove Selected]                                         │
│                                                             │
│            [Cancel]              [Save Route]             │
└─────────────────────────────────────────────────────────────┘
```

### 3.7 Server Settings Screen

```
┌─────────────────────────────────────────────────────────────┐
│  Server Settings                               [Esc] Back  │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│   Host: [localhost___________________________________]     │
│                                                             │
│   Port: [8081________________________________________]     │
│                                                             │
│   Note: Must be between 1024-65535                         │
│                                                             │
│            [Cancel]              [Save]                   │
└─────────────────────────────────────────────────────────────┘
```

### 3.8 Logging Settings Screen

```
┌─────────────────────────────────────────────────────────────┐
│  Logging Settings                              [Esc] Back  │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│   Enable Logging: [✓]                                      │
│                                                             │
│   Level:        [info                        ▼]             │
│                 ├─ debug                                    │
│                 ├─ info  ← selected                        │
│                 ├─ warn                                    │
│                 └─ error                                   │
│                                                             │
│   Destination: [stdout                      ▼]             │
│                 ├─ stdout                                  │
│                 ├─ stderr                                  │
│                 └─ file                                    │
│                                                             │
│   File Path: (shown when "file" destination selected)      │
│   ~/.cc-modelrouter/router.log                            │
│                                                             │
│            [Cancel]              [Save]                   │
└─────────────────────────────────────────────────────────────┘
```

### 3.9 View Config Screen (Read-only)

```
┌─────────────────────────────────────────────────────────────┐
│  Current Configuration (Read-only)             [Esc] Back  │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│   Server:                                                   │
│   ├─ Host: localhost                                        │
│   └─ Port: 8081                                            │
│                                                             │
│   Providers (3):                                            │
│   ├─ anthropic                                             │
│   │   ├─ URL: https://api.anthropic.com                    │
│   │   ├─ Transformer: anthropic                            │
│   │   └─ Models: claude-opus-4, claude-sonnet-4           │
│   ├─ openrouter                                            │
│   │   ├─ URL: https://openrouter.ai/api                    │
│   │   ├─ Transformer: anthropic                            │
│   │   └─ Models: anthropic/claude-opus-4                  │
│   └─ bigmodel                                              │
│       ├─ URL: https://open.bigmodel.cn/api                 │
│       ├─ Transformer: glm_anthropic                        │
│       └─ Models: glm-4.7                                   │
│                                                             │
│   Routes (8):                                               │
│   ├─ default → anthropic:claude-sonnet-4                   │
│   ├─ background → anthropic:claude-haiku-3-5              │
│   ├─ think → anthropic:claude-sonnet-4                    │
│   ├─ thinkMore → openrouter:anthropic/claude-3.5-s       │
│   ├─ ultrathink → openrouter:anthropic/claude-opus-4     │
│   ├─ longContext → anthropic:claude-sonnet-4              │
│   ├─ image → openrouter:anthropic/claude-3.5-s           │
│   └─ webSearch → anthropic:claude-sonnet-4               │
│                                                             │
│   Logging:                                                  │
│   ├─ Enabled: true                                         │
│   ├─ Level: info                                           │
│   └─ Destination: stdout                                   │
│                                                             │
│   Press [P] to export as JSON                              │
│                                                             │
│            [ Close ]                                       │
└─────────────────────────────────────────────────────────────┘
```

---

## 4. Environment Variable Naming Convention

| Provider | Environment Variable |
|----------|---------------------|
| `anthropic` | `ANTHROPIC_API_KEY` |
| `openrouter` | `OPENROUTER_API_KEY` |
| `bigmodel` | `BIGMODEL_API_KEY` |
| `gemini` | `GEMINI_API_KEY` |
| `<custom>` | `<PROVIDER_NAME>_API_KEY` (uppercase) |

**Storage format in config.json:**
```json
{
  "providers": {
    "anthropic": {
      "apiKey": "${ANTHROPIC_API_KEY}",
      "baseURL": "https://api.anthropic.com",
      ...
    }
  }
}
```

---

## 5. Shell Integration

**Files modified:**
- `~/.zshrc` (or `~/.bashrc` if bash detected)

**Content added:**
```bash
# ccrouter - anthropic
export ANTHROPIC_API_KEY="sk-ant-..."
```

**Source options:**
1. **Add to shell config** — Appends export to `~/.zshrc`
2. **Source immediately** — Exports to current shell session
3. **Skip** — Just store `${VAR}` in config, user must manually source

---

## 6. Data Flow

```
User Input → Validation → State Model → Preview → Save → Disk
                  ↓
          Test Connectivity
          (per provider/model)
```

---

## 7. Error Handling

| Scenario | Behavior |
|----------|----------|
| Invalid provider name | Show error, highlight field |
| Duplicate provider | Show warning, offer to overwrite |
| Empty models list | Show error, require at least 1 |
| Test connectivity fails | Show error modal with details |
| Unsaved changes on exit | Confirmation dialog |
| Config file not writable | Error with chmod suggestion |

---

## 8. Keyboard Shortcuts Summary

| Key | Action |
|-----|--------|
| `↑/↓` | Navigate menu/items |
| `Enter` | Select/Confirm |
| `Esc` | Go back/Cancel |
| `Ctrl+C` | Exit (with unsaved warning) |
| `T` | Test connectivity (in provider list) |
| `Del` | Delete item |
| `Tab` | Switch input field |

---

## 9. Acceptance Criteria

1. ✅ Wizard launches with `ccrouter config wizard`
2. ✅ Main menu navigates to all config sections
3. ✅ Add provider in 2 steps max
4. ✅ API keys stored as `${PROVIDER_NAME}_API_KEY` only
5. ✅ Shell config prompt (.zshrc) works
6. ✅ Test connectivity works per provider/model
7. ✅ Edit route chains with drag-reorder
8. ✅ View current config (read-only)
9. ✅ Save with validation and preview
10. ✅ Unsaved changes warning on exit