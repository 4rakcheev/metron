package bot

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// maxDeviceID is the longest device ID allowed by config (15 chars)
const maxDeviceID = "abcdefghijklmno"

func strPtr(s string) *string { return &s }

// assertCallbacksFit fails if any callback in the keyboard is over Telegram's 64-byte limit
func assertCallbacksFit(t *testing.T, name string, keyboard tgbotapi.InlineKeyboardMarkup) {
	t.Helper()
	for _, row := range keyboard.InlineKeyboard {
		for _, btn := range row {
			if btn.CallbackData == nil {
				continue
			}
			if n := len(*btn.CallbackData); n > 64 {
				t.Errorf("%s: button %q callback is %d bytes (max 64): %s", name, btn.Text, n, *btn.CallbackData)
			}
		}
	}
}

func TestLockdownAndApprovalKeyboards_CallbackSize(t *testing.T) {
	if len(maxDeviceID) != 15 {
		t.Fatalf("test device ID must be 15 chars, got %d", len(maxDeviceID))
	}

	children := make([]Child, 100)
	approvals := make([]ChildApproval, 100)
	for i := range children {
		children[i] = Child{ID: "child-with-a-long-identifier", Name: "Child", Emoji: "🧒"}
		approvals[i] = ChildApproval{ChildID: children[i].ID, ChildName: "Child", Approved: i%2 == 0}
	}
	devices := []Device{{ID: maxDeviceID, Name: "Device"}}
	rules := []LockdownRule{
		{ID: math.MaxInt64, ChildID: strPtr("simon"), ChildName: "Simon", DeviceID: strPtr(maxDeviceID), DeviceName: "Device"},
		{ID: 1, ChildName: "All children", DeviceName: "All devices"},
	}

	keyboards := map[string]tgbotapi.InlineKeyboardMarkup{
		"more menu":              BuildMoreMenuButtons(true, 99),
		"main menu":              BuildMainMenuButtons(),
		"lockdown list":          BuildLockdownListButtons(rules),
		"lockdown child":         BuildLockdownChildButtons(children),
		"lockdown device (all)":  BuildLockdownDeviceButtons(devices, lockdownAllChildren),
		"lockdown device (99)":   BuildLockdownDeviceButtons(devices, 99),
		"lockdown duration max":  BuildLockdownDurationButtons(lockdownAllChildren, maxDeviceID),
		"lockdown duration 99":   BuildLockdownDurationButtons(99, maxDeviceID),
		"lockdown duration all":  BuildLockdownDurationButtons(lockdownAllChildren, lockdownAllDevices),
		"lockdown done":          BuildLockdownDoneButtons(),
		"approval":               BuildApprovalButtons(approvals),
		"approval revoke (99)":   BuildApprovalRevokeConfirmButtons(99),
		"approval revoke (9999)": BuildApprovalRevokeConfirmButtons(9999),
	}
	for name, kb := range keyboards {
		assertCallbacksFit(t, name, kb)
	}
}

// findCallback returns the parsed callback of the first button whose text contains label
func findCallback(t *testing.T, keyboard tgbotapi.InlineKeyboardMarkup, label string) *CallbackData {
	t.Helper()
	for _, row := range keyboard.InlineKeyboard {
		for _, btn := range row {
			if btn.CallbackData != nil && strings.Contains(btn.Text, label) {
				data, err := UnmarshalCallback(*btn.CallbackData)
				if err != nil {
					t.Fatalf("bad callback for %q: %v", btn.Text, err)
				}
				return data
			}
		}
	}
	t.Fatalf("no button with label containing %q", label)
	return nil
}

