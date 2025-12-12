# Metron Architecture

This document describes the modular architecture of Metron and the separation of concerns between components.

## Core Principles

1. **Separation of Concerns**: Each driver has its own domain models and storage interfaces
2. **Modularity**: Components can be removed without breaking the core system
3. **Interface-based Design**: Dependencies are injected through well-defined interfaces
4. **No Tight Coupling**: Storage layer doesn't depend on specific driver implementations

## Package Structure

```
metron/
├── internal/
│   ├── core/              # Core domain models (Child, Session, etc.)
│   ├── storage/           # Core storage interface
│   │   └── sqlite/        # SQLite implementation
│   ├── devices/           # Device driver interface
│   ├── drivers/           # Driver implementations
│   │   ├── aqara/         # Aqara Cloud driver
│   │   │   ├── aqara.go   # Driver implementation
│   │   │   └── tokens.go  # Aqara-specific models & storage interface
│   │   └── registry.go    # Driver registry
│   ├── api/               # REST API
│   │   ├── handlers/      # HTTP handlers
│   │   └── middleware/    # HTTP middleware
│   └── scheduler/         # Session scheduler
└── cmd/                   # Application entry points
```

## Storage Architecture

### Core Storage Interface

The `storage.Storage` interface defines persistence operations for **core domain models only**:

```go
type Storage interface {
    // Children operations
    CreateChild(ctx context.Context, child *core.Child) error
    GetChild(ctx context.Context, id string) (*core.Child, error)
    // ... other child operations

    // Session operations
    CreateSession(ctx context.Context, session *core.Session) error
    // ... other session operations

    // Daily Usage operations
    GetDailyUsage(ctx context.Context, childID string, date time.Time) (*core.DailyUsage, error)
    // ... other usage operations
}
```

**Key Design Decision**: Driver-specific storage needs (like Aqara tokens) are **NOT** part of this interface.

### Driver-Specific Storage

Each driver defines its own storage interface for driver-specific data:

**Example: Aqara Token Storage**

```go
// internal/drivers/aqara/tokens.go
package aqara

type AqaraTokens struct {
    RefreshToken         string
    AccessToken          string
    AccessTokenExpiresAt *time.Time
    CreatedAt            time.Time
    UpdatedAt            time.Time
}

type AqaraTokenStorage interface {
    GetAqaraTokens(ctx context.Context) (*AqaraTokens, error)
    SaveAqaraTokens(ctx context.Context, tokens *AqaraTokens) error
}
```

**Benefits**:
- Driver owns its own domain models
- Storage implementation can choose to implement driver interfaces
- Removing a driver doesn't affect core storage interface
- No circular dependencies

### Storage Implementation

The SQLite storage implements **both** the core Storage interface and driver-specific interfaces:

```go
// internal/storage/sqlite/sqlite.go
type SQLiteStorage struct {
    db *sql.DB
}

// Implements storage.Storage
func (s *SQLiteStorage) CreateChild(...) error { }

// Implements aqara.AqaraTokenStorage
func (s *SQLiteStorage) GetAqaraTokens(...) (*aqara.AqaraTokens, error) { }
func (s *SQLiteStorage) SaveAqaraTokens(...) error { }
```

## Device vs Driver Architecture

### Separation of Concerns

Metron separates **devices** (user-facing entities) from **drivers** (control mechanisms):

- **Device**: User-facing entity for display, statistics, and configuration
  - Example: "Living Room TV" (id: "tv1", type: "tv")
  - Multiple devices can use the same driver
  - Device-specific parameters override driver defaults

- **Driver**: Control mechanism implementing the protocol
  - Example: Aqara driver controlling multiple TVs via scenes
  - One driver can control multiple devices
  - Provides default configuration

### Device Registry

Devices are registered globally in configuration:

```json
{
  "devices": [
    {
      "id": "tv1",
      "name": "Living Room TV",
      "type": "tv",
      "driver": "aqara",
      "parameters": {
        "pin_scene_id": "custom-scene"
      }
    },
    {
      "id": "tv2",
      "name": "Bedroom TV",
      "type": "tv",
      "driver": "aqara"
    }
  ]
}
```

**Device Constraints:**
- ID must be ≤15 characters (Telegram callback data limit)
- ID must be unique across all devices
- Type used for display and statistics
- Parameters are optional driver-specific overrides

### Device Driver Interface

All drivers implement the `devices.DeviceDriver` interface:

```go
type DeviceDriver interface {
    Name() string
    StartSession(ctx context.Context, session *core.Session) error
    StopSession(ctx context.Context, session *core.Session) error
    ApplyWarning(ctx context.Context, session *core.Session, minutesRemaining int) error
    GetLiveState(ctx context.Context, deviceID string) (*DeviceState, error)
    Capabilities() DriverCapabilities
}
```

### Session Flow with Devices

