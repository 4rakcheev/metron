# Weekday Approval Gate

On school days a child should first do their homework. With the approval gate, a child cannot start screen time from the child app on the configured days until a parent approves that child for today in Telegram. After approval the child uses their daily limit (`weekday_limit`) on their own, as before.

## Configuration

`config.json`:

```json
"approval_gate": {
  "enabled": true,
  "required_days": ["monday", "tuesday", "wednesday", "thursday", "friday"]
}
```

- `required_days` - lowercase English day names. If empty, Monday to Friday is used.
- Days are computed in the server `timezone`.
- Without this section (or with `"enabled": false`) the gate is off and the approval endpoints are not registered.

## Rules

- **Per child.** Each child is approved separately.
- **For one day.** An approval is valid for the current local date only. A new day needs a new approval.
- **Only child-initiated actions are gated.** Starting or extending a session from the child app needs an approval. A parent starting a session from the bot (`/newsession`) or `POST /v1/sessions` does not, because that is an approval by itself.
- **Downtime wins.** An approved child still cannot start a session during downtime. Check order: lockdown, downtime, approval, remaining time.
- **Lockdowns win.** An approved child cannot use a device blocked by a lockdown rule (see `docs/features/lockdown.md`).
- **Revoke** removes today's approval. It does not stop a session that is already running.
- On storage errors the gate fails closed (the child cannot start).

## Via Telegram Bot

- `/approve` or the "✅ Approve day" button in the main menu shows each child with their status ("⏳ waiting" or "✅ approved") and an Approve or Revoke button.
- `/today` shows the approval status of each child when approval is required today.

## Child Web App

`GET /child/today` returns:

```json
"approval": { "required": true, "approved": false, "approved_at": null }
```

When `required` is `true` and `approved` is `false`, the app shows a "homework first" message and hides the device chooser. Start/extend attempts get `403 APPROVAL_REQUIRED`. The app refreshes every 30 seconds, so the approval appears without a reload. The `approval` field is absent when the gate is disabled.

## Via API

```bash
# Today's status for all children
curl -H "X-Metron-Key: $KEY" http://localhost:8080/v1/approvals/today

# Approve Simon for today
curl -X POST -H "X-Metron-Key: $KEY" -H "Content-Type: application/json" \
  -d '{"approved_by":"telegram"}' http://localhost:8080/v1/children/simon/approval

# Revoke
curl -X DELETE -H "X-Metron-Key: $KEY" http://localhost:8080/v1/children/simon/approval
```

## Storage

Table `daily_approvals` (`child_id`, `date` as `YYYY-MM-DD` in the server timezone, `approved_at`, `approved_by`), one row per child and day.
