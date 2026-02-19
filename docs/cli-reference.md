# CLI Reference

## Installation

```bash
go install github.com/iimmutable/cc-modelrouter/cmd/ccrouter@latest
```

## Global Options

```
  -h, --help      Show help
  -v, --version   Show version
```

## Commands

### ccrouter code

Start the router and launch Claude Code.

```bash
ccrouter code [flags]
```

**Flags:**
```
  -c, --config string   Path to config file
```

**Description:**
- Creates an isolated router instance
- Starts the HTTP server
- Launches Claude Code with `ANTHROPIC_BASE_URL` set to the router
- Handles graceful shutdown on SIGINT/SIGTERM

**Examples:**
```bash
# Use default or project config
ccrouter code

# Use specific config file
ccrouter code -c /path/to/config.json
```

---

### ccrouter start

Start the router server standalone.

```bash
ccrouter start [flags]
```

**Flags:**
```
  -c, --config string   Path to config file
  -p, --port int        Port to listen on (default from config)
```

**Description:**
- Starts the HTTP server in the foreground
- Saves instance metadata for management
- Does NOT launch Claude Code

**Examples:**
```bash
# Start with default config
ccrouter start

# Start on specific port
ccrouter start -p 9090

# Use specific config
ccrouter start -c /path/to/config.json
```

---

### ccrouter stop

Stop a router instance.

```bash
ccrouter stop [instance-id] [flags]
```

**Arguments:**
```
  instance-id   ID of instance to stop (optional)
```

**Flags:**
```
  --all   Stop all running instances
```

**Description:**
- Stops the specified instance by PID
- Removes instance metadata file
- If no ID provided and `--all` not set, shows error

**Examples:**
```bash
# Stop specific instance
ccrouter stop inst_20250216_143022

# Stop all instances
ccrouter stop --all
```

---

### ccrouter restart

Restart a router instance.

```bash
ccrouter restart [instance-id]
```

**Arguments:**
```
  instance-id   ID of instance to restart (required)
```

**Description:**
- Stops the instance
- Starts a new instance with the same configuration
- Reloads config from disk

**Examples:**
```bash
ccrouter restart inst_20250216_143022
```

---

### ccrouter status

Show all running instances.

```bash
ccrouter status
```

**Output:**
```
ID                      PORT    PID     CONFIG TYPE    STARTED
inst_20250216_143022    8081    12345   project        2025-02-16 14:30:22
inst_20250216_150033    8082    12346   global         2025-02-16 15:00:33
```

---

### ccrouter clean

Remove stale instance files.

```bash
ccrouter clean
```

**Description:**
- Removes metadata files for instances that are no longer running
- Useful for cleanup after crashes or manual process termination

---

### ccrouter config

Show active configuration.

```bash
ccrouter config
```

**Description:**
- Displays the currently active configuration as JSON
- Shows whether using global or project-level config

---

### ccrouter logs

Show logs for an instance.

```bash
ccrouter logs [instance-id]
```

**Arguments:**
```
  instance-id   ID of instance (optional, shows all if not provided)
```

**Examples:**
```bash
# Show all logs
ccrouter logs

# Show logs for specific instance
ccrouter logs inst_20250216_143022
```

---

## Instance Management

### Instance Metadata

Instances are stored in `~/.cc-modelrouter/instances/`:

```json
{
  "id": "inst_20250216_143022",
  "port": 8081,
  "pid": 12345,
  "configType": "project",
  "configPath": "/path/to/project/.cc-modelrouter/config.json",
  "startTime": "2025-02-16T14:30:22Z",
  "projectRoot": "/path/to/project"
}
```

### Directory Structure

```
~/.cc-modelrouter/
├── config.json                    # Global configuration
└── instances/
    ├── inst_20250216_143022.json  # Instance metadata
    ├── inst_20250216_150033.json
    └── ...
```

## Typical Workflows

### Development (Project-Specific)

```bash
cd /path/to/project

# Create project config
mkdir -p .cc-modelrouter
cat > .cc-modelrouter/config.json << 'EOF'
{
  "server": {"port": 8081},
  "providers": {...},
  "router": {...}
}
EOF

# Start with project config
ccrouter code
```

### Multiple Projects

```bash
# Terminal 1: Project A
cd /path/to/project-a
ccrouter code    # Uses .cc-modelrouter/config.json

# Terminal 2: Project B
cd /path/to/project-b
ccrouter code    # Uses different config
```

### Standalone Server

```bash
# Start server
ccrouter start

# In another terminal, use with Claude Code
export ANTHROPIC_BASE_URL=http://localhost:8081
claude

# When done
ccrouter stop --all
```

## Environment Variables

The `ccrouter code` command automatically sets:

| Variable | Value |
|----------|-------|
| `ANTHROPIC_BASE_URL` | `http://<host>:<port>` |

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 1 | General error |
| 2 | Configuration error |
| 3 | Server startup error |