func TestLockdownChildEncoding_AllIsDistinctFromFirstChild(t *testing.T) {
	children := []Child{{ID: "simon", Name: "Simon"}, {ID: "anna", Name: "Anna"}}
	kb := BuildLockdownChildButtons(children)

	all := findCallback(t, kb, "All children")
	if all.Action != lockdownAction || all.Step != 2 || all.ChildIndex != lockdownAllChildren {
		t.Errorf("All children callback = %+v", all)
	}
	first := findCallback(t, kb, "Simon")
	if first.ChildIndex != 0 || first.Step != 2 {
		t.Errorf("first child callback = %+v", first)
	}
	second := findCallback(t, kb, "Anna")
	if second.ChildIndex != 1 {
		t.Errorf("second child callback = %+v", second)
	}

	// The child selection must survive the device and duration steps
	devKb := BuildLockdownDeviceButtons([]Device{{ID: "ps51", Name: "PS5"}}, lockdownAllChildren)
	allDev := findCallback(t, devKb, "All devices")
	if allDev.ChildIndex != lockdownAllChildren || allDev.Device != lockdownAllDevices || allDev.Step != 3 {
		t.Errorf("All devices callback = %+v", allDev)
	}
	ps5 := findCallback(t, devKb, "PS5")
	if ps5.ChildIndex != lockdownAllChildren || ps5.Device != "ps51" {
		t.Errorf("PS5 callback = %+v", ps5)
	}

	durKb := BuildLockdownDurationButtons(0, "ps51")
	eod := findCallback(t, durKb, "end of day")
	if eod.Step != 4 || eod.ChildIndex != 0 || eod.Device != "ps51" || eod.Duration != lockdownUntilEndOfDayCode {
		t.Errorf("end of day callback = %+v", eod)
	}
	manual := findCallback(t, durKb, "Until I unlock")
	if manual.Duration != lockdownUntilManualCode {
		t.Errorf("manual callback = %+v", manual)
	}
}

func TestBuildLockdownListButtons_UnlockCarriesRuleID(t *testing.T) {
	rules := []LockdownRule{{ID: 42, ChildID: strPtr("simon"), ChildName: "Simon", DeviceName: "All devices"}}
	kb := BuildLockdownListButtons(rules)

	unlock := findCallback(t, kb, "Unlock Simon: all devices")
	if unlock.SubAction != lockdownSubUnlock || unlock.RuleID != 42 {
		t.Errorf("unlock callback = %+v", unlock)
	}
	if n := findCallback(t, kb, "New lockdown"); n.Step != 1 {
		t.Errorf("new lockdown callback = %+v", n)
	}
	if back := findCallback(t, kb, "Back"); back.Action != "more_menu" {
		t.Errorf("back callback = %+v", back)
	}
}

func TestBuildMoreMenuButtons_LockdownLabel(t *testing.T) {
	findCallback(t, BuildMoreMenuButtons(false, 0), "🔒 Lockdowns")
	if data := findCallback(t, BuildMoreMenuButtons(false, 3), "Lockdowns (3 active)"); data.Action != lockdownAction {
		t.Errorf("lockdown button action = %q", data.Action)
	}
}

func TestBuildApprovalButtons(t *testing.T) {
	kb := BuildApprovalButtons([]ChildApproval{
		{ChildID: "simon", ChildName: "Simon", Approved: false},
		{ChildID: "anna", ChildName: "Anna", Approved: true},
	})
	approve := findCallback(t, kb, "Approve Simon")
	if approve.SubAction != approvalSubApprove || approve.ChildIndex != 0 {
		t.Errorf("approve callback = %+v", approve)
	}
	revoke := findCallback(t, kb, "Revoke Anna")
	if revoke.SubAction != approvalSubRevoke || revoke.ChildIndex != 1 || revoke.Step != 0 {
		t.Errorf("revoke callback = %+v", revoke)
	}
	confirm := findCallback(t, BuildApprovalRevokeConfirmButtons(1), "Yes, revoke")
	if confirm.SubAction != approvalSubRevoke || confirm.Step != 1 || confirm.ChildIndex != 1 {
		t.Errorf("confirm callback = %+v", confirm)
	}
}