1. User creates session with **device ID** (e.g., "tv1")
2. Session manager looks up device in registry
3. Gets driver name from device (e.g., "aqara")
4. Looks up driver in driver registry
5. Passes device parameters to driver (if any)
6. Driver uses device-specific or default parameters

### Aqara Driver Example

The Aqara driver is initialized with its specific dependencies:

```go
// cmd/metron/main.go
aqaraDriver := aqara.NewDriver(
    aqara.Config{...},
    db, // SQLiteStorage implements aqara.AqaraTokenStorage
)
driverRegistry.Register(aqaraDriver)
```

**Key Points**:
- Driver receives `aqara.AqaraTokenStorage` interface, not concrete type
- SQLite storage satisfies this interface
- Driver doesn't know or care about core Storage interface

## API Architecture

### Router Configuration

The API router accepts optional driver-specific dependencies:

```go
type RouterConfig struct {
    Storage           storage.Storage              // Core storage (required)
    Manager           *core.SessionManager         // Session manager (required)
    Registry          *drivers.Registry            // Driver registry (required)
    APIKey            string                       // API key (required)
    Logger            *slog.Logger                 // Logger (required)
    AqaraTokenStorage aqara.AqaraTokenStorage      // Aqara storage (optional)
}
```

### Conditional Route Registration

Admin endpoints are only registered if the corresponding storage is provided:

```go
// Only register Aqara admin endpoints if token storage is available
if config.AqaraTokenStorage != nil {
    adminHandler := handlers.NewAdminHandler(
        config.AqaraTokenStorage,
        config.Logger,
    )
    v1.POST("/admin/aqara/refresh-token", adminHandler.UpdateAqaraRefreshToken)
    v1.GET("/admin/aqara/token-status", adminHandler.GetAqaraTokenStatus)
}
```

**Benefits**:
- Removing Aqara driver = don't provide `AqaraTokenStorage`
- Admin endpoints won't be registered
- No runtime errors or broken routes

## Modularity in Practice

### Adding a New Driver

To add a new driver (e.g., "PS5"):

1. **Create driver package**: `internal/drivers/ps5/`
2. **Define driver-specific models** (if needed): `ps5/models.go`
3. **Define driver storage interface** (if needed): `ps5/storage.go`
4. **Implement driver**: `ps5/driver.go`
5. **Update SQLite** to implement PS5 storage interface (optional)
6. **Register driver** in `cmd/metron/main.go`
7. **Add admin endpoints** (if needed) with conditional registration

### Removing Aqara Driver

To completely remove Aqara support:

1. **Delete** `internal/drivers/aqara/` directory
2. **Remove** Aqara registration from `cmd/metron/main.go`
3. **Remove** `AqaraTokenStorage` from `RouterConfig`
4. **Remove** Aqara methods from SQLite (optional - won't break if kept)
5. **Remove** Aqara config from `config/config.go`

**Result**: Core system continues to work without any changes to:
- Core storage interface
- Core domain models
- Session management
- Other drivers
- Main application flow

## Benefits of This Architecture

1. **Clean Boundaries**: Each component has clear responsibilities
2. **No Circular Dependencies**: Proper dependency direction (driver → storage, not storage → driver)
3. **Easy Testing**: Mock driver-specific storage interfaces independently
4. **Flexible Deployment**: Can deploy with or without specific drivers
5. **Maintainable**: Changes to one driver don't affect others
6. **Extensible**: Adding new drivers doesn't modify core interfaces

## Anti-Patterns Avoided

❌ **Don't**: Add driver-specific methods to core Storage interface
```go
// BAD
type Storage interface {
    CreateChild(...) error
    GetAqaraTokens(...) (*AqaraTokens, error)  // Driver-specific!
    GetPS5Settings(...) (*PS5Settings, error)  // Another driver-specific!
}
```

✅ **Do**: Create separate interfaces for driver-specific needs
```go
// GOOD
type Storage interface {
    CreateChild(...) error
}

// In driver package
type AqaraTokenStorage interface {
    GetAqaraTokens(...) (*AqaraTokens, error)
}
```

❌ **Don't**: Put driver models in shared packages
```go
// BAD - storage/sqlite/models.go
type AqaraTokens struct { ... }  // Couples storage to specific driver
```

✅ **Do**: Keep driver models in driver packages
```go
// GOOD - drivers/aqara/tokens.go
type AqaraTokens struct { ... }
```

❌ **Don't**: Force all handlers to use all storage
```go
// BAD
func NewAdminHandler(storage storage.Storage, ...) {
    // Storage doesn't have Aqara methods!
}
```

✅ **Do**: Use specific interfaces for specific needs
```go
// GOOD
func NewAdminHandler(storage aqara.AqaraTokenStorage, ...) {
    // Clear dependency on what's actually needed
}
```

## Conclusion

This architecture ensures that Metron remains modular, maintainable, and extensible. Each driver is an independent module that can be added or removed without affecting the core system or other drivers.
