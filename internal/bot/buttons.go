package bot

import (
	"encoding/json"
	"fmt"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// CallbackData represents the data embedded in callback buttons
type CallbackData struct {
	Action       string `json:"a"`             // Action type (newsession, manage, etc)
	SubAction    string `json:"sa,omitempty"`  // Sub-action (extend, stop, add_kid)
	Step         int    `json:"s,omitempty"`   // Current step in flow
	ChildID      string `json:"c,omitempty"`   // Child ID (resolved from index)
	ChildIndex   int    `json:"ci,omitempty"`  // Child index in list (for compact callback)
	Device       string `json:"d,omitempty"`   // Device ID
	Duration     int    `json:"m,omitempty"`   // Duration in minutes
	Session      string `json:"ses,omitempty"` // Session ID (resolved from index)
	SessionIndex int    `json:"si,omitempty"`  // Session index in list (for compact callback)
	RuleID       int64  `json:"i,omitempty"`   // Lockdown rule ID (lockdown unlock)
}

// MarshalCallback converts CallbackData to JSON string
func MarshalCallback(data CallbackData) string {
	b, err := json.Marshal(data)
	if err != nil {
		// Should never happen with simple structs
		return ""
	}
	return string(b)
}

// UnmarshalCallback parses callback data from JSON string
func UnmarshalCallback(data string) (*CallbackData, error) {
	var cb CallbackData
	if err := json.Unmarshal([]byte(data), &cb); err != nil {
		return nil, fmt.Errorf("failed to unmarshal callback: %w", err)
	}
	return &cb, nil
}

// BuildChildrenButtons creates buttons for selecting children
func BuildChildrenButtons(children []Child, action string, step int) tgbotapi.InlineKeyboardMarkup {
	var rows [][]tgbotapi.InlineKeyboardButton

	// Individual children - use index instead of full UUID
	for i, child := range children {
		emoji := child.Emoji
		callback := MarshalCallback(CallbackData{
			Action:     action,
			Step:       step,
			ChildIndex: i, // Use index to keep callback data small
		})

		btn := tgbotapi.NewInlineKeyboardButtonData(
			fmt.Sprintf("%s %s", emoji, child.Name),
			callback,
		)
		rows = append(rows, []tgbotapi.InlineKeyboardButton{btn})
	}

	// "Shared" option (all children) - use special index -1
	if len(children) > 1 {
		callback := MarshalCallback(CallbackData{
			Action:     action,
			Step:       step,
			ChildIndex: -1, // Special marker for shared sessions
		})

		btn := tgbotapi.NewInlineKeyboardButtonData("👨‍👩‍👧 Shared (All)", callback)
		rows = append(rows, []tgbotapi.InlineKeyboardButton{btn})
	}

	// Cancel button
	cancelBtn := tgbotapi.NewInlineKeyboardButtonData(
		"❌ Cancel",
		MarshalCallback(CallbackData{Action: "cancel"}),
	)
	rows = append(rows, []tgbotapi.InlineKeyboardButton{cancelBtn})

	return tgbotapi.NewInlineKeyboardMarkup(rows...)
}

// BuildDevicesButtons creates buttons for selecting devices
func BuildDevicesButtons(devices []Device, action string, step int, childIndex int) tgbotapi.InlineKeyboardMarkup {
	var rows [][]tgbotapi.InlineKeyboardButton

	for _, device := range devices {
		emoji := resolveDeviceEmoji(device)
		callback := MarshalCallback(CallbackData{
			Action:     action,
			Step:       step,
			ChildIndex: childIndex, // Use index to keep callback data small
			Device:     device.ID,  // Use device ID, not type
		})

		btn := tgbotapi.NewInlineKeyboardButtonData(
			fmt.Sprintf("%s %s", emoji, device.Name),
			callback,
		)
		rows = append(rows, []tgbotapi.InlineKeyboardButton{btn})
	}

	// Back button
	backBtn := tgbotapi.NewInlineKeyboardButtonData(
		"◀️ Back",
		MarshalCallback(CallbackData{Action: action, Step: step - 1, ChildIndex: childIndex}),
	)

	// Cancel button
	cancelBtn := tgbotapi.NewInlineKeyboardButtonData(
		"❌ Cancel",
		MarshalCallback(CallbackData{Action: "cancel"}),
	)

	rows = append(rows, []tgbotapi.InlineKeyboardButton{backBtn, cancelBtn})

	return tgbotapi.NewInlineKeyboardMarkup(rows...)
}

// BuildDurationButtons creates buttons for selecting duration
func BuildDurationButtons(action string, step int, childIndex int, device string) tgbotapi.InlineKeyboardMarkup {
	durations := []int{5, 15, 30, 60, 120}
	var rows [][]tgbotapi.InlineKeyboardButton

	// Create two rows: [5, 15, 30] and [60, 120]
	var row1, row2 []tgbotapi.InlineKeyboardButton

	for i, duration := range durations {
		callback := MarshalCallback(CallbackData{
			Action:     action,
			Step:       step,
			ChildIndex: childIndex, // Use index to keep callback data small
			Device:     device,
			Duration:   duration,
		})

		label := fmt.Sprintf("+%d", duration)
		btn := tgbotapi.NewInlineKeyboardButtonData(label, callback)

		if i < 3 {
			row1 = append(row1, btn)
		} else {
			row2 = append(row2, btn)
		}
	}

	rows = append(rows, row1, row2)

	// Back and Cancel buttons
	backBtn := tgbotapi.NewInlineKeyboardButtonData(
		"◀️ Back",
		MarshalCallback(CallbackData{Action: action, Step: step - 1, ChildIndex: childIndex, Device: device}),
	)

	cancelBtn := tgbotapi.NewInlineKeyboardButtonData(
		"❌ Cancel",
		MarshalCallback(CallbackData{Action: "cancel"}),
	)

	rows = append(rows, []tgbotapi.InlineKeyboardButton{backBtn, cancelBtn})

	return tgbotapi.NewInlineKeyboardMarkup(rows...)
}

// BuildSessionsButtons creates buttons for selecting an active session for a
// specific action (extend/stop flows). For the full sessions overview with
// per-session action buttons, use BuildSessionManagementButtons instead.
func BuildSessionsButtons(sessions []Session, action string) tgbotapi.InlineKeyboardMarkup {
	var rows [][]tgbotapi.InlineKeyboardButton

	for i, session := range sessions {
		emoji := getDeviceEmoji(session.DeviceType)
		label := fmt.Sprintf("%d. %s %s", i+1, emoji, session.DeviceType)

		callback := MarshalCallback(CallbackData{
			Action:       action,
			Step:         1,
			SessionIndex: i, // Use index to keep callback data small
		})

		btn := tgbotapi.NewInlineKeyboardButtonData(label, callback)
		rows = append(rows, []tgbotapi.InlineKeyboardButton{btn})
	}

	// Cancel button
	cancelBtn := tgbotapi.NewInlineKeyboardButtonData(
		"❌ Cancel",
		MarshalCallback(CallbackData{Action: "cancel"}),
	)
	rows = append(rows, []tgbotapi.InlineKeyboardButton{cancelBtn})

	return tgbotapi.NewInlineKeyboardMarkup(rows...)
}

// BuildSessionManagementButtons creates buttons for managing active sessions
// Each session gets 3 action buttons: Extend, Stop, Add Kid
func BuildSessionManagementButtons(sessions []Session, childrenMap map[string]Child) tgbotapi.InlineKeyboardMarkup {
	var rows [][]tgbotapi.InlineKeyboardButton

	for i, session := range sessions {
		deviceEmoji := getDeviceEmoji(session.DeviceType)
		displayName := getDeviceDisplayName(session.DeviceType)

		// Get child names for this session
		var childNames []string
		for _, childID := range session.ChildIDs {
			if child, ok := childrenMap[childID]; ok {
				childEmoji := child.Emoji
				childNames = append(childNames, childEmoji+" "+child.Name)
			}
		}

		// Header row with session info (not clickable)
		_, remaining := calculateSessionEnd(session)
		sessionLabel := fmt.Sprintf("%d. %s %s", i+1, deviceEmoji, displayName)
		if len(childNames) > 0 {
			sessionLabel += fmt.Sprintf(" · %d min", remaining)
		}

		// Action buttons row: [Extend] [Stop] [Add Kid]
		extendBtn := tgbotapi.NewInlineKeyboardButtonData(
			"⏱ Extend",
			MarshalCallback(CallbackData{
				Action:       "manage",
				SubAction:    "extend",
				Step:         1,
				SessionIndex: i,
			}),
		)

		stopBtn := tgbotapi.NewInlineKeyboardButtonData(
			"🛑 Stop",
			MarshalCallback(CallbackData{
				Action:       "manage",
				SubAction:    "stop",
				Step:         1,
				SessionIndex: i,
			}),
		)

		addKidBtn := tgbotapi.NewInlineKeyboardButtonData(
			"👶 Add Kid",
			MarshalCallback(CallbackData{
				Action:       "manage",
				SubAction:    "add_kid",
				Step:         1,
				SessionIndex: i,
			}),
		)

		// Add session label as a single-button row (for visual grouping)
		labelBtn := tgbotapi.NewInlineKeyboardButtonData(
			sessionLabel,
			MarshalCallback(CallbackData{Action: "noop"}), // No-op callback
		)
		rows = append(rows, []tgbotapi.InlineKeyboardButton{labelBtn})

		// Add action buttons
		rows = append(rows, []tgbotapi.InlineKeyboardButton{extendBtn, stopBtn, addKidBtn})
	}

	// Grant Reward button
	rewardBtn := tgbotapi.NewInlineKeyboardButtonData(
		"🎁 Grant Reward",
		MarshalCallback(CallbackData{Action: "reward", Step: 0}),
	)
	rows = append(rows, []tgbotapi.InlineKeyboardButton{rewardBtn})

	// Cancel button
	cancelBtn := tgbotapi.NewInlineKeyboardButtonData(
		"❌ Cancel",
		MarshalCallback(CallbackData{Action: "cancel"}),
	)
	rows = append(rows, []tgbotapi.InlineKeyboardButton{cancelBtn})

	return tgbotapi.NewInlineKeyboardMarkup(rows...)
}

// BuildAddKidButtons creates buttons for selecting which child to add to a session
func BuildAddKidButtons(sessionIndex int, availableChildren []Child, alreadyShared bool) tgbotapi.InlineKeyboardMarkup {
	var rows [][]tgbotapi.InlineKeyboardButton

	// Show available children to add
	for i, child := range availableChildren {
		emoji := child.Emoji
		callback := MarshalCallback(CallbackData{
			Action:       "manage",
			SubAction:    "add_kid",
			Step:         2,
			SessionIndex: sessionIndex,
			ChildIndex:   i,
		})

		btn := tgbotapi.NewInlineKeyboardButtonData(
			fmt.Sprintf("%s %s", emoji, child.Name),
			callback,
		)
		rows = append(rows, []tgbotapi.InlineKeyboardButton{btn})
	}

	// "Mark as Shared" button if not already shared
	if !alreadyShared && len(availableChildren) > 0 {
		callback := MarshalCallback(CallbackData{
			Action:       "manage",
			SubAction:    "add_kid",
			Step:         2,
			SessionIndex: sessionIndex,
			ChildIndex:   -1, // Special marker for "all available children"
		})

		btn := tgbotapi.NewInlineKeyboardButtonData("👨‍👩‍👧 Mark as Shared (All)", callback)
		rows = append(rows, []tgbotapi.InlineKeyboardButton{btn})
	}

	// Back and Cancel buttons
	backBtn := tgbotapi.NewInlineKeyboardButtonData(
		"◀️ Back",
		MarshalCallback(CallbackData{Action: "manage", Step: 0}),
	)

	cancelBtn := tgbotapi.NewInlineKeyboardButtonData(
		"❌ Cancel",
		MarshalCallback(CallbackData{Action: "cancel"}),
	)

	rows = append(rows, []tgbotapi.InlineKeyboardButton{backBtn, cancelBtn})

	return tgbotapi.NewInlineKeyboardMarkup(rows...)
}

// BuildExtendDurationButtons creates buttons for selecting extension duration
func BuildExtendDurationButtons(sessionIndex int) tgbotapi.InlineKeyboardMarkup {
	durations := []int{5, 15, 30, 60, 120}
	var rows [][]tgbotapi.InlineKeyboardButton

	// Create two rows
	var row1, row2 []tgbotapi.InlineKeyboardButton

	for i, duration := range durations {
		callback := MarshalCallback(CallbackData{
			Action:       "manage",
			SubAction:    "extend",
			Step:         2,
			SessionIndex: sessionIndex,
			Duration:     duration,
		})

		label := fmt.Sprintf("+%d", duration)
		btn := tgbotapi.NewInlineKeyboardButtonData(label, callback)

		if i < 3 {
			row1 = append(row1, btn)
		} else {
			row2 = append(row2, btn)
		}
	}

	rows = append(rows, row1, row2)

	// Back and Cancel buttons
	backBtn := tgbotapi.NewInlineKeyboardButtonData(
		"◀️ Back",
		MarshalCallback(CallbackData{Action: "manage", Step: 0}),
	)

	cancelBtn := tgbotapi.NewInlineKeyboardButtonData(
		"❌ Cancel",
		MarshalCallback(CallbackData{Action: "cancel"}),
	)

	rows = append(rows, []tgbotapi.InlineKeyboardButton{backBtn, cancelBtn})

	return tgbotapi.NewInlineKeyboardMarkup(rows...)
}

// BuildMainMenuButtons creates main menu shortcut buttons
func BuildMainMenuButtons() tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("📊 Today", "/today"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🎬 Sessions",
				MarshalCallback(CallbackData{Action: "sessions_menu"})),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("✅ Approve day",
				MarshalCallback(CallbackData{Action: approvalAction})),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("⚙️ More...",
				MarshalCallback(CallbackData{Action: "more_menu"})),
		),
	)
}

