---
name: profile
description: Switch route profiles during session
---

Switch the active route profile for ccrouter.

## Usage

/profile <name>

Switch to a different profile to use different routes/models.

## How It Works

1. Read the running instance metadata from `~/.cc-modelrouter/instances/*.json` to discover the port and admin token
2. Use `GET /_admin/profiles?token=<adminToken>` on the discovered port to list available profiles
3. Use `POST /_admin/profiles/switch` with the discovered port and token to switch to the requested profile

## Steps

If the user provides a profile name (e.g., `/profile fast`):

1. Find the running ccrouter instance:
   ```bash
   INSTANCE=$(ls -t ~/.cc-modelrouter/instances/*.json 2>/dev/null | head -1)
   ```
2. Extract the port and admin token from the instance file:
   ```bash
   PORT=$(cat "$INSTANCE" | grep -o '"port":[[:space:]]*[0-9]*' | grep -o '[0-9]*')
   TOKEN=$(cat "$INSTANCE" | grep -o '"adminToken":[[:space:]]*"[^"]*"' | grep -o '"[^"]*"$' | tr -d '"')
   ```
3. Switch profile using the admin API:
   ```bash
   curl -s -X POST "http://localhost:$PORT/_admin/profiles/switch" -H "Content-Type: application/json" -H "X-Admin-Token: $TOKEN" -d "{\"profile\":\"<profile_name>\"}"
   ```

If the user runs `/profile` without arguments, or asks to list profiles:

1. Find the running ccrouter instance (same as above)
2. List available profiles:
   ```bash
   curl -s "http://localhost:$PORT/_admin/profiles?token=$TOKEN"
   ```
3. Show the user the list of available profiles and which one is currently active

## Important

- Always discover the port and token from the instance metadata file — never hardcode them
- The instance file is JSON with fields like `port` and `adminToken`
- After switching, confirm the change to the user
