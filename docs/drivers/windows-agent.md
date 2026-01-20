# Windows Agent Documentation

The Windows agent (`metron-win-agent.exe`) enforces screen-time sessions on Windows PCs. It polls the Metron backend and locks the workstation when no active session exists.

## How It Works

1. Agent polls backend every 15 seconds (configurable)
2. Backend returns session status (active/inactive, time remaining, bypass mode)
3. Agent enforces based on status:
   - **No active session** → lock workstation
   - **Active session** → allow usage, show warning at 5 minutes remaining
   - **Bypass mode** → skip enforcement entirely
4. **Network errors** → lock after grace period (fail-closed security)

## Installation

### Prerequisites

- Windows 10 or Windows 11
- Metron backend with device configured (`driver: "passive"`)
- Network access to Metron backend

### Step 1: Build Release Package

On your development machine:

```bash
make release-win-agent
# Creates: bin/metron-win-agent.zip
```

### Step 2: Install on Windows

1. Copy `metron-win-agent.zip` to the Windows PC
2. Extract the zip file
3. Edit `config.txt` if needed (production values are pre-filled if config.json exists)
4. Double-click `install.bat`
5. Click "Yes" when prompted for administrator access

Done! The agent starts immediately and auto-starts at every user login.

### Release Package Contents

| File | Description |
|------|-------------|
| `metron-win-agent.exe` | Agent binary (runs hidden) |
| `config.txt` | Configuration (edit before install) |
| `install.bat` | Double-click to install |
| `install.ps1` | Installation script |
| `README.txt` | Quick reference |

## Configuration

Edit `config.txt` before running `install.bat`:

```ini
# Required
DEVICE_ID=win-pc1
TOKEN=your-agent-token
URL=https://metron.example.com

# Optional (uncomment to change)
# POLL_INTERVAL=15
# GRACE_PERIOD=30
# LOG_LEVEL=info
# LOG_FORMAT=json
```

| Setting | Required | Default | Description |
|---------|----------|---------|-------------|
| `DEVICE_ID` | Yes | - | Device ID registered in Metron |
| `TOKEN` | Yes | - | Agent token from device config |
| `URL` | Yes | - | Metron API base URL |
| `POLL_INTERVAL` | No | 15 | Polling interval (seconds) |
| `GRACE_PERIOD` | No | 30 | Lock delay on network error (seconds) |
| `LOG_LEVEL` | No | info | debug, info, warn, error |
| `LOG_FORMAT` | No | json | json or text |

## Backend Configuration

Add a device with passive driver to Metron `config.json`:

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

Generate a secure token:
```bash
openssl rand -base64 32
```

## File Locations

After installation:

| Path | Description |
|------|-------------|
| `C:\Program Files\Metron\metron-win-agent.exe` | Agent binary |
| `C:\ProgramData\Metron\agent.log` | Log file |

## Managing the Agent

### Check Status

```powershell
Get-ScheduledTask -TaskName MetronAgent
```

### View Logs

```powershell
Get-Content "C:\ProgramData\Metron\agent.log" -Tail 50
```

### Stop/Start

```powershell
Stop-ScheduledTask -TaskName MetronAgent
Start-ScheduledTask -TaskName MetronAgent
```

### Uninstall

```powershell
# Remove scheduled task
Unregister-ScheduledTask -TaskName "MetronAgent" -Confirm:$false

# Remove files (optional)
Remove-Item -Recurse "C:\Program Files\Metron"
Remove-Item -Recurse "C:\ProgramData\Metron"
```

### Update

1. Build new release package
2. Copy to Windows and extract
3. Run `install.bat` again (stops old agent, installs new one)

## Bypass Mode

Parents can temporarily disable enforcement via Telegram Bot (`/bypass` command) or API.

When bypass is active:
- Agent receives `bypass_mode: true` from backend
- No locking occurs regardless of session status
- Can be time-limited or indefinite

## Troubleshooting

### Agent Locks Immediately

1. Check an active session exists for this device in Metron
2. Verify `DEVICE_ID` matches the device in backend config
3. Check `TOKEN` matches `parameters.agent_token` in backend config
4. Test network: `curl https://your-metron-url/health`

### Network Errors

1. Check firewall allows outbound HTTPS to Metron server
2. Increase `GRACE_PERIOD` if network is unreliable
3. Check DNS resolution

### View Debug Logs

Edit `config.txt` and set:
```
LOG_LEVEL=debug
```

Then reinstall or restart the agent.

### Token Errors (401/403)

1. Verify token matches exactly between agent config and backend
2. Check `agent_enabled` is not `false` in backend device config
3. Ensure device ID matches

## Security

### Fail-Closed Design

If the agent cannot reach the backend, it locks the workstation after the grace period (default 30 seconds). This prevents bypassing enforcement by blocking network access.

### Token Security

- Each device has its own unique token
- Tokens can be disabled without deletion (`agent_enabled: false`)
- Agent can only query its own device's session status

## API Reference

The agent uses one endpoint:

```
GET /v1/agent/session?device_id=<device_id>
Authorization: Bearer <token>
```

Response:
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

See [API Documentation](/docs/api/v1.md#agent-endpoints) for details.