// BuildQuickActionsButtons creates compact action buttons for attaching to responses
func BuildQuickActionsButtons() tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("📊 Today", "/today"),
			tgbotapi.NewInlineKeyboardButtonData("🎬 Sessions",
				MarshalCallback(CallbackData{Action: "sessions_menu"})),
			tgbotapi.NewInlineKeyboardButtonData("⚙️ More",
				MarshalCallback(CallbackData{Action: "more_menu"})),
		),
	)
}

// BuildRewardGrantedButtons offers starting a session right after a reward:
// a reward only raises the daily limit, it does not unlock any device by itself
func BuildRewardGrantedButtons() tgbotapi.InlineKeyboardMarkup {
	quick := BuildQuickActionsButtons()
	rows := [][]tgbotapi.InlineKeyboardButton{
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("▶️ Start Session",
				MarshalCallback(CallbackData{Action: "newsession", Step: 0})),
		),
	}
	rows = append(rows, quick.InlineKeyboard...)
	return tgbotapi.NewInlineKeyboardMarkup(rows...)
}

// BuildSessionsMenuButtons creates the sessions management submenu
func BuildSessionsMenuButtons() tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("▶️ New Session",
				MarshalCallback(CallbackData{Action: "newsession", Step: 0})),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🎁 Reward",
				MarshalCallback(CallbackData{Action: "reward", Step: 0})),
			tgbotapi.NewInlineKeyboardButtonData("📉 Fine",
				MarshalCallback(CallbackData{Action: "fine", Step: 0})),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🛑 Stop All Sessions",
				MarshalCallback(CallbackData{Action: "stop_all"})),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🛑 Stop Specific Session",
				MarshalCallback(CallbackData{Action: "stop", Step: 0})),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("◀️ Back",
				MarshalCallback(CallbackData{Action: "main_menu"})),
		),
	)
}

