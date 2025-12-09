# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- Core domain models (Child, Session, DailyUsage)
- SQLite storage layer with foreign key constraints
- Session manager with multi-child support
- Generic scheduler with break rules and auto-expiry
- Aqara Cloud driver with scene triggering (API v3.0)
- Device driver registry for pluggable drivers
- REST API with authentication
- Configuration system (file + environment variables)
- Aqara test CLI tool
- **Main application entry point** (cmd/metron)
  - Database initialization
  - Driver registration and management
  - Scheduler lifecycle control
  - REST API server with graceful shutdown
  - Command-line flags for config and logging
- **Structured logging with slog** (complete migration)
  - JSON format for production (default)
  - Text format for development
  - Configurable log levels (debug, info, warn, error)
  - All logs to stdout (not stderr)
  - Migrated all services: main, scheduler, API handlers
  - Structured fields for easy parsing and querying
  - Component-based filtering (component=main|scheduler|api)
  - Integration examples for log aggregation systems
- Comprehensive test suite (48/48 tests passing)
- Complete documentation (README, ARCHITECTURE, API, TESTING, LOGGING, CHANGELOG)
- Development tooling (Makefile, .editorconfig, .golangci.yml)

### Fixed
- `.gitignore` no longer ignores source code directories

### Planned
- Telegram bot integration
- Docker deployment
- Additional device drivers (PS5, Android, iPad)

## [0.1.0] - 2025-12-01

### Added
- Initial project structure
- TDD-first implementation approach
- MVP feature set complete (except Telegram bot)
