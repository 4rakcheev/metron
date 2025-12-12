# Metron - Unified Screen-Time Orchestrator

Metron is a centralized backend orchestrator that manages children's daily screen-time quotas across various devices. The MVP focuses on TV control through Aqara Cloud API with Telegram-based parent interface.

## Features

- ✅ **Multi-child support** with individual daily time limits
- ✅ **Weekday/weekend scheduling** with different limits
- ✅ **Shared sessions** - multiple children can watch together
- ✅ **Break rules** - mandatory breaks after continuous usage
- ✅ **Auto-expiry** - sessions stop automatically when time runs out
- ✅ **Warnings** - notifications before session ends
- ✅ **Aqara Cloud integration** - control smart home scenes
- ✅ **REST API** - programmatic control with token authentication
- ✅ **Telegram bot** - parent control interface with multi-step flows

## Architecture

```
metron/
├── cmd/
│   ├── aqara-test/      # CLI tool for testing Aqara integration
│   ├── metron/          # Main REST API application
│   └── metron-bot/      # Telegram bot application
├── config/              # Configuration management
├── internal/
│   ├── api/             # REST API handlers
│   ├── bot/             # Telegram bot handlers and flows
│   ├── core/            # Domain models and business logic
│   ├── devices/         # Device driver interface
│   ├── drivers/
│   │   ├── aqara/       # Aqara Cloud driver
│   │   └── registry.go  # Driver registry
│   ├── scheduler/       # Generic session scheduler
│   └── storage/
│       └── sqlite/      # SQLite persistence layer
└── tests/               # Integration tests
```

## Tech Stack

- **Language**: Go 1.22
- **Database**: SQLite3
- **Cloud API**: Aqara Cloud
- **Messaging**: Telegram Bot API
- **Testing**: testify, 48/48 tests passing

## Quick Start

### Prerequisites

- Go 1.22 or higher
- Aqara developer account and credentials
- Telegram bot token (optional for now)

### 1. Install Dependencies

```bash
go mod download
```

### 2. Configure

Create `config.json` from the template:

```bash
cp config.example.json config.json
```

Edit `config.json` with your credentials:
- Aqara: `app_id`, `app_key`, `key_id`, scene IDs
- Telegram: bot token, webhook settings
- Security: API key for REST endpoints

### 3. Test Aqara Integration

```bash
# Build test tool
make build-aqara-test

# Test PIN entry scene
./bin/aqara-test -action pin

# Test warning scene
./bin/aqara-test -action warn

# Test power-off scene
./bin/aqara-test -action off
```

See [docs/TESTING.md](docs/TESTING.md) for detailed testing instructions.

### 4. Run Tests

```bash
# Run all tests
make test

# Run tests with coverage
make test-coverage

# Run specific package tests
go test ./internal/core -v
```

### 5. Build

```bash
# Build all binaries
make build

# Build specific binary
go build -o bin/metron ./cmd/metron
```

### 6. Run Application

```bash
# Run with default settings (JSON logs, INFO level)
./bin/metron

# Or use make
make run-metron

# Configure logging format and level
./bin/metron -log-format json -log-level info    # JSON format (default, best for production)
./bin/metron -log-format text -log-level debug   # Human-readable text format
./bin/metron -log-level debug                     # Enable debug logging
./bin/metron -log-level error                     # Only errors and above

# Use custom config file
./bin/metron -config /path/to/config.json

# Load config from environment variables
./bin/metron -env

# Production example
./bin/metron -config /etc/metron/config.json -log-format json -log-level info
```

**Command-line Flags:**
- **`-config string`**: Path to configuration file (default: `config.json`)
- **`-env`**: Load configuration from environment variables instead of file
- **`-log-format string`**: Output format - `json` (default) or `text`
  - `json` - Structured JSON logs, best for production and log aggregation systems
  - `text` - Human-readable text format, best for local development
- **`-log-level string`**: Minimum log level - `debug`, `info` (default), `warn`, or `error`

**What happens on startup:**
- Initializes SQLite database
- Registers device drivers (Aqara Cloud)
- Starts session scheduler (1-minute intervals)
- Starts REST API server
- All logs written to **stdout** (not stderr)
- Handles graceful shutdown on SIGINT/SIGTERM