// BuildMoreMenuButtons creates the additional features submenu.
// activeLockdowns is the number of active lockdown rules (shown on the button).
func BuildMoreMenuButtons(skipDowntimeActive bool, activeLockdowns int) tgbotapi.InlineKeyboardMarkup {
	skipText := "🌙 Skip Downtime Today"
	if skipDowntimeActive {
		skipText = "✅ Downtime Skipped Today"
	}

	lockdownText := "🔒 Lockdowns"
	if activeLockdowns > 0 {
		lockdownText = fmt.Sprintf("🔒 Lockdowns (%d active)", activeLockdowns)
	}

	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(skipText,
				MarshalCallback(CallbackData{Action: "skip_downtime"})),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(lockdownText,
				MarshalCallback(CallbackData{Action: lockdownAction})),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🔓 Bypass Mode",
				MarshalCallback(CallbackData{Action: "bypass", Step: 0})),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("◀️ Back",
				MarshalCallback(CallbackData{Action: "main_menu"})),
		),
	)
}

// Lockdown flow callback encoding (action "lk", kept short for the 64-byte limit):
//
//	{"a":"lk"}                               list of active rules
//	{"a":"lk","sa":"u","i":<rule id>}        unlock (delete) a rule
//	{"a":"lk","s":1}                         new rule: pick child
//	{"a":"lk","s":2,"ci":<child>}            new rule: pick device
//	{"a":"lk","s":3,"ci":<child>,"d":<dev>}  new rule: pick duration
//	{"a":"lk","s":4,"ci":<child>,"d":<dev>,"m":<until>}  create the rule
//
// <child> is the child index in ListChildren, or lockdownAllChildren (-1) for all children.
// "All" is always encoded explicitly, so an omitted "ci" (index 0) is never read as "all".
// <dev> is the device ID, or lockdownAllDevices ("*") for all devices.
// <until> is lockdownUntilEndOfDayCode or lockdownUntilManualCode; anything else is rejected.
const (
	lockdownAction            = "lk"
	lockdownSubUnlock         = "u"
	lockdownAllChildren       = -1
	lockdownAllDevices        = "*"
	lockdownUntilEndOfDayCode = 1
	lockdownUntilManualCode   = 2
)

