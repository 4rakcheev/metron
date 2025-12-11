# Metron REST API Documentation

This directory contains the complete REST API documentation for Metron.

## Files

- **[v1.md](v1.md)** - Human-readable API v1 documentation with examples
- **[openapi.yaml](openapi.yaml)** - OpenAPI 3.0 specification for API v1

## Quick Links

### API Endpoints

- `GET /health` - Health check (no authentication)
- `GET /v1/children` - List all children
- `POST /v1/children` - Create a new child
- `GET /v1/sessions` - List sessions (with filters)
- `POST /v1/sessions` - Start a new session
- `GET /v1/stats/today` - Today's statistics
- `POST /v1/admin/aqara/refresh-token` - Update Aqara refresh token
- `GET /v1/admin/aqara/token-status` - Check Aqara token status

See [v1.md](v1.md) for complete documentation with request/response examples.

## Authentication

All `/v1/*` endpoints require the `X-Metron-Key` header:

```bash
curl -H "X-Metron-Key: your-api-key" http://localhost:8080/v1/children
```

## Viewing the OpenAPI Specification

### Using Swagger UI (Docker)

```bash
docker run -p 8081:8080 \
  -e SWAGGER_JSON=/openapi.yaml \
  -v $(pwd)/openapi.yaml:/openapi.yaml \
  swaggerapi/swagger-ui
```

Then open http://localhost:8081

### Using Swagger Editor Online

1. Visit https://editor.swagger.io/
2. Import the [openapi.yaml](openapi.yaml) file

## API Versioning

Currently, API v1 is the only version. Future versions will be added with appropriate versioning (v2, v3, etc.).