## Configuration

Metron uses a modular device architecture that separates devices (user-facing entities) from drivers (control mechanisms). See [CONFIG.md](CONFIG.md) for comprehensive configuration guide.

### File-based Configuration

```json
{
  "server": {
    "host": "0.0.0.0",
    "port": 8080
  },
  "database": {
    "path": "./metron.db"
  },
  "security": {
    "api_key": "your-secret-api-key"
  },
  "devices": [
    {
      "id": "tv1",
      "name": "Living Room TV",
      "type": "tv",
      "driver": "aqara",
      "parameters": {
        "pin_scene_id": "custom-scene-for-this-tv"
      }
    }
  ],
  "aqara": {
    "app_id": "your-app-id",
    "app_key": "your-app-key",
    "key_id": "your-key-id",
    "scenes": {
      "tv_pin_entry": "default-scene-id",
      "tv_warning": "default-warning-scene",
      "tv_power_off": "default-off-scene"
    }
  }
}
```

**Key Changes:**
- Devices defined in global `devices` array
- Each device references a `driver` for control
- Optional `parameters` for device-specific driver overrides
- Driver defaults in driver-specific sections (e.g., `aqara.scenes`)

### Environment Variables

All configuration can be set via environment variables:

```bash
export METRON_HOST="0.0.0.0"
export METRON_PORT="8080"
export METRON_DB_PATH="./metron.db"
export METRON_API_KEY="your-secret-key"
export METRON_AQARA_APP_ID="your-app-id"
export METRON_AQARA_APP_KEY="your-app-key"
export METRON_AQARA_KEY_ID="your-key-id"
export METRON_AQARA_TV_PIN_SCENE="scene-id-1"
export METRON_AQARA_TV_WARNING_SCENE="scene-id-2"
export METRON_AQARA_TV_POWEROFF_SCENE="scene-id-3"
```

## Telegram Bot

Metron includes a fully-functional Telegram bot that provides a convenient parent interface for managing screen time. The bot uses webhooks for real-time updates and features multi-step flows with inline buttons.

### Features

- 📊 **Today's Stats** - View real-time usage for all children
- ➕ **New Session** - Multi-step flow (child → device → duration)
- ⏱ **Extend Session** - Add time to active sessions
- 🔒 **Whitelist Security** - Only authorized users can access
- 👶 **Manage Children** - View configured children and limits
- 📺 **View Devices** - List available device types

### Quick Start

See [deploy/bot/README.md](deploy/bot/README.md) for detailed setup instructions.

```bash
# Build bot
make build-bot

# Configure
cp bot-config.example.json bot-config.json
# Edit bot-config.json with your settings
# Set server.port (default: 8081)
# Set telegram.token, allowed_users, webhook_url
# Set metron.base_url and api_key

# Run
./bin/metron-bot -config bot-config.json
```

### Commands

| Command | Description |
|---------|-------------|
| `/start` | Show welcome and quick actions |
| `/today` | View today's screen time summary |
| `/newsession` | Start new session (3-step flow) |
| `/extend` | Extend active session |
| `/children` | List all children |
| `/devices` | List available devices |

## REST API

Metron provides a comprehensive REST API v1 following TMF630 guidelines. All `/v1/*` endpoints require `X-Metron-Key` header for authentication.

### Quick Examples

**Get today's statistics:**
```bash
curl -H "X-Metron-Key: your-api-key" \
  http://localhost:8080/v1/stats/today
```

**Start a session:**
```bash
curl -X POST http://localhost:8080/v1/sessions \
  -H "X-Metron-Key: your-api-key" \
  -H "Content-Type: application/json" \
  -d '{
    "device_id": "tv1",
    "child_ids": ["child-uuid"],
    "minutes": 30
  }'
```

**Extend a session:**
```bash
curl -X PATCH http://localhost:8080/v1/sessions/{session-id} \
  -H "X-Metron-Key: your-api-key" \
  -H "Content-Type: application/json" \
  -d '{
    "action": "extend",
    "additional_minutes": 15
  }'
```

**Stop a session:**
```bash
curl -X PATCH http://localhost:8080/v1/sessions/{session-id} \
  -H "X-Metron-Key: your-api-key" \
  -H "Content-Type: application/json" \
  -d '{"action": "stop"}'
```

