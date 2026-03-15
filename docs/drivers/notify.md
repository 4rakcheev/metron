# Notify Driver

The notify driver sends Telegram notifications when sessions start, stop, or reach a time warning. It is designed for devices managed by external apps (e.g., Google Family Link, Apple Screen Time) where enforcement is manual -- a parent receives a notification and then grants or revokes time in the external app themselves.

## How It Works

1. A session starts on a device configured with `driver: "notify"`
2. The driver sends a Telegram message to every chat ID listed in the config
3. The message includes the child name, device name, duration, and end time
4. If `app_url` is configured, the message includes an inline button linking to the external app
5. When the session ends or a warning fires, another notification is sent

The driver never blocks session creation. If Telegram is unreachable or returns an error, the failure is logged and the session proceeds normally.

## Notification Types

### Child-Initiated Request (via Children Control PWA)

When a child starts a session through the web UI, the notification asks the parent to grant time:

```
Session Request

Masha requested 30 min on Android Phone

Duration: 30 min
Ends at: 14:30

Please grant time in Family Link.
[Open Family Link]
```

### Parent-Initiated Session (via Telegram Bot)

When a parent starts a session through the bot, the notification is a reminder:

```
Session Started

Masha -- 30 min on Android Phone
Ends at: 14:30

Don't forget to grant time in Family Link.
[Open Family Link]
```

### Session Ended

```
Session Ended

Masha -- Android Phone (28 min used)

Revoke bonus time in Family Link.
[Open Family Link]
```

### Warning

```
5 min remaining -- Masha on Android Phone
```

## Configuration

### Top-Level `notify` Section

Add a `notify` section to `config.json`:

```json
{
  "notify": {
    "telegram_token": "123456:ABC-DEF...",
    "chat_ids": [123456789, 987654321]
  }
}
```

| Field | Required | Description |
|-------|----------|-------------|
| `telegram_token` | Yes | Telegram Bot API token. Can be the same token used by the Telegram bot (`metron-bot`). |
| `chat_ids` | Yes | List of Telegram chat IDs to receive notifications. Typically parent chat IDs. |

The driver is only registered when the `notify` section is present in the config. If omitted, devices with `driver: "notify"` will fail to start sessions because no driver is available.

### Device Parameters

Each device using the notify driver can specify parameters:

```json
{
  "devices": [
    {
      "id": "phone1",
      "name": "Android Phone",
      "type": "phone",
      "emoji": "📱",
      "driver": "notify",
      "parameters": {
        "app_url": "https://familylink.google.com",
        "app_name": "Family Link"
      }
    }
  ]
}
```

| Parameter | Required | Default | Description |
|-----------|----------|---------|-------------|
| `app_url` | No | _(none)_ | URL for the external management app. When set, notifications include an inline button linking to this URL. |
| `app_name` | No | `"app"` | Display name for the external app, used in notification text (e.g., "Please grant time in **Family Link**"). |

## Reusing the Telegram Bot Token

The notify driver can share the same Telegram bot token as `metron-bot`. Both use the Telegram Bot API independently -- the bot uses webhooks for interactive commands while the driver uses direct `sendMessage` calls for one-way notifications. There is no conflict.

## Error Handling

All notification failures return `nil` from the driver methods. This is a deliberate design choice: a session must never fail to start because Telegram is temporarily unavailable. Errors are logged at the `ERROR` level with the chat ID and error details.

Rate limiting (HTTP 429) from Telegram is also treated as a non-fatal error -- the notification is lost but the session continues.

## Capabilities

| Capability | Supported |
|------------|-----------|
| Warnings | Yes |
| Live State | No |
| Scheduling | Yes |

The driver supports warnings (`ApplyWarning`) so the scheduler sends time-remaining notifications. Live state is not supported since the driver has no way to query the external app.

## Use Cases

- Android phones/tablets managed by Google Family Link
- iOS devices managed by Apple Screen Time
- Any device where a parental control app handles enforcement but Metron tracks the schedule
- Devices where you want notification-only tracking without any automated enforcement
