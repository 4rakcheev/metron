package bot

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// timezone is the IANA timezone for formatting times (set during bot initialization)
var timezone *time.Location

// SetTimezone sets the timezone for time formatting
func SetTimezone(tz string) error {
	loc, err := time.LoadLocation(tz)
	if err != nil {
		return fmt.Errorf("invalid timezone %s: %w", tz, err)
	}
	timezone = loc
	return nil
}

// formatTime formats a time in the configured timezone
func formatTime(t time.Time, layout string) string {
	if timezone != nil {
		t = t.In(timezone)
	}
	return t.Format(layout)
}

// FormatTodayStats formats today's statistics into a Telegram message.
// approvals maps child ID to today's approval status; pass nil when approval
// is not required today or the approval gate is disabled (no approval lines are shown).
func FormatTodayStats(stats *TodayStats, activeSessions []Session, childrenMap map[string]Child,
	approvals map[string]ChildApproval) string {
	var sb strings.Builder

	sb.WriteString("📊 *Today's Screen Time Summary*\n")
	sb.WriteString(fmt.Sprintf("Date: %s\n\n", stats.Date))

	if len(stats.Children) == 0 {
		sb.WriteString("No children configured yet.\n")
		return sb.String()
	}

	// Group sessions by child for shared time calculation
	childSessionMap := make(map[string][]Session)
	for _, session := range activeSessions {
		for _, childID := range session.ChildIDs {
			childSessionMap[childID] = append(childSessionMap[childID], session)
		}
	}

	for _, child := range stats.Children {
		emoji := child.ChildEmoji

		// For now, we can't distinguish personal from shared in the API response
		// This would require additional API endpoint or session history
		// So we'll just show the total with a note about shared sessions

		sb.WriteString(fmt.Sprintf("%s *%s*\n", emoji, child.ChildName))
		sb.WriteString(fmt.Sprintf("   Used: %d min / %d min (%.0f%%)\n",
			child.TodayUsed, child.TodayLimit, float64(child.UsagePercent)))
		sb.WriteString(fmt.Sprintf("   Remaining: %d min\n", child.TodayRemaining))

		if child.SessionsToday > 0 {
			sb.WriteString(fmt.Sprintf("   Sessions: %d\n", child.SessionsToday))
		}

		if approval, ok := approvals[child.ChildID]; ok && approval.Required {
			if approval.Approved {
				sb.WriteString(fmt.Sprintf("   ✅ Approved today%s\n", formatApprovedAt(approval, " at ")))
			} else {
				sb.WriteString("   ⏳ Waiting for approval (/approve)\n")
			}
		}

		// Show active sessions for this child
		activeSess := childSessionMap[child.ChildID]
		if len(activeSess) > 0 {
			sb.WriteString("   🟢 *Active:*\n")
			for _, sess := range activeSess {
				endTime, remaining := calculateSessionEnd(sess)
				deviceEmoji := getDeviceEmoji(sess.DeviceType)

				// Check if shared
				displayName := getDeviceDisplayName(sess.DeviceType)
				if len(sess.ChildIDs) > 1 {
					sb.WriteString(fmt.Sprintf("      %s %s (shared)\n", deviceEmoji, displayName))
				} else {
					sb.WriteString(fmt.Sprintf("      %s %s\n", deviceEmoji, displayName))
				}
				sb.WriteString(fmt.Sprintf("      Ends %s (+%d min left)\n",
					formatTime(endTime, "15:04"), remaining))
			}
		}

		sb.WriteString("\n")
	}

	if stats.ActiveSessions > 0 {
		sb.WriteString(fmt.Sprintf("🎮 Active sessions: %d\n", stats.ActiveSessions))
	}

	return sb.String()
}