### Complete API Documentation

📖 **[docs/api/v1.md](docs/api/v1.md)** - Complete human-readable API documentation with Telegram bot integration examples

📋 **[docs/api/openapi.yaml](docs/api/openapi.yaml)** - Full OpenAPI 3.0 specification

**Available Endpoints:**
- `GET /health` - Health check (no auth required)
- `GET /v1/children` - List all children
- `GET /v1/children/:id` - Get child with today's stats
- `GET /v1/devices` - List available devices
- `GET /v1/sessions` - List sessions (with filters)
- `POST /v1/sessions` - Start new session
- `GET /v1/sessions/:id` - Get session details
- `PATCH /v1/sessions/:id` - Extend or stop session
- `GET /v1/stats/today` - Today's statistics

**View OpenAPI Spec:**
```bash
# Using Swagger UI (docker)
docker run -p 8081:8080 -e SWAGGER_JSON=/openapi.yaml \
  -v $(pwd)/docs/api/openapi.yaml:/openapi.yaml \
  swaggerapi/swagger-ui

# Or use online editor
# Visit: https://editor.swagger.io/
# Import the docs/api/openapi.yaml file
```

## Development

### Project Structure

- **cmd/** - Application entry points
- **config/** - Configuration loading and validation
- **internal/api/** - HTTP handlers and routing
- **internal/core/** - Business logic and domain models
- **internal/devices/** - Device driver interface
- **internal/drivers/** - Device driver implementations
- **internal/scheduler/** - Session lifecycle management
- **internal/storage/** - Data persistence layer
- **internal/telegram/** - Telegram bot integration

### Adding a New Device Driver

1. Implement the `devices.DeviceDriver` interface:

```go
type DeviceDriver interface {
    Name() string
    StartSession(ctx context.Context, session *core.Session) error
    StopSession(ctx context.Context, session *core.Session) error
    ApplyWarning(ctx context.Context, session *core.Session, minutesRemaining int) error
    GetLiveState(ctx context.Context, deviceID string) (*DeviceState, error)
}
```

2. Register the driver in the registry
3. Add configuration support in `config/config.go`
4. Write tests for the new driver

### Running Tests

```bash
# All tests
make test

# Specific package
go test ./internal/core -v

# With coverage
make test-coverage
open coverage.html

# Race detection
go test -race ./...
```

### Code Quality

```bash
# Format code
make fmt

# Lint
make lint

# Vet
make vet
```

## Roadmap

### MVP (Current)
- [x] Core domain models
- [x] SQLite storage
- [x] Session management
- [x] Generic scheduler
- [x] Multi-device quota sharing
- [x] Aqara Cloud driver
- [x] Smart TV controls through Aqara (Samsung, LG, etc.)
- [x] REST API
- [x] Aqara test CLI
- [x] Telegram bot
- [x] Usage analytics
- [x] Main application

### Future Enhancements
- [ ] PS5 driver (presence detection + shutdown)
- [ ] Android Family Link driver
- [ ] iPad Kidslox driver
- [ ] Web application for kids
- [ ] Kids can request Extend time (push to telegram)
- [ ] Docker deployment

### Security & Code Quality Roadmap
- [ ] Code Quality Foundation (golangci-lint: govet, staticcheck, revive, errcheck, etc)
- [ ] SAST Layer (gosec, semgrep)
- [ ] Extended Security Checks (gitleaks, trivy fs, osv-scanner)
- [ ] Security Gate & SBOM (GitHub CodeQL, CycloneDX, Security Gate)

## Contributing

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Write tests for your changes
4. Ensure all tests pass (`make test`)
5. Commit your changes (`git commit -m 'Add amazing feature'`)
6. Push to the branch (`git push origin feature/amazing-feature`)
7. Open a Pull Request

## License

This is a personal project. All rights reserved.

## Acknowledgments

- Built with TDD principles - 48/48 tests passing
- Designed for extensibility and modularity
- Follows Go best practices and idioms

## Support

For issues and questions, please open an issue on GitHub.

---

**Note**: This is an MVP. The Telegram integration and main application are currently in development.