// BuildLockdownListButtons creates the lockdown overview keyboard: one unlock button
// per active rule, plus "New lockdown" and "Back"
func BuildLockdownListButtons(rules []LockdownRule) tgbotapi.InlineKeyboardMarkup {
	var rows [][]tgbotapi.InlineKeyboardButton

	for _, rule := range rules {
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🔓 Unlock "+lockdownScopeLabel(rule),
				MarshalCallback(CallbackData{Action: lockdownAction, SubAction: lockdownSubUnlock, RuleID: rule.ID})),
		))
	}

	rows = append(rows,
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("➕ New lockdown",
				MarshalCallback(CallbackData{Action: lockdownAction, Step: 1})),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("⬅️ Back",
				MarshalCallback(CallbackData{Action: "more_menu"})),
		),
	)

	return tgbotapi.NewInlineKeyboardMarkup(rows...)
}

// lockdownBackCancelRow returns a [Back, Cancel] row where Back goes to the given callback
func lockdownBackCancelRow(back CallbackData) []tgbotapi.InlineKeyboardButton {
	return tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("◀️ Back", MarshalCallback(back)),
		tgbotapi.NewInlineKeyboardButtonData("❌ Cancel", MarshalCallback(CallbackData{Action: "cancel"})),
	)
}

// BuildLockdownChildButtons creates the child picker for a new lockdown ("All children" first)
func BuildLockdownChildButtons(children []Child) tgbotapi.InlineKeyboardMarkup {
	rows := [][]tgbotapi.InlineKeyboardButton{
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("👥 All children",
				MarshalCallback(CallbackData{Action: lockdownAction, Step: 2, ChildIndex: lockdownAllChildren})),
		),
	}

	for i, child := range children {
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(fmt.Sprintf("%s %s", child.Emoji, child.Name),
				MarshalCallback(CallbackData{Action: lockdownAction, Step: 2, ChildIndex: i})),
		))
	}

	rows = append(rows, lockdownBackCancelRow(CallbackData{Action: lockdownAction}))
	return tgbotapi.NewInlineKeyboardMarkup(rows...)
}

