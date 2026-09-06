# Lockdown Feature

Lockdown is a global "emergency stop" for screen time. When enabled, ALL sessions are blocked - no session can be created or extended by anyone (child web app, Telegram bot, or direct API) until a parent manually disables lockdown.

Unlike downtime, lockdown has no schedule: it is turned on and off manually and stays on indefinitely until unlocked.

## Behavior

When lockdown is **enabled**:

1. All currently active sessions are stopped immediately (used time is recorded as usual).
2. Any attempt to start a session is rejected with `LOCKDOWN_ACTIVE`:
   - Parent/admin API (`POST /v1/sessions`) - `409 Conflict`
   - Child API (`POST /child/sessions`) - `403 Forbidden`
3. Session extensions are rejected the same way.
4. The parent override context (used by the bot for downtime) does NOT bypass lockdown - the block is absolute.
5. The scheduler acts as a safety net: on every tick (1 minute) it ends any active session that slipped through a race window while lockdown was being enabled.

When lockdown is **disabled**:

- Sessions can be started again immediately.

**Time quotas are never reset by lockdown.** Used minutes stay counted, remaining minutes stay available after unlock.

## Via Telegram Bot

More menu ("⚙️ More...") contains the lockdown toggle:

- "🔒 Lock All Sessions" - shown when lockdown is off
- "🔓 Unlock Sessions (LOCKED)" - shown when lockdown is on

Both actions ask for confirmation before applying. Enabling reports how many sessions were stopped. `/today` shows a "🔒 LOCKDOWN ACTIVE" banner while lockdown is on.

## Child Web App

While lockdown is active, `GET /child/today` returns `"lockdown": true` and the child UI:

- shows a red "Screen time is locked" banner,
- hides the device chooser and duration picker,
- gets a `403 LOCKDOWN_ACTIVE` on any start/extend attempt.

## Windows Agent

No agent changes are needed. Enabling lockdown stops all sessions, so the agent sees no active session on its next poll and locks the workstation as usual.

## Via API

```bash
# Get lockdown state
curl -H "X-Metron-Key: $KEY" http://localhost:8080/v1/lockdown

# Enable lockdown (stops all active sessions)
curl -X POST -H "X-Metron-Key: $KEY" http://localhost:8080/v1/lockdown/enable

# Disable lockdown
curl -X POST -H "X-Metron-Key: $KEY" http://localhost:8080/v1/lockdown/disable
```

See `docs/api/v1.md` (Lockdown section) and `docs/api/openapi.yaml` for full request/response schemas.

## Storage

Lockdown state lives in a single-row SQLite table:

```sql
CREATE TABLE lockdown (
    id INTEGER PRIMARY KEY CHECK (id = 1),
    enabled BOOLEAN NOT NULL DEFAULT 0,
    enabled_at DATETIME,
    enabled_by TEXT,
    updated_at DATETIME NOT NULL
);
```

Access goes through the narrow `core.LockdownStorage` interface (implemented by SQLite storage), injected via setters into `SessionManager` and the scheduler - the same pattern as `core.DowntimeSkipStorage`. On storage read errors the checks fail open (sessions are allowed) so a broken flag never blocks or kills sessions by accident.

## Implementation Notes

- Enforcement lives in `core.SessionManager.StartSession` / `ExtendSession` (`internal/core/manager.go`), returning `core.ErrLockdownActive`.
- Enable handler sets the flag FIRST, then stops active sessions - so no new session can slip in between.
- Scheduler safety net: `internal/scheduler/scheduler.go` ends active sessions on tick while lockdown is enabled.
- API handler: `internal/api/handlers/lockdown.go`; routes registered in `internal/api/router.go`.