func TestFormatLockdownLines(t *testing.T) {
	riga, err := time.LoadLocation("Europe/Riga")
	if err != nil {
		t.Skipf("timezone data unavailable: %v", err)
	}
	prev := timezone
	timezone = riga
	t.Cleanup(func() { timezone = prev })

	now := time.Date(2026, 11, 16, 15, 0, 0, 0, riga)
	midnight := time.Date(2026, 11, 17, 0, 0, 0, 0, riga).UTC().Format(time.RFC3339)
	later := time.Date(2026, 11, 18, 0, 0, 0, 0, riga).UTC().Format(time.RFC3339)

	rules := []LockdownRule{
		{ID: 1, ChildID: strPtr("simon"), ChildName: "Simon", DeviceID: strPtr("ps51"), DeviceName: "PS5", ExpiresAt: &midnight},
		{ID: 2, ChildName: "All children", DeviceName: "All devices"},
		{ID: 3, ChildID: strPtr("anna"), ChildName: "Anna_B", DeviceName: "All devices", ExpiresAt: &later},
	}
	got := FormatLockdownLines(rules, now)
	want := "• Simon: PS5 — until end of day\n" +
		"• All children: all devices — until unlocked\n" +
		"• Anna\\_B: all devices — until 18.11 00:00\n"
	if got != want {
		t.Errorf("FormatLockdownLines() =\n%q\nwant\n%q", got, want)
	}
	if !rules[1].IsGlobal() || rules[0].IsGlobal() || rules[2].IsGlobal() {
		t.Error("IsGlobal must be true only for the all/all rule")
	}
}

func TestFormatError_SessionBlockCodes(t *testing.T) {
	lockErr := &StatusError{StatusCode: http.StatusConflict, Code: "LOCKDOWN_ACTIVE",
		APIMessage: "lockdown active: sessions are blocked until unlocked (child simon on device ps51)"}
	got := FormatError(lockErr)
	if !strings.Contains(got, "Blocked by a lockdown") || !strings.Contains(got, "child simon on device ps51") {
		t.Errorf("unexpected lockdown error text: %q", got)
	}

	downErr := &StatusError{StatusCode: http.StatusConflict, Code: "DOWNTIME_ACTIVE", APIMessage: "x"}
	if got := FormatError(downErr); !strings.Contains(got, "Downtime is active") {
		t.Errorf("unexpected downtime error text: %q", got)
	}

	other := &StatusError{StatusCode: http.StatusBadRequest, Code: "INSUFFICIENT_TIME", Message: "API error 400: no time (INSUFFICIENT_TIME)"}
	if got := FormatError(other); !strings.Contains(got, "INSUFFICIENT\\_TIME") {
		t.Errorf("generic errors must stay escaped: %q", got)
	}
}

func TestFormatApprovals(t *testing.T) {
	notRequired := FormatApprovals(&TodayApprovals{Required: false}, "")
	if !strings.Contains(notRequired, "Approval is not required today (weekend)") {
		t.Errorf("missing not-required text: %q", notRequired)
	}

	at := time.Date(2026, 11, 16, 14, 5, 0, 0, time.UTC).Format(time.RFC3339)
	prev := timezone
	timezone = time.UTC
	t.Cleanup(func() { timezone = prev })

	got := FormatApprovals(&TodayApprovals{Required: true, Children: []ChildApproval{
		{ChildName: "Simon", Required: true},
		{ChildName: "Anna", Required: true, Approved: true, ApprovedAt: &at},
	}}, "✅ Anna can now use screen time today.")
	for _, want := range []string{"⏳ Simon — waiting", "✅ Anna — approved at 14:05", "Anna can now use screen time today."} {
		if !strings.Contains(got, want) {
			t.Errorf("FormatApprovals() missing %q in %q", want, got)
		}
	}
}

