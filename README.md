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
- 🚧 **Telegram bot** - parent control interface (coming soon)

## Architecture

```
metron/
├── cmd/
│   ├── aqara-test/      # CLI tool for testing Aqara integration
│   └── metron/          # Main application (coming soon)
├── config/              # Configuration management
├── internal/
│   ├── api/             # REST API handlers
│   ├── core/            # Domain models and business logic
│   ├── devices/         # Device driver interface
│   ├── drivers/
│   │   ├── aqara/       # Aqara Cloud driver
│   │   └── registry.go  # Driver registry
│   ├── scheduler/       # Generic session scheduler
│   ├── storage/
│   │   └── sqlite/      # SQLite persistence layer
│   └── telegram/        # Telegram bot (coming soon)
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

See [TESTING.md](TESTING.md) for detailed testing instructions.

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

## Configuration

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
  "aqara": {
    "app_id": "your-app-id",
    "app_key": "your-app-key",
    "key_id": "your-key-id",
    "scenes": {
      "tv_pin_entry": "scene-id-1",
      "tv_warning": "scene-id-2",
      "tv_power_off": "scene-id-3"
    }
  }
}
```

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

## REST API

All endpoints require `X-Metron-Key` header for authentication.

### Start TV Session

```bash
POST /sessions/tv/start
Content-Type: application/json
X-Metron-Key: your-api-key

{
  "child_ids": ["child1", "child2"],
  "minutes": 30
}
```

### Extend Session

```bash
POST /sessions/{id}/extend
Content-Type: application/json
X-Metron-Key: your-api-key

{
  "additional_minutes": 15
}
```

### Stop Session

```bash
POST /sessions/{id}/stop
X-Metron-Key: your-api-key
```

### Get Session Status

```bash
GET /sessions/{id}
X-Metron-Key: your-api-key
```

### Get Active Sessions

```bash
GET /status
X-Metron-Key: your-api-key
```

### List Children

```bash
GET /children
X-Metron-Key: your-api-key
```

### Get Child Status

```bash
GET /children/{id}/status
X-Metron-Key: your-api-key
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
- [x] Aqara Cloud driver
- [x] REST API
- [x] Aqara test CLI
- [ ] Telegram bot
- [ ] Main application
- [ ] Docker deployment

### Future Enhancements
- [ ] PS5 driver (presence detection + shutdown)
- [ ] Android Family Link driver
- [ ] iPad Kidslox driver
- [ ] Smart TV drivers (Samsung, LG, etc.)
- [ ] Browser plugin for usage tracking
- [ ] Web dashboard
- [ ] Multi-device quota sharing
- [ ] Usage analytics and reports
- [ ] Notifications (email, SMS, push)

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
