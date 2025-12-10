# Shared Time Tracking in Metron

## Overview

Metron supports **shared time sessions** where multiple children can use the same device simultaneously. When children share screen time together (e.g., watching TV or playing PS5), all participants' daily usage is tracked equally.

## How It Works

### Creating a Shared Session

When you create a session with multiple `child_ids`, it creates a **single session** that tracks time for **all children simultaneously**.

**Example:**
```bash
POST /v1/sessions
{
  "device_type": "tv",
  "device_id": "tv1",
  "child_ids": ["alice-id", "bob-id", "charlie-id"],
  "minutes": 30
}
```

This creates **one session** where:
- Alice, Bob, and Charlie watch TV together
- The session runs for 30 minutes total (not 90 minutes)
- All three children's daily usage will be incremented by 30 minutes

### Time Accounting

When the session completes (either manually stopped or auto-expired):

1. System calculates elapsed time from session start
2. Loops through **all** `child_ids` in the session
3. Increments **each child's** daily usage by the **same elapsed time**

**Code Reference** (`internal/core/manager.go:200-208`):
```go
// Update daily usage for all children
elapsed := int(time.Since(session.StartTime).Minutes())
today := time.Now()

for _, childID := range session.ChildIDs {
    if err := m.storage.IncrementDailyUsage(ctx, childID, today, elapsed); err != nil {
        return fmt.Errorf("failed to update daily usage for child %s: %w", childID, err)
    }
}
```

## Real-World Examples

### Example 1: Two Children Watching TV

**Setup:**
- Alice has 60 minutes/day limit
- Bob has 90 minutes/day limit
- Both have used 0 minutes today

**Action:**
```bash
POST /v1/sessions
{
  "device_type": "tv",
  "child_ids": ["alice", "bob"],
  "minutes": 30
}
```

**Result after 30 minutes:**
- Alice: 30 minutes used, 30 remaining
- Bob: 30 minutes used, 60 remaining
- Both children tracked the same 30 minutes

### Example 2: Three Children, Different Limits

**Setup:**
- Alice: 60 min limit, 40 already used (20 remaining)
- Bob: 90 min limit, 0 used (90 remaining)
- Charlie: 120 min limit, 0 used (120 remaining)

**Action:**
```bash
POST /v1/sessions
{
  "device_type": "tv",
  "child_ids": ["alice", "bob", "charlie"],
  "minutes": 30
}
```

**Result:**
- Session will be **rejected** because Alice only has 20 minutes remaining
- Error: `INSUFFICIENT_TIME` - "child has insufficient remaining time"
- **No session is created** if ANY child lacks sufficient time

### Example 3: Sequential Sessions

Children can have separate sessions or join shared sessions throughout the day:

**Morning - Alice alone:**
```bash
POST /v1/sessions
{
  "device_type": "tv",
  "child_ids": ["alice"],
  "minutes": 20
}
```
Result: Alice uses 20 minutes

**Afternoon - Alice and Bob together:**
```bash
POST /v1/sessions
{
  "device_type": "tv",
  "child_ids": ["alice", "bob"],
  "minutes": 30
}
```
Result:
- Alice: 20 + 30 = 50 minutes used
- Bob: 0 + 30 = 30 minutes used

**Evening - Bob alone:**
```bash
POST /v1/sessions
{
  "device_type": "tv",
  "child_ids": ["bob"],
  "minutes": 15
}
```
Result:
- Alice: Still 50 minutes used
- Bob: 30 + 15 = 45 minutes used

## Time Limit Validation

Before creating a session, Metron checks if **all children** have sufficient remaining time:

1. Calculates each child's `today_remaining` time
2. Compares against requested session `minutes`
3. **Rejects** the session if ANY child lacks sufficient time