func TestIsFeatureDisabled(t *testing.T) {
	if !isFeatureDisabled(&StatusError{StatusCode: http.StatusNotFound}) {
		t.Error("404 without code must mean feature disabled")
	}
	if isFeatureDisabled(&StatusError{StatusCode: http.StatusNotFound, Code: "CHILD_NOT_FOUND"}) {
		t.Error("404 CHILD_NOT_FOUND must not mean feature disabled")
	}
	if isFeatureDisabled(&StatusError{StatusCode: http.StatusInternalServerError}) {
		t.Error("500 must not mean feature disabled")
	}
}

func TestMetronAPI_LockdownAndApprovalRequests(t *testing.T) {
	var gotCreate map[string]interface{}
	var gotDeletePath string

	mux := http.NewServeMux()
	mux.HandleFunc("/v1/lockdowns", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			_, _ = w.Write([]byte(`[{"id":3,"child_id":null,"child_name":"All children","device_id":"ps51",` +
				`"device_name":"PS5","reason":null,"enabled_at":"2026-11-16T10:00:00Z","enabled_by":"telegram","expires_at":null}]`))
			return
		}
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &gotCreate)
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":4,"child_id":null,"child_name":"All children","device_id":null,"device_name":"All devices",` +
			`"enabled_at":"2026-11-16T10:00:00Z","expires_at":null,"stopped_sessions":2,"failed_sessions":1,"message":"ok"}`))
	})
	mux.HandleFunc("/v1/lockdowns/", func(w http.ResponseWriter, r *http.Request) {
		gotDeletePath = r.Method + " " + r.URL.Path
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("/v1/children/missing/approval", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":"Child not found","code":"CHILD_NOT_FOUND"}`))
	})
	// /v1/approvals/today is not registered: the mux answers a plain-text 404 like gin does

	srv := httptest.NewServer(mux)
	defer srv.Close()

	api := NewMetronAPI(srv.URL, "key", slog.New(slog.NewTextHandler(io.Discard, nil)))
	ctx := context.Background()

	rules, err := api.ListLockdowns(ctx)
	if err != nil || len(rules) != 1 || rules[0].ChildID != nil || rules[0].DeviceID == nil || *rules[0].DeviceID != "ps51" {
		t.Fatalf("ListLockdowns() = %+v, %v", rules, err)
	}

	res, err := api.CreateLockdown(ctx, "", "", LockdownUntilEndOfDay)
	if err != nil {
		t.Fatalf("CreateLockdown() error: %v", err)
	}
	if res.ID != 4 || !res.IsGlobal() || res.StoppedSessions != 2 || res.FailedSessions != 1 {
		t.Errorf("CreateLockdown() = %+v", res)
	}
	if _, ok := gotCreate["child_id"]; ok {
		t.Errorf("child_id must be omitted for all children: %v", gotCreate)
	}
	if _, ok := gotCreate["device_id"]; ok {
		t.Errorf("device_id must be omitted for all devices: %v", gotCreate)
	}
	if gotCreate["until"] != "end_of_day" || gotCreate["enabled_by"] != "telegram" {
		t.Errorf("unexpected create body: %v", gotCreate)
	}

	if err := api.DeleteLockdown(ctx, math.MaxInt64); err != nil {
		t.Fatalf("DeleteLockdown() error: %v", err)
	}
	if gotDeletePath != "DELETE /v1/lockdowns/9223372036854775807" {
		t.Errorf("DeleteLockdown() request = %q", gotDeletePath)
	}

	if _, err := api.ListTodayApprovals(ctx); !isFeatureDisabled(err) {
		t.Errorf("ListTodayApprovals() on unregistered route: want feature-disabled error, got %v", err)
	}
	_, err = api.ApproveChild(ctx, "missing")
	if !isNotFound(err) || isFeatureDisabled(err) || apiErrorCode(err) != "CHILD_NOT_FOUND" {
		t.Errorf("ApproveChild(missing) error = %v", err)
	}
}
