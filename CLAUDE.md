# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Metron is a unified screen-time orchestrator - a centralized backend system for managing children's daily screen-time quotas across devices. Features TV control through Aqara Cloud API with two main user interfaces:

- **Parent Interface**: Telegram Bot - primary management UI for parents to control sessions, view statistics, and manage children
- **Child Interface**: React PWA (web/children-control) - child-facing web app for PIN-based authentication and session management

## Build Commands

```bash
make build              # Build all binaries (metron, metron-bot, aqara-test)
make build-metron       # Build main REST API server
make build-bot          # Build Telegram bot
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
```

## Architecture

### Device vs Driver Separation

- **Device**: User-facing entity (e.g., "Living Room TV") - defined in config with ID, name, type
- **Driver**: Control mechanism (e.g., Aqara Cloud) - implements `devices.DeviceDriver` interface
- Multiple devices can use the same driver with different parameters

### Key Packages

| Package | Purpose |
|---------|---------|
| `internal/core` | Domain models: Child, Session, BreakRule, DailyUsage, SessionManager |
| `internal/devices` | DeviceDriver interface definition |
| `internal/drivers/aqara` | Aqara Cloud API driver with token management |
| `internal/api` | REST API: handlers, middleware (auth, requestid, recovery) |
| `internal/bot` | Telegram bot: flows, buttons, message formatting |
| `internal/storage/sqlite` | SQLite persistence for core models and driver tokens |
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

**Key features:** whitelist security (only authorized Telegram users), real-time usage stats, session management.

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

Device IDs must be ≤15 characters (Telegram callback data limit).

## Documentation

- `docs/ARCHITECTURE.md` - Detailed system design and storage patterns
- `docs/api/v1.md` - Complete REST API reference
- `docs/api/openapi.yaml` - OpenAPI 3.0 specification
- `docs/drivers/aqara-tokens.md` - Token management details
- `deploy/systemd/` - Production deployment with systemd