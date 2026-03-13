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
  -c, --config string          Path to config file
      --log-destination string Log destination (file|stdout|stderr|path)
      --log-level string       Log level: debug, info, warn, error
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

# Enable debug logging to file
ccrouter code --log-level=debug --log-destination=file
```

---

### ccrouter start

Start the router server standalone.

```bash
ccrouter start [flags]
```

**Flags:**
```
  -c, --config string          Path to config file
  -p, --port int               Port to listen on (overrides config)
  -H, --host string            Host to bind to (overrides config)
      --log-destination string Log destination (file|stdout|stderr|path)
      --log-level string       Log level: debug, info, warn, error
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

# Start with debug logging to stdout
ccrouter start --log-level=debug --log-destination=stdout
```

---

### ccrouter stop

Stop a router instance.

```bash
ccrouter stop [instance-id] [flags]
```

**Arguments:**
```
  instance-id   ID of instance to stop (optional — stops all if omitted)
```

**Flags:**
```
  -f, --force   Force stop using SIGKILL instead of SIGTERM
```

**Description:**
- Stops the specified instance by PID
- Removes instance metadata file
- If no ID provided, stops all running instances

**Examples:**
```bash
# Stop specific instance
ccrouter stop inst_20250216_143022

# Stop all instances
ccrouter stop

# Force kill a stuck instance
ccrouter stop -f inst_20250216_143022
```

---

### ccrouter restart

Restart a router instance.

```bash
ccrouter restart [instance-id] [flags]
```

**Arguments:**
```
  instance-id   ID of instance to restart (optional — restarts all if omitted)
```

**Flags:**
```
  -c, --config string   Path to config file for restart
```

**Description:**
- Stops the instance
- Starts a new instance with the same configuration
- Reloads config from disk
- If no ID provided, restarts all running instances

**Examples:**
```bash
# Restart specific instance
ccrouter restart inst_20250216_143022

# Restart all instances
ccrouter restart
```

---

### ccrouter status

Show all running instances.

```bash
ccrouter status [flags]
```

**Flags:**
```
  -a, --all   Show all instances including dead ones
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
ccrouter clean [flags]
```

**Flags:**
```
  -a, --all   Remove all instance files including running ones
```

**Description:**
- Removes metadata files for instances that are no longer running
- Useful for cleanup after crashes or manual process termination
- Use `--all` with caution — stops and removes all instances

---

### ccrouter config

Configuration management commands.

```bash
ccrouter config [subcommand]
```

**Subcommands:**

#### ccrouter config show

Display the active configuration (API keys are masked).

```bash
ccrouter config show [-c /path/to/config.json]
```

**Description:**
- Displays the currently active configuration as JSON
- Shows whether using global or project-level config
- API keys are masked for security

#### ccrouter config path

Show the configuration file search paths.

```bash
ccrouter config path
```

#### ccrouter config init

Create a sample configuration file.

```bash
ccrouter config init [flags]
```

**Flags:**
```
  --global   Create in global location (~/.cc-modelrouter/config.json)
```

**Examples:**
```bash
# Create project-level config
ccrouter config init

# Create global config
ccrouter config init --global
```

---

### ccrouter logs

Show logs for an instance.

```bash
ccrouter logs [instance-id] [flags]
```

**Arguments:**
```
  instance-id   ID of instance (optional, shows all if not provided)
```

**Flags:**
```
  -f, --follow   Follow log output (like tail -f)
  -n, --tail int Number of lines to show from the end (default: 100)
```

**Examples:**
```bash
# Show all logs
ccrouter logs

# Show logs for specific instance
ccrouter logs inst_20250216_143022

# Follow logs in real-time
ccrouter logs -f inst_20250216_143022

# Show last 50 lines
ccrouter logs -n 50 inst_20250216_143022
```

---

### ccrouter usage

Show usage statistics for models.

```bash
ccrouter usage [instance-id] [period]
```

**Arguments:**
```
  instance-id   ID of instance (optional, shows all instances if not provided)
  period        Time period: all-time, today, this-week, last-week, this-month, last-month,
                this-quarter, last-year, or custom range YYYYMMDD-YYYYMMDD
```

**Examples:**
```bash
# Show all-time usage across all instances
ccrouter usage

# Show usage for specific instance
ccrouter usage inst_20250315_143022

# Show usage for specific period
ccrouter usage today
ccrouter usage this-week
ccrouter usage 20250301-20250315
```

**Description:**
- Displays token usage statistics per model, route, and instance
- Data is stored in SQLite at `~/.cc-modelrouter/usage.db`
- Uses buffered writes (500 records or 3 seconds) for performance

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
