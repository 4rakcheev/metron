# Logging Guide

Metron uses Go's built-in `slog` (structured logging) package for enterprise-grade logging.

## Features

- **Structured Logging**: All logs include contextual fields for easy parsing and querying
- **Multiple Formats**: JSON (production) and Text (development)
- **Log Levels**: Debug, Info, Warn, Error
- **stdout Output**: All logs go to stdout (not stderr) for proper log redirection
- **Zero Dependencies**: Uses Go 1.21+ standard library

## Configuration

### Command-line Flags

```bash
./bin/metron -log-format json -log-level info
```

- **`-log-format`**: `json` (default) or `text`
- **`-log-level`**: `debug`, `info` (default), `warn`, or `error`

### Examples

```bash
# Production: JSON format with INFO level
./bin/metron -log-format json -log-level info

# Development: Text format with DEBUG level
./bin/metron -log-format text -log-level debug

# Minimal logging: Only errors
./bin/metron -log-level error
```

## Output Examples

### JSON Format (Production)

```json
{"timestamp":"2025-12-09T15:30:45.123456Z","level":"INFO","msg":"Loading configuration","component":"main","use_env":false,"config_path":"config.json"}
{"timestamp":"2025-12-09T15:30:45.234567Z","level":"INFO","msg":"Initializing database","component":"main","path":"./metron.db"}
{"timestamp":"2025-12-09T15:30:45.345678Z","level":"INFO","msg":"Registering Aqara Cloud driver","component":"main","base_url":"https://open-ger.aqara.com","pin_scene":"AL.123","warn_scene":"AL.456","off_scene":"AL.789"}
{"timestamp":"2025-12-09T15:30:45.456789Z","level":"INFO","msg":"Starting session scheduler","component":"main","interval":"1m"}
{"timestamp":"2025-12-09T15:30:45.567890Z","level":"INFO","msg":"Scheduler started","component":"scheduler"}
{"timestamp":"2025-12-09T15:30:45.678901Z","level":"INFO","msg":"HTTP server starting","component":"main","host":"0.0.0.0","port":8080,"endpoint":"http://0.0.0.0:8080"}
```

### Text Format (Development)

```
timestamp=2025-12-09T15:30:45.123456Z level=INFO msg="Loading configuration" component=main use_env=false config_path=config.json
timestamp=2025-12-09T15:30:45.234567Z level=INFO msg="Initializing database" component=main path=./metron.db
timestamp=2025-12-09T15:30:45.345678Z level=INFO msg="Registering Aqara Cloud driver" component=main base_url=https://open-ger.aqara.com pin_scene=AL.123 warn_scene=AL.456 off_scene=AL.789
timestamp=2025-12-09T15:30:45.456789Z level=INFO msg="Starting session scheduler" component=main interval=1m
timestamp=2025-12-09T15:30:45.567890Z level=INFO msg="Scheduler started" component=scheduler
timestamp=2025-12-09T15:30:45.678901Z level=INFO msg="HTTP server starting" component=main host=0.0.0.0 port=8080 endpoint=http://0.0.0.0:8080
```

## Log Levels

### DEBUG
Verbose logging for troubleshooting. Includes detailed operational information.

```bash
./bin/metron -log-level debug
```

### INFO (Default)
Standard operational information. Suitable for production.

```bash
./bin/metron -log-level info
```

### WARN
Warnings and above. Use when you want to reduce log volume but still see issues.

```bash
./bin/metron -log-level warn
```

### ERROR
Only errors. Minimal logging for critical issues only.

```bash
./bin/metron -log-level error
```

## Component-Based Filtering

Every log message includes a `component` field to identify which service generated it. This allows easy filtering and troubleshooting.

### Available Components

- **`main`** - Application startup, configuration, initialization
- **`scheduler`** - Session scheduler (break enforcement, auto-expiry, warnings)
- **`api`** - REST API handlers and HTTP requests

