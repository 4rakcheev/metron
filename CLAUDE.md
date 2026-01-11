# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Metron is a unified screen-time orchestrator - a centralized backend system for managing children's daily screen-time quotas across devices. Features TV control through Aqara Cloud API and Windows PC control through a native agent, with two main user interfaces:

- **Parent Interface**: Telegram Bot - primary management UI for parents to control sessions, view statistics, and manage children
- **Child Interface**: React PWA (web/children-control) - child-facing web app for PIN-based authentication and session management
- **Windows Agent**: Native agent for Windows PCs that enforces screen-time by locking the workstation

## Build Commands

```bash
make build              # Build all binaries (metron, metron-bot, aqara-test)
make build-metron       # Build main REST API server
make build-bot          # Build Telegram bot
make build-win-agent    # Build Windows agent (cross-compile)
make test               # Run all tests with -v
make test-coverage      # Generate HTML coverage report
make test-race          # Run with race detector
make fmt                # Format code
make vet                # Run go vet
make lint               # Run golangci-lint
go test ./internal/core -v  # Run specific package tests
```

## Running

```bash
./bin/metron                    # Run API server (reads config.json)
./bin/metron-bot -config bot-config.json  # Run Telegram bot
./bin/aqara-test -action pin    # Test Aqara integration (pin/warn/off)
./bin/metron-win-agent.exe -device-id win-pc1 -token xxx -url https://...  # Windows agent
```

## Architecture

### Device vs Driver Separation

- **Device**: User-facing entity (e.g., "Living Room TV") - defined in config with ID, name, type
- **Driver**: Control mechanism - implements `devices.DeviceDriver` interface
  - **Push-based** (e.g., Aqara Cloud): Backend actively controls device
  - **Pull-based** (e.g., Passive): Agent polls backend for session status
- Multiple devices can use the same driver with different parameters

### Key Packages

| Package | Purpose |
|---------|---------|
| `internal/core` | Domain models: Child, Session, BreakRule, DailyUsage, DeviceBypass, SessionManager |
| `internal/devices` | DeviceDriver interface definition |
| `internal/drivers/aqara` | Aqara Cloud API driver with token management (push-based) |
| `internal/drivers/passive` | No-op driver for agent-controlled devices (pull-based) |
| `internal/winagent` | Windows agent: enforcer, HTTP client, platform operations |
| `internal/api` | REST API: handlers, middleware (auth, agent_auth, requestid, recovery) |
| `internal/bot` | Telegram bot: flows, buttons, message formatting |
| `internal/storage/sqlite` | SQLite persistence for core models, driver tokens, device bypass |
| `internal/scheduler` | Session lifecycle: 1-minute interval checks, warnings, auto-expiry |

### Storage Pattern

Core `storage.Storage` interface handles domain models only. Driver-specific storage (e.g., `aqara.AqaraTokenStorage`) is defined in driver packages. SQLite implements both interfaces. This allows drivers to be added/removed without modifying core storage.

### API Route Pattern

Admin endpoints are conditionally registered based on available storage interfaces:
```go
if config.AqaraTokenStorage != nil {
    v1.POST("/admin/aqara/refresh-token", ...)
}
```

## User Interfaces

### Parent UI: Telegram Bot (`cmd/metron-bot`)

Primary management interface for parents. Uses webhooks for real-time updates with multi-step flows and inline buttons.

**Commands:**
- `/start` - Welcome and quick actions
- `/today` - View today's screen time summary for all children
- `/newsession` - Start new session (child → device → duration flow)
- `/extend` - Add time to active sessions
- `/children` - List all children with their limits
- `/devices` - List available devices

**Key features:** whitelist security (only authorized Telegram users), real-time usage stats, session management, bypass mode control.

### Child UI: React PWA (`web/children-control`)

Child-facing web application for PIN-based authentication and self-service session management.

```bash
cd web/children-control
npm install
npm run dev     # Dev server on :5173
npm run build   # Production build
npm run lint    # ESLint
```

**Tech stack:** React 18, TypeScript, Vite, Tailwind CSS, React Router
**Key features:** 4-digit PIN login, real-time session status, 30-second auto-refresh, offline-capable PWA, mobile-responsive

### Windows Agent (`cmd/metron-win-agent`)

Native Windows agent that enforces screen-time by locking the workstation when no active session exists.

