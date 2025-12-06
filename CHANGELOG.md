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
- Aqara Cloud driver with scene triggering
- Device driver registry for pluggable drivers
- REST API with authentication
- Configuration system (file + environment variables)
- Aqara test CLI tool
- Comprehensive test suite (48/48 tests passing)
- Project documentation (README, TESTING, CHANGELOG)
- Development tooling (Makefile, .editorconfig, .golangci.yml)

### Planned
- Telegram bot integration
- Main application entry point
- Docker deployment
- Additional device drivers (PS5, Android, iPad)

## [0.1.0] - 2025-12-01

### Added
- Initial project structure
- TDD-first implementation approach
- MVP feature set complete (except Telegram bot)