**Implementation** (`internal/core/manager.go:67-89`):
```go
// Validate children exist and have sufficient time
today := time.Now()
for _, childID := range childIDs {
    child, err := m.storage.GetChild(ctx, childID)
    if err != nil {
        return nil, fmt.Errorf("failed to get child %s: %w", childID, err)
    }

    // Check daily time remaining
    dailyLimit := child.GetDailyLimit(today)
    usage, err := m.storage.GetDailyUsage(ctx, childID, today)

    minutesUsed := 0
    if err == nil && usage != nil {
        minutesUsed = usage.MinutesUsed
    }

    remaining := dailyLimit - minutesUsed
    if remaining < durationMinutes {
        return nil, fmt.Errorf("%w: child %s has %d minutes remaining, requested %d",
            ErrInsufficientTime, child.Name, remaining, durationMinutes)
    }
}
```

## Session Extension

When extending a shared session, the extension applies to **all children equally**:

```bash
PATCH /v1/sessions/{session-id}
{
  "action": "extend",
  "additional_minutes": 15
}
```

- All children in the session must have at least 15 minutes remaining
- If any child lacks sufficient time, extension is **rejected**
- If approved, all children's usage will increase by 15 minutes when session completes

## Use Cases

### 1. Siblings Watching TV Together
**Scenario:** Alice and Bob watch a movie together
**Solution:** Create one session with both children
**Benefit:** Accurately tracks shared screen time

### 2. Playdates
**Scenario:** Alice has a friend over, both play PS5
**Solution:** Create session with both children's IDs
**Benefit:** Both children's time is tracked fairly

### 3. Educational Content
**Scenario:** All three children watch educational content together
**Solution:** Create one session with all children
**Benefit:** All children's quotas are decremented equally

### 4. Different Activities, Same Device
**Scenario:** Morning (Alice alone), Afternoon (Alice + Bob), Evening (Bob alone)
**Solution:** Create separate sessions for each scenario
**Benefit:** Accurate individual and shared time tracking

## Statistics and Reporting

### Today's Stats API

The `/v1/stats/today` endpoint shows individual usage:

```json
{
  "date": "2025-12-09",
  "children": [
    {
      "child_id": "alice",
      "child_name": "Alice",
      "today_used": 50,
      "today_remaining": 10,
      "today_limit": 60,
      "sessions_today": 2,
      "usage_percent": 83
    },
    {
      "child_id": "bob",
      "child_name": "Bob",
      "today_used": 45,
      "today_remaining": 45,
      "today_limit": 90,
      "sessions_today": 2,
      "usage_percent": 50
    }
  ]
}
```

From this you can see:
- Alice participated in 2 sessions, used 50 minutes total
- Bob participated in 2 sessions, used 45 minutes total
- They may have shared some sessions and had individual sessions

## Telegram Bot Integration

### Display Shared Sessions

When listing active sessions, show all participants:

```
📺 Active TV Session
├ Duration: 30 minutes
├ Remaining: 20 minutes
└ Children: Alice, Bob, Charlie

Actions:
[Extend +5] [Extend +15] [Extend +30] [Stop]
```

### Smart Extension

Check if all children can extend:

```
⚠️ Extension Request: +15 minutes

Alice: ✅ 30 min remaining
Bob: ✅ 60 min remaining
Charlie: ❌ 10 min remaining (insufficient)

Cannot extend - Charlie needs more time
```

### Child Selection for New Session

Allow parent to select multiple children:

```
Select children for TV session:
☑️ Alice (20/60 min used today)
☑️ Bob (30/90 min used today)
☐ Charlie (110/120 min used today)

Duration: [30 min] [60 min] [120 min]

⚠️ Charlie has only 10 min remaining
```

## API Examples

### Create Shared Session
```bash
curl -X POST http://localhost:8080/v1/sessions \
  -H "X-Metron-Key: your-key" \
  -H "Content-Type: application/json" \
  -d '{
    "device_type": "tv",
    "child_ids": ["alice-id", "bob-id"],
    "minutes": 30
  }'
```

### Check Individual Child Status
```bash
curl -H "X-Metron-Key: your-key" \
  http://localhost:8080/v1/children/alice-id
```

