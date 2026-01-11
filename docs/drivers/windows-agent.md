# Windows Agent Documentation

The Windows agent (`metron-win-agent.exe`) is a screen-time enforcement agent that runs on Windows workstations. It polls the Metron backend for session status and locks the workstation when no active session exists.

## Overview

The Windows agent implements a **pull-based** enforcement model:

1. Agent polls backend every 15 seconds (configurable)
2. Backend returns session status (active/inactive, time remaining, bypass mode)
3. Agent enforces based on status:
   - No active session: lock workstation
   - Active session: allow usage, show warning at 5 minutes remaining
   - Bypass mode: skip enforcement entirely
4. Network errors: lock after grace period (fail-closed security)

## Installation

### Prerequisites

- Windows 10 or Windows 11
- Metron backend configured with:
  - Device using `"driver": "passive"`
  - Agent token in device `parameters.agent_token`
- Network access to Metron backend

### Build

Cross-compile from macOS or Linux:

```bash
make build-win-agent
# Produces: bin/metron-win-agent.exe
```

Or build directly on Windows:

```powershell
go build -o metron-win-agent.exe ./cmd/metron-win-agent
```

### Copy to Windows

Transfer `metron-win-agent.exe` to the target Windows machine.

## Configuration

The agent is configured entirely via command-line flags:

| Flag | Required | Default | Description |
|------|----------|---------|-------------|
| `-device-id` | Yes | - | Device ID registered in Metron |
| `-token` | Yes | - | Agent authentication token (Bearer token) |
| `-url` | Yes | - | Metron API base URL |
| `-poll-interval` | No | 15 | Polling interval in seconds |
| `-grace-period` | No | 30 | Grace period before locking on network error (seconds) |
| `-log-path` | No | stdout | Log file path |
| `-log-level` | No | info | Log level: debug, info, warn, error |
| `-log-format` | No | json | Log format: json or text |

### Example

```powershell
.\metron-win-agent.exe `
  -device-id "win-pc1" `
  -token "your-agent-token" `
  -url "https://metron.example.com" `
  -poll-interval 15 `
  -grace-period 30 `
  -log-path "C:\ProgramData\Metron\agent.log" `
  -log-level info
```

## Backend Configuration

Add a device with the passive driver and agent token to your Metron `config.json`:

```json
{
  "devices": [
    {
      "id": "win-pc1",
      "name": "Kids Windows PC",
      "type": "computer",
      "driver": "passive",
      "parameters": {
        "agent_token": "secure-random-token-here"
      }
    }
  ]
}
```

### Optional Parameters

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `agent_token` | string | - | Bearer token for agent authentication (required) |
| `agent_enabled` | bool | `true` | Set to `false` to disable the agent without removing the token |

### Generate a Secure Token

```bash
openssl rand -base64 32
```

## Running as a Windows Service

For production use, run the agent as a Windows service so it starts automatically.

### Using NSSM (Non-Sucking Service Manager)

1. Download NSSM from https://nssm.cc/

2. Install the service:

```powershell
nssm install MetronAgent "C:\Program Files\Metron\metron-win-agent.exe"
```

3. Configure arguments:

```powershell
nssm set MetronAgent AppParameters "-device-id win-pc1 -token your-token -url https://metron.example.com -log-path C:\ProgramData\Metron\agent.log"
```

4. Set the service to start automatically:

```powershell
nssm set MetronAgent Start SERVICE_AUTO_START
```

5. Start the service:

```powershell
nssm start MetronAgent
```

### Using Windows Task Scheduler

Alternatively, create a scheduled task that runs at user login:

1. Open Task Scheduler
2. Create Basic Task: "Metron Agent"
3. Trigger: At log on
4. Action: Start a program
5. Program: `C:\Program Files\Metron\metron-win-agent.exe`
6. Arguments: `-device-id win-pc1 -token your-token -url https://metron.example.com`

## Security Model

### Fail-Closed Design

The agent implements fail-closed security:

- If the agent cannot reach the backend, it will lock the workstation after the grace period
- Default grace period is 30 seconds
- This prevents bypassing enforcement by blocking network access

### Grace Period

The grace period prevents immediate lockout due to brief network interruptions:

1. Network error occurs
2. Timer starts counting
3. Subsequent polls continue to fail
4. After grace period expires, workstation is locked
5. Successful poll resets the timer

### Token Security

- Each device has its own agent token in the `parameters.agent_token` field
- Tokens can be disabled without deletion (set `parameters.agent_enabled: false`)
- Agent cannot query session status for other devices
- Token is sent as `Authorization: Bearer <token>` header

## Bypass Mode

Parents can temporarily disable enforcement for a device via:

1. **Telegram Bot**: Use `/bypass` command
2. **API**: `POST /v1/devices/:id/bypass`

### Bypass Options

| Duration | Description |
|----------|-------------|
| 1 hour | Bypass for 1 hour |
| 2 hours | Bypass for 2 hours |
| Until bedtime | Bypass until configured bedtime |
| Indefinite | Bypass until manually disabled |

When bypass is active:

- Agent receives `bypass_mode: true` in poll response
- Agent logs bypass status but skips enforcement
- No locking occurs regardless of session status
- Bypass can expire automatically or be manually cleared