### Filtering Examples

**Filter scheduler logs only:**
```bash
# JSON format with jq
./bin/metron -log-format json | jq 'select(.component == "scheduler")'

# Text format with grep
./bin/metron -log-format text | grep 'component=scheduler'
```

**Filter API logs only:**
```bash
./bin/metron -log-format json | jq 'select(.component == "api")'
```

**Filter multiple components:**
```bash
./bin/metron -log-format json | jq 'select(.component == "scheduler" or .component == "api")'
```

**Exclude main startup logs:**
```bash
./bin/metron -log-format json | jq 'select(.component != "main")'
```

## Integration with Log Aggregation

### JSON Format Benefits

The JSON format is designed for easy integration with log aggregation systems:

- **Elasticsearch/Kibana**: Direct JSON ingestion, filter by `component` field
- **Splunk**: JSON parsing for easy field extraction, search `component=scheduler`
- **CloudWatch Logs**: JSON fields automatically indexed, filter pattern `{ $.component = "api" }`
- **Datadog**: Structured attributes for filtering and alerting, facet on `component`
- **Loki**: Label extraction from JSON fields, query `{component="scheduler"}`

**Example Queries:**

- **Elasticsearch**: `component:scheduler AND level:ERROR`
- **Splunk**: `component="api" level="INFO"`
- **CloudWatch Logs Filter**: `{ $.component = "scheduler" && $.level = "ERROR" }`
- **Datadog**: `@component:api @level:INFO`
- **Loki (LogQL)**: `{component="scheduler"} |= "error"`

### Example: Systemd Journal

```bash
# Run metron with systemd, logs go to journalctl
journalctl -u metron -f -o json-pretty
```

### Example: File Output

```bash
# Redirect to file (all logs go to stdout)
./bin/metron > /var/log/metron.log 2>&1
```

### Example: Docker Logs

```bash
# Docker automatically captures stdout
docker logs metron -f
```

## Common Log Messages

### Startup
```json
{"timestamp":"...","level":"INFO","msg":"Loading configuration","use_env":false,"config_path":"config.json"}
{"timestamp":"...","level":"INFO","msg":"Initializing database","path":"./metron.db"}
{"timestamp":"...","level":"INFO","msg":"Registering Aqara Cloud driver","base_url":"...","pin_scene":"...","warn_scene":"...","off_scene":"..."}
{"timestamp":"...","level":"INFO","msg":"Initializing session manager"}
{"timestamp":"...","level":"INFO","msg":"Starting session scheduler","interval":"1m"}
{"timestamp":"...","level":"INFO","msg":"Initializing REST API server"}
{"timestamp":"...","level":"INFO","msg":"HTTP server starting","host":"0.0.0.0","port":8080,"endpoint":"http://0.0.0.0:8080"}
```

### Shutdown
```json
{"timestamp":"...","level":"INFO","msg":"Shutdown signal received","signal":"interrupt"}
{"timestamp":"...","level":"INFO","msg":"Stopping scheduler"}
{"timestamp":"...","level":"INFO","msg":"Shutting down HTTP server","timeout":"10s"}
{"timestamp":"...","level":"INFO","msg":"Graceful shutdown complete"}
```

### Errors
```json
{"timestamp":"...","level":"ERROR","msg":"Application failed","error":"failed to load config: config file not found"}
```

## Best Practices

1. **Production**: Use JSON format with INFO level
   ```bash
   ./bin/metron -log-format json -log-level info
   ```

2. **Development**: Use text format with DEBUG level
   ```bash
   ./bin/metron -log-format text -log-level debug
   ```

3. **Troubleshooting**: Temporarily enable DEBUG level
   ```bash
   ./bin/metron -log-level debug
   ```

4. **Log Rotation**: Use external tools like `logrotate` or Docker's log driver

5. **Monitoring**: Set up alerts on ERROR level logs in your log aggregation system