Response:
```json
{
  "id": "alice-id",
  "name": "Alice",
  "today_used": 50,
  "today_remaining": 10,
  "sessions_today": 2
}
```

### List Active Sessions
```bash
curl -H "X-Metron-Key: your-key" \
  "http://localhost:8080/v1/sessions?active=true"
```

Response shows all active sessions with their child IDs:
```json
[
  {
    "id": "session-1",
    "device_type": "tv",
    "child_ids": ["alice-id", "bob-id"],
    "start_time": "2025-12-09T16:00:00Z",
    "expected_duration": 30,
    "remaining_minutes": 20,
    "status": "active"
  }
]
```

## Database Schema

Shared time tracking is supported by the database schema:

### Sessions Table
```sql
CREATE TABLE sessions (
    id TEXT PRIMARY KEY,
    device_type TEXT NOT NULL,
    device_id TEXT NOT NULL,
    child_ids TEXT NOT NULL,  -- JSON array: ["child1", "child2"]
    start_time TIMESTAMP NOT NULL,
    expected_duration INTEGER NOT NULL,
    remaining_minutes INTEGER NOT NULL,
    status TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
```

### Daily Usage Table
```sql
CREATE TABLE daily_usage (
    child_id TEXT NOT NULL,
    date TEXT NOT NULL,  -- YYYY-MM-DD format
    minutes_used INTEGER NOT NULL DEFAULT 0,
    session_count INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (child_id, date),
    FOREIGN KEY (child_id) REFERENCES children(id)
);
```

Each child has their own row in `daily_usage`, tracking their total minutes used that day, regardless of whether the time was in shared or individual sessions.

## Break Rules in Shared Sessions

**Important Implementation Detail:**

When multiple children share a session, break rules are enforced collectively:

1. **Break Trigger**: If ANY child needs a break (based on their individual break rule), the ENTIRE session is paused for ALL children
2. **Break Duration**: The break duration is determined by the first child who triggers it
3. **Resume Together**: All children resume the session together when the break ends

**Example:**
```
Session with Alice and Bob:
- Alice: break after 45 minutes, 10-minute break
- Bob: break after 60 minutes, 15-minute break

At 45 minutes:
- Alice's break rule triggers
- ENTIRE session pauses for BOTH Alice and Bob
- Break lasts 10 minutes (Alice's break duration)
- Both resume at 55 minutes
```

**Code Reference** (`internal/scheduler/scheduler.go:109-139`):
```go
// Check if any child needs a break
for _, childID := range session.ChildIDs {
    child, err := s.storage.GetChild(ctx, childID)
    if child.BreakRule != nil && session.NeedsBreak(child.BreakRule) {
        // Enforce break for ENTIRE session
        session.Status = core.SessionStatusPaused
        // ... break logic
        return s.storage.UpdateSession(ctx, session)
    }
}
```

**Rationale**: When children watch TV or play together, it's logical that they all take breaks together rather than individually.

**Considerations**:
- Children with different break rules in the same session may experience unexpected pauses
- The child with the shortest break interval will trigger breaks for everyone
- Consider assigning similar break rules to children who frequently share sessions

## Best Practices

1. **Always validate before creating sessions** - Check all children have sufficient time
2. **Use meaningful device IDs** - `tv1`, `ps5-living-room`, etc.
3. **Track session completion** - Ensure sessions are properly stopped to update usage
4. **Assign similar break rules to siblings** - Prevents unexpected breaks in shared sessions
5. **Monitor first-to-trigger behavior** - The child with shortest break interval affects all participants

## Future Enhancements

Potential improvements to shared time tracking:

1. **Partial participation** - Track when children join/leave mid-session
2. **Weighted time** - Different multipliers for different children
3. **Session splitting** - Automatically split session if one child reaches limit
4. **Shared quotas** - Family-level daily limits in addition to individual limits
5. **Activity types** - Educational content might not count against quota
