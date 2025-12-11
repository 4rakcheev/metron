# Aqara Cloud API Token Management

This document explains how to manage Aqara Cloud API tokens in Metron. The token management system automatically refreshes access tokens and handles token expiration.

## Overview

Aqara Cloud API uses two types of tokens:

- **Refresh Token**: Valid for ~1 month, used to obtain access tokens
- **Access Token**: Valid for ~1 week, used for API requests

Metron automatically:
- Refreshes access tokens when they expire (stored in memory)
- Updates refresh tokens when they change (stored in database)
- Provides clear error messages when refresh tokens expire

## Initial Setup

### Step 1: Get Your Refresh Token

1. Go to the [Aqara Developer Console](https://developer.aqara.com/console/app-management/)
2. Navigate to **Application Details** → **Authorization Management** → **Aqara account authorization**
3. Enter your Aqara account credentials
4. Click **Obtain AccessToken**
5. Close the authorization page
6. Click **Authorization Details** for your application
7. Copy the **Refresh Token**

### Step 2: Store the Refresh Token in Metron

Use the admin API endpoint to store the refresh token:

```bash
curl -X POST http://localhost:8080/v1/admin/aqara/refresh-token \
  -H "X-Metron-Key: your-api-key" \
  -H "Content-Type: application/json" \
  -d '{
    "refresh_token": "YOUR_REFRESH_TOKEN_HERE"
  }'
```

**Response:**
```json
{
  "message": "Refresh token updated successfully",
  "updated_at": "2025-12-11T18:30:00Z"
}
```

### Step 3: Verify Token Status

Check the token status:

```bash
curl -X GET http://localhost:8080/v1/admin/aqara/token-status \
  -H "X-Metron-Key: your-api-key"
```

**Response (when configured):**
```json
{
  "configured": true,
  "refresh_token_updated_at": "2025-12-11T18:30:00Z",
  "access_token_status": "valid",
  "access_token_expires_in_seconds": 518400
}
```

**Response (when not configured):**
```json
{
  "configured": false,
  "message": "No refresh token configured. Use POST /v1/admin/aqara/refresh-token to add one."
}
```

## How It Works

### Automatic Token Refresh

1. **First Request**: When Metron makes its first Aqara API call:
   - Checks if access token exists in memory
   - If not, fetches refresh token from database
   - Calls Aqara API to get new access token
   - Stores access token in memory with expiry time
   - Updates both tokens in database

2. **Subsequent Requests**: For each Aqara API call:
   - Checks if cached access token is still valid
   - If valid, uses cached token (no API call needed)
   - If expired, automatically refreshes using steps above

3. **Token Updates**: When refreshing access token:
   - Aqara API returns both new access token AND new refresh token
   - Both are automatically saved to database
   - No manual intervention needed

### Token Storage

- **Refresh Token**: Stored in SQLite database (`aqara_tokens` table)
- **Access Token**: Stored in memory with expiry timestamp
- **Auto-cleanup**: Access token cache includes 5-minute safety buffer

## Error Handling

### No Refresh Token Configured

If you try to use Aqara driver without a refresh token:

```
Error: no refresh token configured - please add one using the admin API
```

**Solution**: Follow Step 1 and Step 2 above to configure a refresh token.

### Refresh Token Expired

If the refresh token expires (after ~1 month of inactivity):

```
Error: refresh token has expired - please update it manually
```

**Solution**: Get a new refresh token following the steps in "Initial Setup" above.

## Configuration

### Remove access_token from config.json

The `access_token` field is no longer used in `config.json`. Remove it if present:

```json
{
  "aqara": {
    "app_id": "your-app-id",
    "app_key": "your-app-key",
    "key_id": "your-key-id",
    "base_url": "https://open-cn.aqara.com",
    "scenes": {
      "tv_pin_entry": "scene-id-1",
      "tv_warning": "scene-id-2",
      "tv_power_off": "scene-id-3"
    }
  }
}
```

## Testing

### Using the aqara-test Tool

The `aqara-test` CLI tool now requires a refresh token:

```bash
# Build the test tool
make build-aqara-test

# Test with refresh token
./bin/aqara-test \
  -config config.json \
  -action pin \
  -refresh-token "YOUR_REFRESH_TOKEN"
```

**Actions:**
- `pin` - Trigger PIN entry scene (StartSession)
- `warn` - Trigger warning scene (ApplyWarning)
- `off` - Trigger power-off scene (StopSession)

## API Reference

### POST /v1/admin/aqara/refresh-token

Updates the Aqara refresh token in the database.

**Headers:**
- `X-Metron-Key`: Your API key
- `Content-Type`: application/json

**Request Body:**
```json
{
  "refresh_token": "string (required)"
}
```

**Response (200 OK):**
```json
{
  "message": "Refresh token updated successfully",
  "updated_at": "2025-12-11T18:30:00Z"
}
```

**Error Response (400 Bad Request):**
```json
{
  "error": "Invalid request body",
  "code": "INVALID_REQUEST",
  "details": "refresh_token is required"
}
```

### GET /v1/admin/aqara/token-status

Returns the current status of Aqara tokens.

**Headers:**
- `X-Metron-Key`: Your API key

**Response (200 OK - Configured):**
```json
{
  "configured": true,
  "refresh_token_updated_at": "2025-12-11T18:30:00Z",
  "access_token_status": "valid",
  "access_token_expires_in_seconds": 518400
}
```

**Access Token Status Values:**
- `not_cached` - No access token in memory yet
- `cached_no_expiry` - Token cached but no expiry info
- `expired` - Token expired, will refresh on next use
- `valid` - Token is valid and ready to use

## Troubleshooting

### Problem: "Failed to get access token: no refresh token configured"

**Cause**: No refresh token stored in database.

**Solution**: Use the admin API to store your refresh token (see Step 2 above).

### Problem: "Failed to get access token: refresh token has expired"

**Cause**: Refresh token is older than ~1 month.

**Solution**: Get a new refresh token from Aqara Developer Console (see Step 1 above) and update it via API (see Step 2).

### Problem: API calls fail with "401 Unauthorized"

**Cause**: Access token might be invalid or refresh failed.

**Solution**:
1. Check token status: `GET /v1/admin/aqara/token-status`
2. If refresh token expired, get and update new one
3. Check Aqara Developer Console for API key issues

### Problem: "API returned error code 106/107"

**Cause**: Refresh token is invalid or expired.

**Solution**: Get a new refresh token and update it via the admin API.

## Security Considerations

1. **API Key Protection**: The admin endpoints are protected by the same API key as other Metron endpoints. Keep your API key secure.

2. **Refresh Token Storage**: Refresh tokens are stored in the SQLite database. Ensure the database file has appropriate file permissions.

3. **Access Token in Memory**: Access tokens are only stored in memory and are never written to disk or logs.

4. **Token Rotation**: Both tokens are automatically rotated when refreshing, providing additional security.

## Migration from Old System

If you previously had `access_token` in your config:

1. Remove the `access_token` field from `config.json`
2. Get your current refresh token from Aqara Developer Console
3. Store it via the admin API
4. Restart Metron

The system will automatically handle token refresh from this point forward.