### Clearing Bypass

Via API:

```bash
curl -X DELETE http://metron.example.com/v1/devices/win-pc1/bypass \
  -H "X-Metron-Key: your-api-key"
```

Via Telegram Bot: Use `/bypass` command and select "Disable".

## Logging

### Log Levels

| Level | Description |
|-------|-------------|
| debug | Detailed polling and state transitions |
| info | Session changes, warnings, locks |
| warn | Network errors, grace period |
| error | Failed operations, configuration errors |

### Log Fields

Logs include structured fields:

- `component`: Component name (main, enforcer, metron-client)
- `device_id`: Configured device ID
- `session_id`: Active session ID (when applicable)
- `error`: Error details (when applicable)

### Example Log Output

```json
{"time":"2025-01-11T10:00:00Z","level":"INFO","msg":"new session detected","component":"enforcer","session_id":"abc-123","ends_at":"2025-01-11T10:30:00Z"}
{"time":"2025-01-11T10:25:00Z","level":"INFO","msg":"warning threshold reached","component":"enforcer","session_id":"abc-123","remaining":"5m0s"}
{"time":"2025-01-11T10:30:00Z","level":"INFO","msg":"session expired, locking workstation","component":"enforcer","session_id":"abc-123"}
```

## Troubleshooting

### Agent Won't Start

1. Check required flags are provided:
   ```powershell
   .\metron-win-agent.exe -device-id "..." -token "..." -url "..."
   ```

2. Check URL is reachable:
   ```powershell
   curl https://metron.example.com/health
   ```

3. Check logs for configuration errors

### Agent Locks Immediately

1. Verify an active session exists for the device in Metron
2. Check device ID matches between agent and backend
3. Check token is correct and enabled
4. Check network connectivity to backend

### Network Errors / Grace Period Lockouts

1. Check backend is reachable: `curl <url>/health`
2. Check firewall allows outbound HTTPS
3. Increase grace period if network is unreliable: `-grace-period 60`
4. Check DNS resolution

### Token Errors (401/403)

1. Verify `parameters.agent_token` in device config matches agent `-token` flag exactly
2. Check `parameters.agent_enabled` is not set to `false` (defaults to `true`)
3. Check device ID matches between agent `-device-id` flag and device `id` in config
4. Check token is not expired or rotated

### Warning Notifications Not Showing

1. Check Windows notification settings
2. Run agent with elevated privileges for toast notifications
3. Check Focus Assist is not suppressing notifications

## API Reference

The agent uses a single endpoint:

### GET /v1/agent/session

**Query Parameters:**
- `device_id`: Device ID to query

**Headers:**
- `Authorization: Bearer <token>`

**Response:**

```json
{
  "active": true,
  "session_id": "session-uuid",
  "ends_at": "2025-01-11T10:30:00Z",
  "warn_at": "2025-01-11T10:25:00Z",
  "server_time": "2025-01-11T10:00:00Z",
  "bypass_mode": false
}
```

See [API Documentation](/docs/api/v1.md#agent) for full details.

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                     Windows Agent                            │
│                                                              │
│  ┌────────────┐    ┌────────────┐    ┌────────────────────┐ │
│  │   Main     │───>│  Enforcer  │───>│  MetronClient      │ │
│  │            │    │            │    │  (HTTP)            │ │
│  └────────────┘    └─────┬──────┘    └────────────────────┘ │
│                          │                                   │
│                          ▼                                   │
│                    ┌────────────┐                            │
│                    │  Platform  │                            │
│                    │  (Win API) │                            │
│                    └────────────┘                            │
│                          │                                   │
│                          ▼                                   │
│               LockWorkstation() / Toast                      │
└─────────────────────────────────────────────────────────────┘
```

### Components

| Component | File | Purpose |
|-----------|------|---------|
| Main | `cmd/metron-win-agent/main.go` | Entry point, flag parsing, setup |
| Config | `internal/winagent/config.go` | Configuration and validation |
| Enforcer | `internal/winagent/enforcer.go` | Enforcement loop and state machine |
| Client | `internal/winagent/client.go` | HTTP client for Metron API |
| Platform | `internal/winagent/platform.go` | Windows-specific operations |

### State Machine

```
                    ┌─────────┐
                    │  Start  │
                    └────┬────┘
                         │
                         ▼
              ┌──────────────────┐
              │   Poll Backend   │◄────────┐
              └────────┬─────────┘         │
                       │                   │
           ┌───────────┼───────────┐       │
           ▼           ▼           ▼       │
      ┌─────────┐ ┌─────────┐ ┌─────────┐  │
      │ Active  │ │Inactive │ │ Bypass  │  │
      │ Session │ │         │ │ Mode    │  │
      └────┬────┘ └────┬────┘ └────┬────┘  │
           │           │           │       │
           ▼           ▼           ▼       │
      ┌─────────┐ ┌─────────┐ ┌─────────┐  │
      │ Check   │ │  Lock   │ │  Skip   │  │
      │ Warning │ │Workstat.│ │ Enforce │  │
      └────┬────┘ └────┬────┘ └────┬────┘  │
           │           │           │       │
           └───────────┴───────────┴───────┘
                    (wait poll interval)
```
