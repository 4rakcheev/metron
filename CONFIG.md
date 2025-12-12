# Metron Configuration Guide

## Configuration Structure

Metron uses a JSON configuration file with the following main sections:

### Server Configuration
```json
{
  "server": {
    "host": "0.0.0.0",
    "port": 8080
  }
}
```

### Database Configuration
```json
{
  "database": {
    "path": "./metron.db"
  }
}
```

### Security Configuration
```json
{
  "security": {
    "api_key": "your-secret-api-key-here",
    "allowed_ips": [],
    "enable_ip_check": false
  }
}
```

## Device Architecture

### Device Registry

Devices are defined in the global `devices` array. Each device represents a controllable entity (TV, phone, tablet, etc.) and references a driver for control.

```json
{
  "devices": [
    {
      "id": "tv1",
      "name": "Living Room TV",
      "type": "tv",
      "driver": "aqara",
      "parameters": {
        "pin_scene_id": "scene-id-for-pin-entry",
        "warning_scene_id": "scene-id-for-warning",
        "off_scene_id": "scene-id-for-power-off"
      }
    }
  ]
}
```

#### Device Fields

- **id** (required): Unique device identifier
  - Max 15 characters (Telegram callback data limit)
  - Examples: "tv1", "phone_alice", "ipad_bob"
  - Used in API requests and Telegram bot interactions

- **name** (required): User-friendly display name
  - Shown in UI and notifications
  - Examples: "Living Room TV", "Alice's iPhone"

- **type** (required): Device type for categorization and statistics
  - Used for display, grouping, and stats
  - Examples: "tv", "phone", "tablet", "ps5"

- **driver** (required): Driver name for device control
  - References a configured driver (e.g., "aqara", "kidslox")
  - Determines how the device is controlled

- **parameters** (optional): Device-specific driver parameters
  - Override driver defaults for this specific device
  - Allows multiple devices to use the same driver with different settings
  - Structure depends on the driver (see driver documentation)

### Driver Parameters

#### Separation of Concerns

- **Driver**: Control mechanism (global configuration)
- **Device**: User-facing entity with optional parameter overrides

This architecture allows:
1. Multiple devices (tv1, tv2) controlled by one driver (aqara)
2. Device-specific customization (different scenes per TV)
3. Future extensibility (multiple phones via Kidslox API with different device_ids)

#### Example: Aqara Driver

**Global defaults** in `aqara` section:
```json
{
  "aqara": {
    "app_id": "your-app-id",
    "app_key": "your-app-key",
    "key_id": "your-key-id",
    "base_url": "https://open-cn.aqara.com",
    "scenes": {
      "tv_pin_entry": "default-pin-scene",
      "tv_warning": "default-warning-scene",
      "tv_power_off": "default-off-scene"
    }
  }
}
```

**Device-specific overrides** in device parameters:
```json
{
  "id": "tv1",
  "driver": "aqara",
  "parameters": {
    "pin_scene_id": "custom-pin-scene-for-tv1",
    "warning_scene_id": "custom-warning-scene-for-tv1",
    "off_scene_id": "custom-off-scene-for-tv1"
  }
}
```

**Aqara Parameters:**
- `pin_scene_id`: Scene to trigger on session start (PIN entry)
- `warning_scene_id`: Scene to trigger for time warnings
- `off_scene_id`: Scene to trigger on session stop (power off)

#### Example: Future Kidslox Driver

```json
{
  "devices": [
    {
      "id": "phone_alice",
      "name": "Alice's iPhone",
      "type": "phone",
      "driver": "kidslox",
      "parameters": {
        "device_id": "alice-phone-uuid-from-kidslox",
        "profile_id": "restricted-profile"
      }
    },
    {
      "id": "phone_bob",
      "name": "Bob's iPhone",
      "type": "phone",
      "driver": "kidslox",
      "parameters": {
        "device_id": "bob-phone-uuid-from-kidslox",
        "profile_id": "restricted-profile"
      }
    }
  ],
  "kidslox": {
    "api_key": "shared-api-key-for-all-devices"
  }
}
```

### Device ID Constraints

**Important:** Device IDs must be ≤15 characters due to Telegram callback data limits (64 bytes total).

**Good IDs:**
- "tv1", "tv2", "tv_living"
- "phone1", "ipad_alice"
- "ps5_bedroom"

**Bad IDs (too long):**
- "living_room_television_main"
- "alice_iphone_12_pro_max"

## Telegram Bot Configuration

Bot configuration (`bot-config.json`) includes timezone support:

```json
{
  "telegram": {
    "token": "YOUR_BOT_TOKEN",
    "allowed_users": [123456789],
    "timezone": "Europe/Riga"
  }
}
```

**timezone**: IANA timezone name for time display formatting
- Default: "UTC"
- Examples: "Europe/Riga", "America/New_York", "Asia/Tokyo"
- Times are stored in UTC, displayed in configured timezone

## Migration from Old Configuration

### Before (deprecated):
```json
{
  "aqara": {
    "devices": [
      {
        "id": "aqara-device-id",
        "name": "Living Room TV",
        "device_type": "tv"
      }
    ]
  }
}
```

### After (current):
```json
{
  "devices": [
    {
      "id": "tv1",
      "name": "Living Room TV",
      "type": "tv",
      "driver": "aqara"
    }
  ],
  "aqara": {
    "scenes": { ... }
  }
}
```

**Changes:**
1. Devices moved to global `devices` array
2. Removed `aqara.devices` array
3. Added `driver` field to each device
4. Renamed `device_type` to `type`
5. Added optional `parameters` for customization