**CLI flags:**
- `-device-id` (required): Device ID registered in Metron
- `-token` (required): Agent authentication token
- `-url` (required): Metron API base URL
- `-poll-interval` (default 15s): How often to poll backend
- `-grace-period` (default 30s): Grace period before locking on network error
- `-log-path`, `-log-level`, `-log-format`: Logging configuration

**Key features:**
- Polls `/v1/agent/session` endpoint for session status
- Locks workstation when no active session
- Shows warning notification at 5 minutes remaining
- Fail-closed security: locks after grace period on network errors
- Respects bypass mode for temporary enforcement suspension

See `docs/drivers/windows-agent.md` for full documentation.

## Deployment

Currently deployed to Ubuntu virtual server via GitHub Actions (`.github/workflows/deploy.yml`) with systemd services. No Docker/Kubernetes yet (planned for future when more services are needed).

**Deployment flow:**
1. Push to `master` triggers GitHub Actions
2. Build: runs tests, builds Go binaries (metron, metron-bot), builds child UI (npm)
3. Deploy: SCP binaries to server, update configs from secrets, restart systemd services

**Server structure:**
- `/opt/metron/` - Main API binary and config
- `/opt/metron-bot/` - Telegram bot binary and config
- `/opt/metron-child/dist/` - Child UI static files

**Systemd services:** `metron.service`, `metron-bot.service` (see `deploy/systemd/`)

**GitHub Secrets required:**
- `HOST`, `SSH_USER`, `SSH_KEY` - Server access
- `METRON_CONFIG_JSON`, `BOT_CONFIG_JSON` - App configurations
- `CHILD_UI_API_BASE` - API URL for child UI build

## Code Style

### Go Style
- Follow [Uber Go Style Guide](https://github.com/uber-go/guide/blob/master/style.md)
- Tabs for indentation, 140 char line length
- Local import prefix: `metron`
- Linting: golangci-lint with errcheck, govet, staticcheck, gocyclo (complexity 15), misspell (US)
- Tests excluded from gocyclo and errcheck

### REST API
- Use **Gin framework** for all HTTP API endpoints
- Follow **TMF630 REST API Design Guidelines** from TMForum for API design patterns

### Frontend
- JSON/YAML: 2-space indent
- TypeScript with strict mode

## Adding a New Driver

1. Create `internal/drivers/<name>/` package
2. Define driver-specific models and storage interface in the driver package
3. Implement `devices.DeviceDriver` interface
4. Add storage methods to SQLite (implements driver's storage interface)
5. Register driver in `cmd/metron/main.go`
6. Add conditional API route registration if driver has admin endpoints

## Configuration

Two config files:
- `config.json` - Main API: server, database, security, devices array, aqara settings
- `bot-config.json` - Bot: server port, telegram token/webhook, metron API connection

Key configuration sections:
- `devices[].parameters.agent_token`: Per-device Bearer tokens for agent authentication
- `devices[].parameters.agent_enabled`: Set to `false` to disable agent without removing token
- `devices[].driver`: Use "passive" for agent-controlled devices, "aqara" for push-controlled

Device IDs must be ≤15 characters (Telegram callback data limit).

## Documentation

- `docs/ARCHITECTURE.md` - Detailed system design, storage patterns, Windows agent architecture
- `docs/api/v1.md` - Complete REST API reference (including agent and bypass endpoints)
- `docs/api/openapi.yaml` - OpenAPI 3.0 specification
- `docs/drivers/aqara-tokens.md` - Aqara token management details
- `docs/drivers/windows-agent.md` - Windows agent installation and configuration
- `deploy/systemd/` - Production deployment with systemd

### Documentation Maintenance Rules

**IMPORTANT:** When making code changes, always keep documentation in sync:

1. **API Changes** - When adding, modifying, or removing API endpoints:
   - Update `docs/api/openapi.yaml` with the endpoint definition, request/response schemas
   - Update `docs/api/v1.md` with usage examples
   - Include all HTTP methods, parameters, request bodies, and response codes

2. **Configuration Changes** - When modifying config structures:
   - Update `config.example.json` with examples
   - Update relevant documentation (CLAUDE.md, driver docs)

3. **New Features** - When adding significant new features:
   - Update `docs/ARCHITECTURE.md` if it affects system design
   - Create new documentation files in `docs/` if needed
   - Update `CLAUDE.md` project overview section

4. **Driver Changes** - When adding or modifying drivers:
   - Create/update driver-specific docs in `docs/drivers/`
   - Update `docs/ARCHITECTURE.md` driver section

**Verification:** After API changes, the OpenAPI spec should include all endpoints from `internal/api/router.go`.