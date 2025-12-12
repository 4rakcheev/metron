# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Fixed
- **Telegram Bot Navigation**
  - Fixed Back button in new session device selection step (now preserves child selection)
  - Fixed Back button in new session duration selection step (now preserves device selection)
  - Fixed session extension remaining time calculation (now accounts for elapsed time)
- **Session Extension**
  - Remaining minutes now correctly calculated from actual time instead of simple increment
  - API now returns accurate remaining time in extension response
- **Scheduler Session Management**
  - Fixed "driver not found" error preventing sessions from being stopped automatically
  - Scheduler now correctly looks up device before getting driver
  - Sessions will now properly expire and trigger device power-off
- **Device Parameters Architecture**
  - Fixed "device not found in context" error when scheduler stops sessions
  - Device parameters now embedded in Session object instead of passed via context
  - More explicit and maintainable architecture with no context magic
  - Device parameters persisted in database for session lifetime

### Added
- **Kidslox Device Driver (iPad Support)**
  - Full support for Kidslox parental control API
  - Device control: lock, unlock, time extension
  - Device-specific parameters: device_id, profile_id
  - Session lifecycle: start (unlock + set time), extend (add time), end (lock)
  - Comprehensive test coverage with HTTP mocks
  - ExtendableDriver interface for drivers supporting time extensions
- **Device/Driver Architecture Separation**
  - Global device registry with configurable devices
  - Device-specific driver parameters for customization
  - Device constraints: ID ≤15 chars for Telegram compatibility
  - Support for multiple devices using same driver
  - Device types for display/statistics vs drivers for control
- **Enhanced Configuration**
  - Device configuration in global `devices` array
  - Device-specific parameter overrides (`parameters` field)
  - Driver defaults with per-device customization
  - Comprehensive configuration documentation (CONFIG.md)
  - Updated example configurations with device architecture
- **Timezone Support for Telegram Bot**
  - Configurable timezone (IANA format, e.g., "Europe/Riga")
  - Time display formatting at presentation layer
  - Times stored in UTC, displayed in user's timezone
- **Session Management Improvements**
  - Immediate warning for short sessions (≤5 minutes)
  - Warning state reset on session extension
  - Single source of truth for time calculations
  - Comprehensive logging for session extension and scheduler
  - Debug logging for scheduler tick processing
- **Gin-based REST API v1** (TMF630 compliant)
  - Modular handler architecture (children, devices, sessions, stats, health)
  - Middleware stack (request ID, logging, recovery, content-type, auth)
  - All endpoints under `/v1/` prefix
  - Health check endpoint (no auth required)
  - Children endpoints: GET list, GET single with today's stats
  - Devices endpoint: GET list with capabilities
  - Sessions endpoints: GET list (filtered), POST create, GET single, PATCH update (extend/stop)
  - Stats endpoint: GET today's summary for all children
  - TMF630 error responses with standardized codes
  - Full OpenAPI 3.0 specification (openapi.yaml)
  - Comprehensive API documentation (API_V1.md)
  - Telegram bot ready endpoints
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

### Changed
- **API refactored from net/http to Gin framework**
  - Replaced monolithic handlers.go with modular handler structure
  - Improved error handling with standardized TMF630 responses
  - Added comprehensive middleware stack
  - Better separation of concerns following Uber Go Style Guide

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
