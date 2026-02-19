# Configuration Guide

## Configuration Files

### Locations

| Scope | Path | Priority |
|-------|------|----------|
| Global | `~/.cc-modelrouter/config.json` | Low |
| Project | `<project>/.cc-modelrouter/config.json` | High (overrides global) |

Project configuration **completely overrides** global configuration when present. There is no deep merging.

### Basic Structure

```json
{
  "server": {
    "port": 8081,
    "host": "localhost"
  },
  "providers": {
    "provider-name": {
      "apiKey": "your-api-key",
      "baseURL": "https://api.example.com",
      "models": ["model-1", "model-2"]
    }
  },
  "router": {
    "routes": {
      "default": "provider:model",
      "background": "provider:model"
    },
    "maxRetries": 2,
    "retryDelay": "500ms"
  }
}
```

## Server Configuration

```json
{
  "server": {
    "port": 8081,
    "host": "localhost"
  }
}
```

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `port` | int | 8081 | Port to listen on |
| `host` | string | localhost | Host to bind to |

## Provider Configuration

```json
{
  "providers": {
    "openrouter": {
      "apiKey": "${OPENROUTER_API_KEY}",
      "baseURL": "https://openrouter.ai/api/v1",
      "models": [
        "anthropic/claude-sonnet-4",
        "google/gemini-2.5-pro"
      ]
    }
  }
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `apiKey` | string | Yes | API key (supports env vars) |
| `baseURL` | string | Yes | API base URL |
| `models` | []string | Yes | List of available models |

### Supported Providers

#### OpenRouter

```json
{
  "openrouter": {
    "apiKey": "${OPENROUTER_API_KEY}",
    "baseURL": "https://openrouter.ai/api/v1",
    "models": ["anthropic/claude-sonnet-4"]
  }
}
```

- **Transformer**: `openrouter` (OpenAI-compatible)
- **Auth**: `Authorization: Bearer <key>`

#### Google Gemini

```json
{
  "gemini": {
    "apiKey": "${GEMINI_API_KEY}",
    "baseURL": "https://generativelanguage.googleapis.com/v1beta",
    "models": ["gemini-2.0-flash", "gemini-2.5-pro"]
  }
}
```

- **Transformer**: `gemini` (native format)
- **Auth**: Query parameter `key=<api-key>`

#### Alibaba Qwen (DashScope)

```json
{
  "qwen": {
    "apiKey": "${DASHSCOPE_API_KEY}",
    "baseURL": "https://dashscope.aliyuncs.com/compatible-mode/v1",
    "models": ["qwen-turbo", "qwen-plus"]
  }
}
```

- **Transformer**: `qwen` (OpenAI-compatible)
- **Auth**: `Authorization: Bearer <key>`

#### Zhipu GLM (BigModel)

```json
{
  "bigmodel": {
    "apiKey": "${BIGMODEL_API_KEY}",
    "baseURL": "https://open.bigmodel.cn/api/anthropic",
    "models": ["glm-4.7", "glm-4.5-air", "glm-4.6v"]
  }
}
```

- **Transformer**: `glm` (Anthropic-compatible)
- **Auth**: `Authorization: Bearer <key>`

#### Anthropic (Direct)

```json
{
  "anthropic": {
    "apiKey": "${ANTHROPIC_API_KEY}",
    "baseURL": "https://api.anthropic.com",
    "models": ["claude-sonnet-4-20250514"]
  }
}
```

- **Transformer**: `anthropic` (pass-through)
- **Auth**: `x-api-key: <key>`

## Router Configuration

```json
{
  "router": {
    "routes": {
      "default": "openrouter:anthropic/claude-sonnet-4",
      "background": "bigmodel:glm-4.5-air",
      "think": "openrouter:anthropic/claude-sonnet-4",
      "thinkMore": "openrouter:anthropic/claude-sonnet-4",
      "ultrathink": "openrouter:anthropic/claude-opus-4",
      "longContext": "gemini:gemini-2.5-pro",
      "webSearch": "gemini:gemini-2.5-pro",
      "image": "bigmodel:glm-4.6v"
    },
    "maxRetries": 2,
    "retryDelay": "500ms"
  }
}
```

### Routes

| Route | Description | Trigger | Detection Method |
|-------|-------------|---------|------------------|
| `default` | Default fallback | All unmatched requests | - |
| `background` | Background tasks | Claude Code background agents | Model contains "claude" + "haiku" |
| `think` | Basic thinking | "think" trigger phrase | `budget_tokens >= 4,000` |
| `thinkMore` | Enhanced thinking | "think hard", "think more" | `budget_tokens >= 10,000` |
| `ultrathink` | Maximum thinking | "ultrathink", "think harder" | `budget_tokens >= 32,000` |
| `longContext` | Long conversations | Large context | Token count > 60,000 |
| `webSearch` | Web search enabled | Web search tools | Tool names contain "web"/"search" |
| `image` | Image processing | Images in request | Request contains image blocks |

### Thinking Levels

Claude Code supports multiple thinking intensity levels. When a user types trigger phrases like "think", "think more", or "ultrathink", Claude Code converts these to specific `budget_tokens` values before sending the API request.

| Level | Budget Tokens | Route | Trigger Phrases |
|-------|---------------|-------|-----------------|
| Basic | ~4,000 | `think` | "think", "思考" |
| Middle | ~10,000 | `thinkMore` | "think hard", "think more", "think deeply", "megathink", "好好想", "多想想" |
| Highest | ~32,000 | `ultrathink` | "ultrathink", "think harder", "think intensely", "think longer", "仔细思考", "深思" |

**Fallback Behavior:**

The router supports flexible thinking configuration with automatic fallback:

1. **Full 3-tier config:** Configure `think`, `thinkMore`, and `ultrathink` for different models at each level
2. **2-tier config:** Configure only `think` and `thinkMore` - highest level uses `thinkMore`
3. **1-tier config:** Configure only `think` - all thinking levels use `think`

Example for cost optimization:
```json
{
  "router": {
    "routes": {
      "default": "openrouter:claude-sonnet-4",
      "think": "openrouter:claude-sonnet-4",
      "thinkMore": "openrouter:claude-sonnet-4",
      "ultrathink": "openrouter:claude-opus-4"
    }
  }
}
```

### Route Format

```
provider:model[;provider:model;...]
```

Multiple targets are tried in sequence with failover:

```json
{
  "default": "openrouter:claude-sonnet-4;bigmodel:glm-4.7;gemini:gemini-2.5-pro"
}
```

### Retry Configuration

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `maxRetries` | int | 2 | Max retries per route |
| `retryDelay` | string | 500ms | Delay between retries |

## Environment Variables

Use `${VAR_NAME}` or `$VAR_NAME` syntax for secure value injection:

```json
{
  "providers": {
    "openrouter": {
      "apiKey": "${OPENROUTER_API_KEY}",
      "baseURL": "https://openrouter.ai/api/v1"
    }
  }
}
```

### Setting Environment Variables

```bash
# In ~/.bashrc or ~/.zshrc
export OPENROUTER_API_KEY="sk-or-..."
export GEMINI_API_KEY="AIza..."
export BIGMODEL_API_KEY="..."
```

## Complete Example

```json
{
  "server": {
    "port": 8081,
    "host": "localhost"
  },
  "providers": {
    "bigmodel": {
      "apiKey": "${BIGMODEL_API_KEY}",
      "baseURL": "https://open.bigmodel.cn/api/anthropic",
      "models": ["glm-4.7", "glm-4.5-air", "glm-4.6v"]
    },
    "openrouter": {
      "apiKey": "${OPENROUTER_API_KEY}",
      "baseURL": "https://openrouter.ai/api/v1",
      "models": [
        "anthropic/claude-sonnet-4",
        "anthropic/claude-opus-4",
        "google/gemini-2.5-pro"
      ]
    },
    "gemini": {
      "apiKey": "${GEMINI_API_KEY}",
      "baseURL": "https://generativelanguage.googleapis.com/v1beta",
      "models": ["gemini-2.5-pro", "gemini-2.0-flash"]
    },
    "qwen": {
      "apiKey": "${DASHSCOPE_API_KEY}",
      "baseURL": "https://dashscope.aliyuncs.com/compatible-mode/v1",
      "models": ["qwen-turbo", "qwen-plus"]
    }
  },
  "router": {
    "routes": {
      "default": "bigmodel:glm-4.7;openrouter:anthropic/claude-sonnet-4",
      "background": "bigmodel:glm-4.5-air",
      "think": "openrouter:anthropic/claude-sonnet-4",
      "thinkMore": "openrouter:anthropic/claude-sonnet-4",
      "ultrathink": "openrouter:anthropic/claude-opus-4",
      "longContext": "gemini:gemini-2.5-pro;openrouter:google/gemini-2.5-pro",
      "webSearch": "gemini:gemini-2.5-pro",
      "image": "bigmodel:glm-4.6v;gemini:gemini-2.5-pro"
    },
    "maxRetries": 2,
    "retryDelay": "500ms"
  }
}
```

## Project-Level Override

For project-specific configuration, create `.cc-modelrouter/config.json` in your project root:

```
my-project/
├── .cc-modelrouter/
│   └── config.json    # Project-specific config
├── src/
└── ...
```

When running `ccrouter code` from within the project directory, the project config will be used instead of the global config.

## Viewing Active Configuration

```bash
ccrouter config
```

This displays the currently active configuration (global or project-level).
