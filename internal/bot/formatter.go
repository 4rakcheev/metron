package bot

import (
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

// FormatTodayStats formats today's statistics into a Telegram message
func FormatTodayStats(stats *TodayStats, activeSessions []Session, childrenMap map[string]Child) string {
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
		emoji := getChildEmoji(child.ChildName)

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
		emoji := getChildEmoji(child.Name)
		sb.WriteString(fmt.Sprintf("%s *%s*\n", emoji, child.Name))
		sb.WriteString(fmt.Sprintf("   ID: `%s`\n", child.ID))
		sb.WriteString(fmt.Sprintf("   Weekday: %d min\n", child.WeekdayLimit))
		sb.WriteString(fmt.Sprintf("   Weekend: %d min\n", child.WeekendLimit))

		if child.BreakRule != nil {
			sb.WriteString(fmt.Sprintf("   Break: every %d min, %d min rest\n",
				child.BreakRule.BreakAfterMinutes,
				child.BreakRule.BreakDurationMinutes))
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
		emoji := getDeviceEmoji(device.Type)
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
				emoji := getChildEmoji(child.Name)
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
			emoji := getChildEmoji(child.Name)
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

// FormatError formats an error message
func FormatError(err error) string {
	return fmt.Sprintf("❌ *Error*\n\n%s", err.Error())
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

// getChildEmoji returns an emoji for a child based on their name
func getChildEmoji(name string) string {
	// Map common names to emojis
	lowerName := strings.ToLower(name)

	switch {
	case strings.Contains(lowerName, "сем") || strings.Contains(lowerName, "sem"):
		return "👦"
	case strings.Contains(lowerName, "алис") || strings.Contains(lowerName, "alic"):
		return "👧"
	default:
		return "👶"
	}
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
		return "📱"
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
