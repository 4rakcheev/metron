# Downtime Feature

Downtime is a scheduled period when device access is restricted for children. This feature supports different schedules for weekdays vs weekends and allows temporarily skipping downtime.

## Configuration

Downtime is configured in `config.json` under the `downtime` key.

### Per-Day Schedules (Recommended)

Set explicit schedules for each day of the week. This gives you full control:

```json
{
  "downtime": {
    "sunday": { "start_time": "21:00", "end_time": "10:00" },
    "monday": { "start_time": "21:00", "end_time": "10:00" },
    "tuesday": { "start_time": "21:00", "end_time": "10:00" },
    "wednesday": { "start_time": "21:00", "end_time": "10:00" },
    "thursday": { "start_time": "21:00", "end_time": "10:00" },
    "friday": { "start_time": "22:00", "end_time": "10:00" },
    "saturday": { "start_time": "22:00", "end_time": "10:00" }
  }
}
```

This configuration means:
- **Sun-Thu nights**: Downtime starts at 21:00 (school nights - early wake up tomorrow)
- **Fri-Sat nights**: Downtime starts at 22:00 (can stay up later - no school tomorrow)

**Fields:**
- `start_time` - When downtime begins (HH:MM format, 24-hour)
- `end_time` - When downtime ends (HH:MM format, 24-hour)

### Weekday/Weekend Schedules (Alternative)

You can also use grouped schedules as a simpler alternative:

```json
{
  "downtime": {
    "weekday": { "start_time": "21:00", "end_time": "10:00" },
    "weekend": { "start_time": "22:00", "end_time": "10:00" }
  }
}
```

With this format:
- **weekday** applies to Monday through Friday
- **weekend** applies to Saturday and Sunday

**Note:** Per-day schedules take priority over weekday/weekend if both are specified.

### Overnight Downtime

Downtime can span midnight. For example, `21:00` to `10:00` means:
- Downtime starts at 9 PM
- Continues through midnight
- Ends at 10 AM the next day

### Legacy Format (Backward Compatible)

The old flat format still works and applies the same schedule to both weekdays and weekends:

```json
{
  "downtime": {
    "start_time": "22:00",
    "end_time": "10:00"
  }
}
```

### Timezone

Downtime times are interpreted in the timezone configured in `config.json`:

```json
{
  "timezone": "Europe/Riga"
}
```

## Per-Child Downtime

Each child has a `downtime_enabled` flag that determines whether downtime applies to them:

```json
{
  "id": "child-uuid",
  "name": "Alice",
  "downtime_enabled": true
}
```

This can be toggled via:
- **Telegram Bot**: Use `/children` command and tap on a child
- **API**: `PATCH /v1/children/:id` with `{"downtime_enabled": false}`

## Skip Downtime Today

Downtime can be skipped for all children for the current day. This is useful for special occasions (holidays, movie nights, etc.).

### Via Telegram Bot

1. Tap **More...** from the main menu
2. Tap **Skip Downtime Today**
3. The button will show **Downtime Skipped Today** when active

### Via API

```bash
# Skip downtime for today
POST /v1/downtime/skip-today

# Check if downtime is skipped
GET /v1/downtime/skip-status
```

**Response:**
```json
{
  "skipped_today": true,
  "skip_date": "2025-12-09"
}
```

### Auto-Expiry

The skip automatically expires at midnight (in the configured timezone). No manual reset is needed.

## How Downtime Works

1. **Session Creation**: When creating a new session during downtime, it will be blocked unless:
   - The child has `downtime_enabled: false`
   - Downtime is skipped for today

2. **Active Sessions**: Sessions already in progress are not affected when downtime begins

3. **Day-of-Week Logic**: The system selects the schedule based on the current day:
   - **Per-day schedules** (e.g., `friday`, `saturday`) have highest priority
   - **Grouped schedules** (`weekday`, `weekend`) are used as fallback
   - With grouped schedules: Mon-Fri = weekday, Sat-Sun = weekend

## Storage

Skip date is stored in the `downtime_skip` SQLite table using a single-row pattern:

```sql
CREATE TABLE downtime_skip (
    id INTEGER PRIMARY KEY CHECK (id = 1),
    skip_date DATE NOT NULL,
    created_at DATETIME NOT NULL
);
```

## API Reference

See [API v1 Documentation](../api/v1.md#downtime) for complete endpoint documentation.
