# Documentation Map

This document shows where to find all documentation after the reorganization.

## Root Directory (Minimal)

```
metron/
├── README.md                    # Main entry point - start here
├── CHANGELOG.md                 # Project changelog
├── config.example.json          # Main API config template
└── bot-config.example.json      # Bot config template
```

## Documentation (`docs/`)

### Core Documentation

```
docs/
├── README.md                    # Documentation index and navigation
├── ARCHITECTURE.md              # System architecture and design principles
└── TESTING.md                   # Testing guide
```

### API Documentation (`docs/api/`)

```
docs/api/
├── README.md                    # API overview and quick links
├── v1.md                        # Complete API v1 reference with examples
└── openapi.yaml                 # OpenAPI 3.0 specification
```

### Driver Documentation (`docs/drivers/`)

```
docs/drivers/
└── aqara-tokens.md              # Aqara Cloud API token management guide
```

### Feature Documentation (`docs/features/`)

```
docs/features/
├── downtime.md                  # Downtime schedules and skip functionality
└── shared-time.md               # Multi-child shared session feature
```

### Development Documentation (`docs/development/`)

```
docs/development/
├── git-commits.md               # Git commit conventions
├── logging.md                   # Structured logging guide
├── bot-implementation-notes.md  # Telegram bot implementation details
└── review-fixes.md              # Code review notes
```

## Deployment (`deploy/`)

### Bot Deployment (`deploy/bot/`)

```
deploy/bot/
└── README.md                    # Telegram bot deployment guide
```

### Systemd Deployment (`deploy/systemd/`)

```
deploy/systemd/
├── README.md                    # Systemd service installation and management
├── SERVER_SETUP.md              # Server preparation and user setup
├── metron.service               # Main API systemd service file
└── metron-bot.service           # Telegram bot systemd service file
```

## Quick Reference

### I Want To...

**...understand the system architecture**
→ [docs/ARCHITECTURE.md](ARCHITECTURE.md)

**...use the REST API**
→ [docs/api/v1.md](api/v1.md) or [docs/api/README.md](api/README.md)

**...deploy Metron**
→ [deploy/systemd/README.md](../deploy/systemd/README.md)

**...deploy the Telegram bot**
→ [deploy/bot/README.md](../deploy/bot/README.md)

**...manage Aqara tokens**
→ [docs/drivers/aqara-tokens.md](drivers/aqara-tokens.md)

**...run tests**
→ [docs/TESTING.md](TESTING.md)

**...understand shared sessions**
→ [docs/features/shared-time.md](features/shared-time.md)

**...configure downtime schedules**
→ [docs/features/downtime.md](features/downtime.md)

**...contribute code**
→ [docs/development/git-commits.md](development/git-commits.md)

**...set up logging**
→ [docs/development/logging.md](development/logging.md)

## Before and After

### Before Reorganization ❌

```
metron/
├── README.md
├── API_V1.md
├── ARCHITECTURE.md
├── AQARA_TOKEN_MANAGEMENT.md
├── BOT_IMPLEMENTATION.md
├── BOT_README.md
├── CHANGELOG.md
├── GIT_COMMIT_GUIDE.md
├── LOGGING.md
├── REVIEW_FIXES.md
├── SHARED_TIME.md
├── TESTING.md
├── openapi.yaml
├── deploy/
│   ├── DEPLOY.md
│   ├── metron.service
│   └── metron-bot.service
└── ...
```

### After Reorganization ✅

```
metron/
├── README.md                           # Clean root
├── CHANGELOG.md
├── config.example.json
├── bot-config.example.json
├── docs/                               # All documentation
│   ├── README.md
│   ├── ARCHITECTURE.md
│   ├── TESTING.md
│   ├── api/                           # API docs
│   ├── drivers/                       # Driver docs
│   ├── features/                      # Feature docs
│   └── development/                   # Dev docs
└── deploy/                            # Deployment artifacts
    ├── bot/                           # Bot deployment
    └── systemd/                       # Systemd services
```

## Benefits of New Structure

1. **Clean Root** - Only essential files visible
2. **Organized** - Documentation grouped by purpose
3. **Discoverable** - Clear navigation with README files
4. **Maintainable** - Easy to find and update docs
5. **Scalable** - Can add new categories without cluttering
6. **Standard** - Follows Go project best practices

## Migration Notes

All documentation links in `README.md` have been updated to point to new locations.

If you have bookmarks or external links to old documentation paths, update them:
- `API_V1.md` → `docs/api/v1.md`
- `TESTING.md` → `docs/TESTING.md`
- `BOT_README.md` → `deploy/bot/README.md`
- `openapi.yaml` → `docs/api/openapi.yaml`