// FormatChildren formats the children list
func FormatChildren(children []Child) string {
	var sb strings.Builder

	sb.WriteString("👶 *Children List*\n\n")

	if len(children) == 0 {
		sb.WriteString("No children configured.\n")
		return sb.String()
	}

	for _, child := range children {
		emoji := child.Emoji
		sb.WriteString(fmt.Sprintf("%s *%s*\n", emoji, child.Name))
		sb.WriteString(fmt.Sprintf("   ID: `%s`\n", child.ID))
		sb.WriteString(fmt.Sprintf("   Weekday: %d min\n", child.WeekdayLimit))
		sb.WriteString(fmt.Sprintf("   Weekend: %d min\n", child.WeekendLimit))

		if child.BreakRule != nil {
			sb.WriteString(fmt.Sprintf("   Break: every %d min, %d min rest\n",
				child.BreakRule.BreakAfterMinutes,
				child.BreakRule.BreakDurationMinutes))
		}

		// Show downtime status
		if child.DowntimeEnabled {
			sb.WriteString("   🌙 Downtime: Enabled\n")
		} else {
			sb.WriteString("   ☀️ Downtime: Disabled\n")
		}

		sb.WriteString("\n")
	}

	return sb.String()
}

// FormatDevices formats the devices list
func FormatDevices(devices []Device) string {
	var sb strings.Builder

	sb.WriteString("📺 *Available Devices*\n\n")

	if len(devices) == 0 {
		sb.WriteString("No devices configured.\n")
		return sb.String()
	}

	for _, device := range devices {
		emoji := resolveDeviceEmoji(device)
		displayName := getDeviceDisplayName(device.Type)
		sb.WriteString(fmt.Sprintf("%s *%s*\n", emoji, displayName))
		sb.WriteString(fmt.Sprintf("   Driver: `%s`\n", device.Type))

		var features []string
		if device.Capabilities.SupportsWarnings {
			features = append(features, "warnings")
		}
		if device.Capabilities.SupportsLiveState {
			features = append(features, "live state")
		}
		if device.Capabilities.SupportsScheduling {
			features = append(features, "scheduling")
		}

		if len(features) > 0 {
			sb.WriteString(fmt.Sprintf("   Features: %s\n", strings.Join(features, ", ")))
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

// FormatActiveSessions formats active sessions for selection
func FormatActiveSessions(sessions []Session, childrenMap map[string]Child) string {
	var sb strings.Builder

	sb.WriteString("🎮 *Active Sessions*\n\n")

	if len(sessions) == 0 {
		sb.WriteString("No active sessions.\n")
		return sb.String()
	}

	for i, sess := range sessions {
		deviceEmoji := getDeviceEmoji(sess.DeviceType)
		displayName := getDeviceDisplayName(sess.DeviceType)
		endTime, remaining := calculateSessionEnd(sess)

		// Parse start time
		startTime, err := time.Parse(time.RFC3339, sess.StartTime)
		if err != nil {
			startTime = time.Now()
		}

		// Get child names with emoji
		var childNames []string
		for _, childID := range sess.ChildIDs {
			if child, ok := childrenMap[childID]; ok {
				emoji := child.Emoji
				childNames = append(childNames, emoji+" "+child.Name)
			}
		}

		sb.WriteString(fmt.Sprintf("%d. %s *%s*\n", i+1, deviceEmoji, displayName))
		sb.WriteString(fmt.Sprintf("   Children: %s\n", strings.Join(childNames, ", ")))
		sb.WriteString(fmt.Sprintf("   Started: %s\n", formatTime(startTime, "15:04")))
		sb.WriteString(fmt.Sprintf("   Ends %s (+%d min left)\n\n",
			formatTime(endTime, "15:04"), remaining))
	}

	return sb.String()
}

// FormatSessionCreated formats a success message for session creation
func FormatSessionCreated(session *Session, childrenMap map[string]Child) string {
	var sb strings.Builder

	deviceEmoji := getDeviceEmoji(session.DeviceType)
	displayName := getDeviceDisplayName(session.DeviceType)
	endTime, _ := calculateSessionEnd(*session)

	sb.WriteString("✅ *Session Started*\n\n")
	sb.WriteString(fmt.Sprintf("%s Device: *%s*\n", deviceEmoji, displayName))

	// Get child names
	var childNames []string
	for _, childID := range session.ChildIDs {
		if child, ok := childrenMap[childID]; ok {
			emoji := child.Emoji
			childNames = append(childNames, emoji+" "+child.Name)
		}
	}

	if len(childNames) > 0 {
		sb.WriteString(fmt.Sprintf("👶 Children: %s\n", strings.Join(childNames, ", ")))
	}

	sb.WriteString(fmt.Sprintf("⏱ Duration: %d minutes\n", session.ExpectedDuration))
	sb.WriteString(fmt.Sprintf("🏁 Ends at: %s\n", formatTime(endTime, "15:04")))

	return sb.String()
}

// FormatSessionExtended formats a success message for session extension
func FormatSessionExtended(session *Session, additionalMinutes int) string {
	var sb strings.Builder

	endTime, remaining := calculateSessionEnd(*session)

	sb.WriteString("✅ *Session Extended*\n\n")
	sb.WriteString(fmt.Sprintf("➕ Added: %d minutes\n", additionalMinutes))
	sb.WriteString(fmt.Sprintf("⏱ Remaining: %d minutes\n", remaining))
	sb.WriteString(fmt.Sprintf("🏁 New end time: %s\n", formatTime(endTime, "15:04")))

	return sb.String()
}

// FormatChildrenAddedToSession formats a success message for adding children to a session
func FormatChildrenAddedToSession(session *Session, addedChildIDs []string, childrenMap map[string]Child) string {
	var sb strings.Builder

	deviceEmoji := getDeviceEmoji(session.DeviceType)
	displayName := getDeviceDisplayName(session.DeviceType)
	_, remaining := calculateSessionEnd(*session)

	sb.WriteString("✅ *Children Added to Session*\n\n")
	sb.WriteString(fmt.Sprintf("%s Device: *%s*\n", deviceEmoji, displayName))

	// Show added children
	var addedNames []string
	for _, childID := range addedChildIDs {
		if child, ok := childrenMap[childID]; ok {
			emoji := child.Emoji
			addedNames = append(addedNames, emoji+" "+child.Name)
		}
	}

	if len(addedNames) > 0 {
		sb.WriteString(fmt.Sprintf("➕ Added: %s\n", strings.Join(addedNames, ", ")))
	}

	// Show all children now in session
	var allNames []string
	for _, childID := range session.ChildIDs {
		if child, ok := childrenMap[childID]; ok {
			emoji := child.Emoji
			allNames = append(allNames, emoji+" "+child.Name)
		}
	}

	if len(allNames) > 0 {
		sb.WriteString(fmt.Sprintf("👶 All Children: %s\n", strings.Join(allNames, ", ")))
	}

	sb.WriteString(fmt.Sprintf("⏱ Remaining: %d minutes\n", remaining))

	return sb.String()
}

// FormatSessionStopped formats a success message for stopping a session early
func FormatSessionStopped(session *Session, childrenMap map[string]Child) string {
	var sb strings.Builder

	deviceEmoji := getDeviceEmoji(session.DeviceType)
	displayName := getDeviceDisplayName(session.DeviceType)
	_, remaining := calculateSessionEnd(*session)

	sb.WriteString("🛑 *Session Stopped*\n\n")
	sb.WriteString(fmt.Sprintf("%s Device: *%s*\n", deviceEmoji, displayName))

	// Get child names
	var childNames []string
	for _, childID := range session.ChildIDs {
		if child, ok := childrenMap[childID]; ok {
			emoji := child.Emoji
			childNames = append(childNames, emoji+" "+child.Name)
		}
	}

	if len(childNames) > 0 {
		sb.WriteString(fmt.Sprintf("👶 Children: %s\n", strings.Join(childNames, ", ")))
	}

	if remaining > 0 {
		sb.WriteString(fmt.Sprintf("↩️ Returned %d minutes to available time\n", remaining))
	}

	sb.WriteString("\n✅ Device has been locked.")

	return sb.String()
}

// FormatRewardGranted formats a success message for granting a reward
func FormatRewardGranted(childName, childEmoji string, response *GrantRewardResponse) string {
	var sb strings.Builder

	sb.WriteString("✅ *Reward Granted!*\n\n")
	sb.WriteString(fmt.Sprintf("%s *%s*\n\n", childEmoji, childName))
	sb.WriteString(fmt.Sprintf("🎁 Bonus added: +%d minutes\n", response.MinutesGranted))
	sb.WriteString(fmt.Sprintf("📊 Total rewards today: %d minutes\n", response.TodayRewardGranted))
	sb.WriteString(fmt.Sprintf("⏱ Remaining time: %d minutes\n", response.TodayRemaining))
	sb.WriteString(fmt.Sprintf("📏 Daily limit: %d minutes\n", response.TodayLimit))

	return sb.String()
}

// FormatFineApplied formats a success message for applying a fine
func FormatFineApplied(childName, childEmoji string, response *DeductFineResponse) string {
	var sb strings.Builder

	sb.WriteString("*Fine Applied*\n\n")
	sb.WriteString(fmt.Sprintf("%s *%s*\n\n", childEmoji, childName))
	sb.WriteString(fmt.Sprintf("Time deducted: -%d minutes\n", response.MinutesDeducted))
	sb.WriteString(fmt.Sprintf("Total fines today: %d minutes\n", response.TodayFinesDeducted))
	sb.WriteString(fmt.Sprintf("Remaining time: %d minutes\n", response.TodayRemaining))
	sb.WriteString(fmt.Sprintf("Daily limit: %d minutes\n", response.TodayLimit))

	return sb.String()
}

// markdownEscaper escapes Telegram legacy Markdown control characters
var markdownEscaper = strings.NewReplacer("_", "\\_", "*", "\\*", "`", "\\`", "[", "\\[")

// escapeMarkdown escapes user/API-provided text so it can be embedded in a Markdown message
func escapeMarkdown(s string) string {
	return markdownEscaper.Replace(s)
}

// FormatError formats an error message
func FormatError(err error) string {
	// Escape Markdown control characters: an unbalanced _ or * in an error
	// message (e.g. error codes like LOCKDOWN_ACTIVE) makes Telegram reject
	// the whole message with a parse error, so the user would see nothing.
	switch apiErrorCode(err) {
	case "LOCKDOWN_ACTIVE":
		var statusErr *StatusError
		detail := ""
		if errors.As(err, &statusErr) && statusErr.APIMessage != "" {
			detail = "\n\n" + escapeMarkdown(statusErr.APIMessage)
		}
		return "🔒 *Blocked by a lockdown*" + detail +
			"\n\nTo unlock, open ⚙️ More, then 🔒 Lockdowns."
	case "DOWNTIME_ACTIVE":
		return "🌙 *Downtime is active*\n\nSessions cannot start during downtime." +
			"\n\nTo allow screen time today, open ⚙️ More, then 🌙 Skip Downtime Today."
	}
	return fmt.Sprintf("❌ *Error*\n\n%s", escapeMarkdown(err.Error()))
}

// lockdownScopeLabel describes the scope of a rule, e.g. "Simon: PS5" or "All children: all devices"
func lockdownScopeLabel(rule LockdownRule) string {
	child := "All children"
	if rule.ChildID != nil {
		child = rule.ChildName
	}
	device := "all devices"
	if rule.DeviceID != nil {
		device = rule.DeviceName
	}
	return child + ": " + device
}

// lockdownExpiryText describes when a rule ends: "until unlocked", "until end of day" or "until 02.01 15:04"
func lockdownExpiryText(rule LockdownRule, now time.Time) string {
	if rule.ExpiresAt == nil || *rule.ExpiresAt == "" {
		return "until unlocked"
	}
	expiresAt, err := time.Parse(time.RFC3339, *rule.ExpiresAt)
	if err != nil {
		return "until " + *rule.ExpiresAt
	}
	loc := timezone
	if loc == nil {
		loc = time.Local
	}
	local := now.In(loc)
	midnight := time.Date(local.Year(), local.Month(), local.Day()+1, 0, 0, 0, 0, loc)
	if expiresAt.Equal(midnight) {
		return "until end of day"
	}
	return "until " + formatTime(expiresAt, "02.01 15:04")
}

// FormatLockdownLines formats active rules as Markdown bullet lines, one per rule
func FormatLockdownLines(rules []LockdownRule, now time.Time) string {
	var sb strings.Builder
	for _, rule := range rules {
		sb.WriteString(fmt.Sprintf("• %s — %s\n", escapeMarkdown(lockdownScopeLabel(rule)), lockdownExpiryText(rule, now)))
	}
	return sb.String()
}

// FormatLockdownList formats the lockdown overview screen
func FormatLockdownList(rules []LockdownRule, note string, now time.Time) string {
	var sb strings.Builder
	if note != "" {
		sb.WriteString(note + "\n\n")
	}
	sb.WriteString("🔒 *Lockdowns*\n\n")
	sb.WriteString("A lockdown stops matching sessions and blocks new ones (child app, bot, API). Time quotas are kept.\n\n")
	if len(rules) == 0 {
		sb.WriteString("No active lockdowns.\n")
		return sb.String()
	}
	sb.WriteString("*Active:*\n")
	sb.WriteString(FormatLockdownLines(rules, now))
	return sb.String()
}

// FormatLockdownCreated formats the result of creating a lockdown rule
func FormatLockdownCreated(result *CreateLockdownResult, now time.Time) string {
	var sb strings.Builder
	sb.WriteString("🔒 *Lockdown Enabled*\n\n")
	sb.WriteString(fmt.Sprintf("%s — %s\n\n",
		escapeMarkdown(lockdownScopeLabel(result.LockdownRule)), lockdownExpiryText(result.LockdownRule, now)))
	sb.WriteString(fmt.Sprintf("Stopped %d session(s).\n", result.StoppedSessions))
	if result.FailedSessions > 0 {
		sb.WriteString(fmt.Sprintf("⚠️ %d session(s) could not be stopped. They will be ended automatically within a minute.\n",
			result.FailedSessions))
	}
	sb.WriteString("\nTime quotas are kept.")
	return sb.String()
}

// formatApprovedAt returns prefix + local approval time (e.g. " at 16:05"), or "" if unknown
func formatApprovedAt(approval ChildApproval, prefix string) string {
	if approval.ApprovedAt == nil || *approval.ApprovedAt == "" {
		return ""
	}
	t, err := time.Parse(time.RFC3339, *approval.ApprovedAt)
	if err != nil {
		return ""
	}
	return prefix + formatTime(t, "15:04")
}

// FormatApprovals formats the "Approve day" screen
func FormatApprovals(approvals *TodayApprovals, note string) string {
	var sb strings.Builder
	if note != "" {
		sb.WriteString(note + "\n\n")
	}
	sb.WriteString("✅ *Approve day*\n\n")
	if approvals.Required {
		sb.WriteString("Approval is required today: children can start screen time only after you approve them " +
			"(for example, after homework). Sessions you start from the bot do not need approval.\n\n")
	} else {
		sb.WriteString("Approval is not required today (weekend) — children can start on their own.\n\n")
	}

	if len(approvals.Children) == 0 {
		sb.WriteString("No children configured.\n")
		return sb.String()
	}

	for _, child := range approvals.Children {
		name := escapeMarkdown(strings.TrimSpace(child.ChildEmoji + " " + child.ChildName))
		if child.Approved {
			sb.WriteString(fmt.Sprintf("✅ %s — approved%s\n", name, formatApprovedAt(child, " at ")))
		} else {
			sb.WriteString(fmt.Sprintf("⏳ %s — waiting\n", name))
		}
	}
	return sb.String()
}

// calculateSessionEnd calculates when a session will end and how many minutes remain
// This is the single source of truth for end time and remaining calculation
func calculateSessionEnd(session Session) (time.Time, int) {
	startTime, err := time.Parse(time.RFC3339, session.StartTime)
	if err != nil {
		// Fallback to current time
		startTime = time.Now()
	}

	// Calculate end time from start + expected duration (authoritative)
	endTime := startTime.Add(time.Duration(session.ExpectedDuration) * time.Minute)

	// Calculate remaining minutes from end time - now (don't trust session.RemainingMinutes)
	remaining := int(time.Until(endTime).Minutes())
	if remaining < 0 {
		remaining = 0
	}

	return endTime, remaining
}

// resolveDeviceEmoji returns the device's custom emoji if set, otherwise falls back to type-based default
func resolveDeviceEmoji(device Device) string {
	if device.Emoji != "" {
		return device.Emoji
	}
	return getDeviceEmoji(device.Type)
}

// getDeviceEmoji returns an emoji for a device type
func getDeviceEmoji(deviceType string) string {
	lowerType := strings.ToLower(deviceType)

	switch {
	case strings.Contains(lowerType, "tv"):
		return "📺"
	case strings.Contains(lowerType, "ps5") || strings.Contains(lowerType, "playstation"):
		return "🎮"
	case strings.Contains(lowerType, "ipad") || strings.Contains(lowerType, "tablet"):
		return "⬛"
	case strings.Contains(lowerType, "phone"):
		return "📱"
	case strings.Contains(lowerType, "aqara"):
		return "📺" // Aqara driver controls TV
	default:
		return "🖥"
	}
}

// getDeviceDisplayName returns a user-friendly display name for a device type
func getDeviceDisplayName(deviceType string) string {
	lowerType := strings.ToLower(deviceType)

	switch {
	case strings.Contains(lowerType, "tv"):
		return "TV"
	case strings.Contains(lowerType, "ps5") || strings.Contains(lowerType, "playstation"):
		return "PS5"
	case strings.Contains(lowerType, "ipad") || strings.Contains(lowerType, "tablet"):
		return "iPad"
	case strings.Contains(lowerType, "phone"):
		return "Phone"
	case strings.Contains(lowerType, "aqara"):
		return "TV" // Aqara driver controls TV, display as "TV"
	default:
		return deviceType
	}
}

// formatBypassExpiry describes when a bypass ends, e.g. "until 23:59" or "indefinitely"
func formatBypassExpiry(bypass *DeviceBypass) string {
	if bypass == nil || bypass.ExpiresAt == nil || *bypass.ExpiresAt == "" {
		return "(no expiry, until manually disabled)"
	}
	expiresAt, err := time.Parse(time.RFC3339, *bypass.ExpiresAt)
	if err != nil {
		return fmt.Sprintf("(until %s)", *bypass.ExpiresAt)
	}
	return fmt.Sprintf("(until %s)", formatTime(expiresAt, "02.01 15:04"))
}

// minutesUntilEndOfDay returns whole minutes from now until the next local midnight
// in the configured timezone (at least 1)
func minutesUntilEndOfDay(now time.Time) int {
	loc := timezone
	if loc == nil {
		loc = time.Local
	}
	local := now.In(loc)
	midnight := time.Date(local.Year(), local.Month(), local.Day()+1, 0, 0, 0, 0, loc)
	minutes := int(midnight.Sub(local).Minutes())
	if minutes < 1 {
		minutes = 1
	}
	return minutes
}