// BuildLockdownDeviceButtons creates the device picker for a new lockdown ("All devices" first)
func BuildLockdownDeviceButtons(devices []Device, childIndex int) tgbotapi.InlineKeyboardMarkup {
	rows := [][]tgbotapi.InlineKeyboardButton{
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("📺 All devices",
				MarshalCallback(CallbackData{Action: lockdownAction, Step: 3, ChildIndex: childIndex, Device: lockdownAllDevices})),
		),
	}

	for _, device := range devices {
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(fmt.Sprintf("%s %s", resolveDeviceEmoji(device), device.Name),
				MarshalCallback(CallbackData{Action: lockdownAction, Step: 3, ChildIndex: childIndex, Device: device.ID})),
		))
	}

	rows = append(rows, lockdownBackCancelRow(CallbackData{Action: lockdownAction, Step: 1}))
	return tgbotapi.NewInlineKeyboardMarkup(rows...)
}

// BuildLockdownDurationButtons creates the duration picker for a new lockdown
func BuildLockdownDurationButtons(childIndex int, deviceID string) tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🌙 Until end of day",
				MarshalCallback(CallbackData{
					Action: lockdownAction, Step: 4, ChildIndex: childIndex, Device: deviceID, Duration: lockdownUntilEndOfDayCode,
				})),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🔐 Until I unlock",
				MarshalCallback(CallbackData{
					Action: lockdownAction, Step: 4, ChildIndex: childIndex, Device: deviceID, Duration: lockdownUntilManualCode,
				})),
		),
		lockdownBackCancelRow(CallbackData{Action: lockdownAction, Step: 2, ChildIndex: childIndex}),
	)
}

