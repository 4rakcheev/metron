# Metron Documentation

Welcome to the Metron documentation. This directory contains all technical documentation for the Metron screen-time orchestrator.

## Documentation Index

### Architecture & Design

- **[ARCHITECTURE.md](ARCHITECTURE.md)** - System architecture, modular design, and separation of concerns
- **[TESTING.md](TESTING.md)** - Testing guide and test coverage

### API Documentation

- **[api/](api/)** - Complete REST API documentation
  - [API Overview](api/README.md)
  - [API v1 Reference](api/v1.md) - Human-readable API documentation with examples
  - [OpenAPI Specification](api/openapi.yaml) - Machine-readable API spec

### Driver Documentation

- **[drivers/](drivers/)** - Driver-specific documentation
  - [Aqara Token Management](drivers/aqara-tokens.md) - How to manage Aqara Cloud API tokens

### Features

- **[features/](features/)** - Feature-specific documentation
  - [Shared Time](features/shared-time.md) - Multi-child shared session support

### Development

- **[development/](development/)** - Development guides and internal notes
  - [Git Commit Guide](development/git-commits.md) - Commit message conventions
  - [Logging Guide](development/logging.md) - Structured logging implementation
  - [Bot Implementation Notes](development/bot-implementation-notes.md) - Telegram bot implementation details
  - [Review Fixes](development/review-fixes.md) - Code review notes and fixes

## Quick Links

### Getting Started
- [Main README](../README.md) - Project overview and quick start
- [Configuration Examples](../config.example.json) - Main API configuration
- [Bot Configuration](../bot-config.example.json) - Telegram bot configuration

### Deployment
- [Deployment Guide](../deploy/systemd/README.md) - Systemd service deployment
- [Server Setup](../deploy/systemd/SERVER_SETUP.md) - Server preparation and user configuration
- [Bot Deployment](../deploy/bot/README.md) - Telegram bot deployment guide

### API Access
- [API v1 Documentation](api/v1.md) - Complete API reference
- [Health Check](http://localhost:8080/health) - Check if service is running
- [Swagger UI](https://editor.swagger.io/) - Interactive API explorer (import openapi.yaml)

## Documentation Standards

### File Naming
- Use lowercase with hyphens: `my-document.md`
- Be descriptive: `aqara-tokens.md` not `tokens.md`
- Group related docs in subdirectories

### Content Structure
1. **Title** - Clear, descriptive H1 heading
2. **Overview** - Brief description of what the document covers
3. **Main Content** - Detailed information with examples
4. **Links** - Related documentation and external resources

### Code Examples
- Include working, tested examples
- Show both request and response for API examples
- Include error cases where relevant

## Contributing to Documentation

When adding new features or making significant changes:

1. Update relevant documentation in this directory
2. Add examples and use cases
3. Update the main README.md if user-facing
4. Keep documentation in sync with code changes

## Need Help?

- Check the [main README](../README.md) for project overview
- Review [API documentation](api/v1.md) for API usage
- See [architecture docs](ARCHITECTURE.md) for system design
- Open an issue on GitHub for questions
