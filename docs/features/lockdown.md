# Lockdown Feature

A lockdown blocks screen time for a child (or all children) on a device (or all devices). It is used as a punishment ("Simon cannot use the PS5") or as an emergency stop for everyone ("all children, all devices").

Each lockdown is a **rule** with:

| Field | Values |
|-------|--------|
| Child | one child, or all children |
| Device | one device, or all devices |
| Duration | until end of day (expires at local midnight, server `timezone`), or until a parent removes it |

Several rules can be active at the same time. Creating a rule with the same child and device again replaces the old one (for example, to change its duration).

## Behavior

When a rule is **created**:

1. Active sessions covered by the rule are stopped immediately (used time is recorded as usual). A shared session is stopped if any of its children is covered.
2. Any attempt to start a covered session is rejected with `LOCKDOWN_ACTIVE`:
   - Parent/admin API (`POST /v1/sessions`) - `409 Conflict`
   - Child API (`POST /child/sessions`) - `403 Forbidden`
3. Extending a covered session and adding a covered child to a session are rejected the same way.
4. Nothing bypasses a lockdown: not the parent, not the daily approval.
5. The scheduler acts as a safety net: on every tick (1 minute) it ends any active session covered by a rule (e.g. one that started during a race).

When a rule is **removed** or **expires**, sessions can be started again right away.

**Time quotas are never reset by a lockdown.** Used minutes stay counted, remaining minutes stay available after unlock.

Examples:

| Rule | Effect |
|------|--------|
| Simon + PS5, until end of day | Simon cannot play PS5 today. Alisa can. Simon can still use the TV. |
| Simon + all devices, until unlocked | Simon has no screen time until a parent removes the rule. |
| All children + all devices, until unlocked | The old "global lockdown": nobody can start anything. |

## Via Telegram Bot

"⚙️ More..." menu, then "🔒 Lockdowns":

- shows the active rules with their duration and a "🔓 Unlock" button for each,
- "➕ New lockdown": pick the child (or all), then the device (or all), then the duration ("Until end of day" or "Until I unlock"). The bot reports how many sessions were stopped.

`/today` lists the active rules. `/newsession` stops early only when a rule for all children and all devices is active; otherwise the server rejects a blocked combination with a clear error.

## Child Web App

- `GET /child/today` returns `"lockdown": true` when a rule blocks this child on **all** devices (`lockdown_until` is set for "until end of day" rules). The app shows a red banner and hides the device chooser.
- `GET /child/devices` marks each device with `"locked": true` (and `locked_until`) when a rule blocks this child on it. The app shows these devices as locked and does not let the child pick them.
- Start/extend attempts get `403 LOCKDOWN_ACTIVE`.

## Windows Agent

No agent changes are needed. Creating a rule stops matching sessions, so the agent sees no active session on its next poll and locks the workstation. Note: a device bypass (`/v1/devices/:id/bypass`) still unlocks an agent device without a session, because it is an explicit parent action.

## Via API

```bash
# List active rules
curl -H "X-Metron-Key: $KEY" http://localhost:8080/v1/lockdowns

# Simon cannot use PS5 until midnight (stops his PS5 session if running)
curl -X POST -H "X-Metron-Key: $KEY" -H "Content-Type: application/json" \
  -d '{"child_id":"simon","device_id":"ps51","until":"end_of_day"}' \
  http://localhost:8080/v1/lockdowns

# Global lockdown (all children, all devices, until removed)
curl -X POST -H "X-Metron-Key: $KEY" -H "Content-Type: application/json" \
  http://localhost:8080/v1/lockdowns

# Remove a rule
curl -X DELETE -H "X-Metron-Key: $KEY" http://localhost:8080/v1/lockdowns/3
```

See `docs/api/v1.md` (Lockdown section) and `docs/api/openapi.yaml` for full request/response schemas.

## Storage and Migration

Rules are stored in the `lockdown_rules` table (`child_id`, `device_id`, `reason`, `enabled_at`, `enabled_by`, `expires_at`). "All" is stored as `"*"`. Expired rules are ignored and cleaned up when rules are read.

The old single-row `lockdown` table (global on/off flag) is migrated on start: if it was enabled, it becomes an "all children, all devices, until removed" rule. Then the old table is dropped. The old endpoints `GET /v1/lockdown`, `POST /v1/lockdown/enable` and `POST /v1/lockdown/disable` were removed.