// BuildLockdownDoneButtons is shown after a lockdown is created: back to the list or quick actions
func BuildLockdownDoneButtons() tgbotapi.InlineKeyboardMarkup {
	quick := BuildQuickActionsButtons()
	rows := [][]tgbotapi.InlineKeyboardButton{
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🔒 Lockdowns",
				MarshalCallback(CallbackData{Action: lockdownAction})),
		),
	}
	rows = append(rows, quick.InlineKeyboard...)
	return tgbotapi.NewInlineKeyboardMarkup(rows...)
}

// Approval flow callback encoding (action "apr"):
//
//	{"a":"apr"}                         approval screen
//	{"a":"apr","sa":"ok","ci":<i>}      approve child i
//	{"a":"apr","sa":"rv","ci":<i>}      ask to confirm revoking child i
//	{"a":"apr","sa":"rv","s":1,"ci":<i>} revoke child i
//
// <i> is the child index in the GET /v1/approvals/today children list.
const (
	approvalAction     = "apr"
	approvalSubApprove = "ok"
	approvalSubRevoke  = "rv"
)

// BuildApprovalButtons creates the approval screen keyboard: approve or revoke per child, plus Back
func BuildApprovalButtons(children []ChildApproval) tgbotapi.InlineKeyboardMarkup {
	var rows [][]tgbotapi.InlineKeyboardButton

	for i, child := range children {
		label := "✅ Approve " + child.ChildName
		sub := approvalSubApprove
		if child.Approved {
			label = "↩️ Revoke " + child.ChildName
			sub = approvalSubRevoke
		}
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(label,
				MarshalCallback(CallbackData{Action: approvalAction, SubAction: sub, ChildIndex: i})),
		))
	}

	rows = append(rows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("⬅️ Back",
			MarshalCallback(CallbackData{Action: "main_menu"})),
	))

	return tgbotapi.NewInlineKeyboardMarkup(rows...)
}

// BuildApprovalRevokeConfirmButtons asks to confirm revoking today's approval for a child
func BuildApprovalRevokeConfirmButtons(childIndex int) tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("↩️ Yes, revoke",
				MarshalCallback(CallbackData{Action: approvalAction, SubAction: approvalSubRevoke, Step: 1, ChildIndex: childIndex})),
			tgbotapi.NewInlineKeyboardButtonData("◀️ Back",
				MarshalCallback(CallbackData{Action: approvalAction})),
		),
	)
}

// BuildRewardDurationButtons creates buttons for selecting reward duration
func BuildRewardDurationButtons(childIndex int) tgbotapi.InlineKeyboardMarkup {
	durations := []int{15, 30, 60}
	var rows [][]tgbotapi.InlineKeyboardButton

	// Create one row with all three reward options
	var row []tgbotapi.InlineKeyboardButton

	for _, duration := range durations {
		callback := MarshalCallback(CallbackData{
			Action:     "reward",
			Step:       2,
			ChildIndex: childIndex,
			Duration:   duration,
		})

		label := fmt.Sprintf("+%d min", duration)
		btn := tgbotapi.NewInlineKeyboardButtonData(label, callback)
		row = append(row, btn)
	}

	rows = append(rows, row)

	// Back and Cancel buttons
	backBtn := tgbotapi.NewInlineKeyboardButtonData(
		"◀️ Back",
		MarshalCallback(CallbackData{Action: "reward", Step: 0}),
	)

	cancelBtn := tgbotapi.NewInlineKeyboardButtonData(
		"❌ Cancel",
		MarshalCallback(CallbackData{Action: "cancel"}),
	)

	rows = append(rows, []tgbotapi.InlineKeyboardButton{backBtn, cancelBtn})

	return tgbotapi.NewInlineKeyboardMarkup(rows...)
}

// BuildFineDurationButtons creates buttons for selecting fine duration
func BuildFineDurationButtons(childIndex int) tgbotapi.InlineKeyboardMarkup {
	durations := []int{15, 30, 60}
	var rows [][]tgbotapi.InlineKeyboardButton

	// Create one row with all three fine options
	var row []tgbotapi.InlineKeyboardButton

	for _, duration := range durations {
		callback := MarshalCallback(CallbackData{
			Action:     "fine",
			Step:       2,
			ChildIndex: childIndex,
			Duration:   duration,
		})

		label := fmt.Sprintf("-%d min", duration)
		btn := tgbotapi.NewInlineKeyboardButtonData(label, callback)
		row = append(row, btn)
	}

	rows = append(rows, row)

	// Back and Cancel buttons
	backBtn := tgbotapi.NewInlineKeyboardButtonData(
		"Back",
		MarshalCallback(CallbackData{Action: "fine", Step: 0}),
	)

	cancelBtn := tgbotapi.NewInlineKeyboardButtonData(
		"Cancel",
		MarshalCallback(CallbackData{Action: "cancel"}),
	)

	rows = append(rows, []tgbotapi.InlineKeyboardButton{backBtn, cancelBtn})

	return tgbotapi.NewInlineKeyboardMarkup(rows...)
}

// BuildDowntimeToggleButtons creates buttons for toggling downtime per child
func BuildDowntimeToggleButtons(children []Child) tgbotapi.InlineKeyboardMarkup {
	var rows [][]tgbotapi.InlineKeyboardButton

	// Create button for each child
	for i, child := range children {
		emoji := child.Emoji
		var statusEmoji string
		if child.DowntimeEnabled {
			statusEmoji = "🌙"
		} else {
			statusEmoji = "☀️"
		}

		callback := MarshalCallback(CallbackData{
			Action:     "downtime",
			SubAction:  "toggle",
			ChildIndex: i,
		})

		label := fmt.Sprintf("%s %s %s", emoji, child.Name, statusEmoji)
		btn := tgbotapi.NewInlineKeyboardButtonData(label, callback)
		rows = append(rows, []tgbotapi.InlineKeyboardButton{btn})
	}

	// Cancel button
	cancelBtn := tgbotapi.NewInlineKeyboardButtonData(
		"❌ Close",
		MarshalCallback(CallbackData{Action: "cancel"}),
	)
	rows = append(rows, []tgbotapi.InlineKeyboardButton{cancelBtn})

	return tgbotapi.NewInlineKeyboardMarkup(rows...)
}

// DeviceWithBypass represents a device with its bypass status
type DeviceWithBypass struct {
	Device        Device
	BypassEnabled bool
}

// BuildBypassDevicesButtons creates buttons for selecting devices for bypass
func BuildBypassDevicesButtons(devices []DeviceWithBypass) tgbotapi.InlineKeyboardMarkup {
	var rows [][]tgbotapi.InlineKeyboardButton

	for _, dw := range devices {
		emoji := resolveDeviceEmoji(dw.Device)
		var statusEmoji string
		if dw.BypassEnabled {
			statusEmoji = "✅"
		} else {
			statusEmoji = "🔒"
		}

		callback := MarshalCallback(CallbackData{
			Action: "bypass",
			Step:   1,
			Device: dw.Device.ID,
		})

		label := fmt.Sprintf("%s %s %s", emoji, dw.Device.Name, statusEmoji)
		btn := tgbotapi.NewInlineKeyboardButtonData(label, callback)
		rows = append(rows, []tgbotapi.InlineKeyboardButton{btn})
	}

	// Cancel button
	cancelBtn := tgbotapi.NewInlineKeyboardButtonData(
		"❌ Close",
		MarshalCallback(CallbackData{Action: "cancel"}),
	)
	rows = append(rows, []tgbotapi.InlineKeyboardButton{cancelBtn})

	return tgbotapi.NewInlineKeyboardMarkup(rows...)
}

// bypassUntilEndOfDay is the callback duration marker for "bypass until local midnight".
// The bot never offers an indefinite bypass: a forgotten one silently disables enforcement for good.
const bypassUntilEndOfDay = -1

// BuildBypassActionsButtons creates buttons for bypass actions (enable/disable)
func BuildBypassActionsButtons(deviceID string, currentlyEnabled bool) tgbotapi.InlineKeyboardMarkup {
	var rows [][]tgbotapi.InlineKeyboardButton

	if currentlyEnabled {
		// Show disable option
		disableBtn := tgbotapi.NewInlineKeyboardButtonData(
			"🔒 Disable Bypass",
			MarshalCallback(CallbackData{
				Action:    "bypass",
				SubAction: "disable",
				Step:      2,
				Device:    deviceID,
			}),
		)
		rows = append(rows, []tgbotapi.InlineKeyboardButton{disableBtn})
	} else {
		// Show enable options with duration
		durations := []struct {
			minutes int
			label   string
		}{
			{60, "1 hour"},
			{120, "2 hours"},
			{480, "Until bedtime"},
			{bypassUntilEndOfDay, "Until end of day"},
		}

		for _, d := range durations {
			callback := MarshalCallback(CallbackData{
				Action:    "bypass",
				SubAction: "enable",
				Step:      2,
				Device:    deviceID,
				Duration:  d.minutes,
			})
			btn := tgbotapi.NewInlineKeyboardButtonData(
				"✅ "+d.label,
				callback,
			)
			rows = append(rows, []tgbotapi.InlineKeyboardButton{btn})
		}
	}

	// Back and Cancel buttons
	backBtn := tgbotapi.NewInlineKeyboardButtonData(
		"◀️ Back",
		MarshalCallback(CallbackData{Action: "bypass", Step: 0}),
	)
	cancelBtn := tgbotapi.NewInlineKeyboardButtonData(
		"❌ Cancel",
		MarshalCallback(CallbackData{Action: "cancel"}),
	)
	rows = append(rows, []tgbotapi.InlineKeyboardButton{backBtn, cancelBtn})

	return tgbotapi.NewInlineKeyboardMarkup(rows...)
}
