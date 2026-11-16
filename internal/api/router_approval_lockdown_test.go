package api

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"metron/internal/core"
	"metron/internal/devices"
	"metron/internal/drivers"
	"metron/internal/drivers/passive"
	"metron/internal/storage/sqlite"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testAPIKey = "test-key"

var allWeekDays = []string{"sunday", "monday", "tuesday", "wednesday", "thursday", "friday", "saturday"}

type testDeviceRegistry struct{ r *devices.Registry }

func (t *testDeviceRegistry) Get(id string) (core.Device, error) {
	d, err := t.r.Get(id)
	if err != nil {
		return nil, err
	}
	return d, nil
}

type testDriverRegistry struct{ r *drivers.Registry }

func (t *testDriverRegistry) Get(name string) (core.DeviceDriver, error) {
	d, err := t.r.Get(name)
	if err != nil {
		return nil, err
	}
	return d, nil
}

type testEnv struct {
	router  *gin.Engine
	store   *sqlite.SQLiteStorage
	manager *core.SessionManager
}

// newTestEnv builds the full router with SQLite, a real session manager and two passive devices.
// gateDays nil disables the approval gate.
func newTestEnv(t *testing.T, gateDays []string) *testEnv {
	t.Helper()
	gin.SetMode(gin.TestMode)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	store, err := sqlite.New(filepath.Join(t.TempDir(), "test.db"), time.UTC)
	require.NoError(t, err)
	t.Cleanup(func() { _ = store.Close() })

	ctx := context.Background()
	for _, c := range []*core.Child{
		{ID: "simon", Name: "Simon", PIN: "1111", WeekdayLimit: 60, WeekendLimit: 60},
		{ID: "alisa", Name: "Alisa", PIN: "2222", WeekdayLimit: 60, WeekendLimit: 60},
	} {
		require.NoError(t, store.CreateChild(ctx, c))
	}

	driverRegistry := drivers.NewRegistry()
	require.NoError(t, driverRegistry.Register(passive.NewDriver(logger)))
	deviceRegistry := devices.NewRegistry()
	require.NoError(t, deviceRegistry.Register(&devices.Device{ID: "tv1", Name: "TV", Type: "tv", Driver: "passive"}))
	require.NoError(t, deviceRegistry.Register(&devices.Device{ID: "ps51", Name: "PS5", Type: "game", Driver: "passive"}))

	manager := core.NewSessionManager(store, &testDeviceRegistry{deviceRegistry}, &testDriverRegistry{driverRegistry},
		core.NewTimeCalculationService(store, time.UTC), core.NewDowntimeService(nil, time.UTC), time.UTC, logger)
	manager.SetLockdownStorage(store)

	var gate *core.ApprovalGateService
	if gateDays != nil {
		gate = core.NewApprovalGateService(gateDays, store, time.UTC)
		manager.SetApprovalGate(gate)
	}

	router := NewRouter(RouterConfig{
		Storage:         store,
		Manager:         manager,
		DriverRegistry:  driverRegistry,
		DeviceRegistry:  deviceRegistry,
		LockdownStorage: store,
		ApprovalGate:    gate,
		Timezone:        time.UTC,
		APIKey:          testAPIKey,
		Logger:          logger,
	})
	return &testEnv{router: router, store: store, manager: manager}
}

func (e *testEnv) do(t *testing.T, method, path string, body interface{}, headers map[string]string) (*httptest.ResponseRecorder, map[string]interface{}) {
	t.Helper()
	var reader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		require.NoError(t, err)
		reader = bytes.NewReader(data)
	}
	req := httptest.NewRequest(method, path, reader)
	if body != nil || method == http.MethodPost {
		req.Header.Set("Content-Type", "application/json")
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	e.router.ServeHTTP(w, req)

	var out map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &out)
	return w, out
}

func (e *testEnv) admin(t *testing.T, method, path string, body interface{}) (*httptest.ResponseRecorder, map[string]interface{}) {
	return e.do(t, method, path, body, map[string]string{"X-Metron-Key": testAPIKey})
}

// childToken logs a child in with their PIN and returns the Bearer token
func (e *testEnv) childToken(t *testing.T, childID, pin string) string {
	t.Helper()
	w, out := e.do(t, http.MethodPost, "/child/auth/login", map[string]string{"child_id": childID, "pin": pin}, nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	token, _ := out["session_id"].(string)
	require.NotEmpty(t, token, "login response: %s", w.Body.String())
	return token
}

func (e *testEnv) child(t *testing.T, token, method, path string, body interface{}) (*httptest.ResponseRecorder, map[string]interface{}) {
	return e.do(t, method, path, body, map[string]string{"Authorization": "Bearer " + token})
}

func TestApprovalGate_ChildNeedsApprovalParentDoesNot(t *testing.T) {
	env := newTestEnv(t, allWeekDays)
	token := env.childToken(t, "simon", "1111")

	// Child app shows the gate state
	w, today := env.child(t, token, http.MethodGet, "/child/today", nil)
	require.Equal(t, http.StatusOK, w.Code)
	approval := today["approval"].(map[string]interface{})
	assert.Equal(t, true, approval["required"])
	assert.Equal(t, false, approval["approved"])

	// Child cannot start a session
	w, out := env.child(t, token, http.MethodPost, "/child/sessions", map[string]interface{}{"device_id": "tv1", "minutes": 10})
	assert.Equal(t, http.StatusForbidden, w.Code, w.Body.String())
	assert.Equal(t, "APPROVAL_REQUIRED", out["code"])

	// Parent can start a session without approval
	w, _ = env.admin(t, http.MethodPost, "/v1/sessions", map[string]interface{}{"device_id": "ps51", "child_ids": []string{"alisa"}, "minutes": 10})
	assert.Equal(t, http.StatusCreated, w.Code, w.Body.String())

	// Parent approves Simon
	w, out = env.admin(t, http.MethodPost, "/v1/children/simon/approval", map[string]string{"approved_by": "telegram"})
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	assert.Equal(t, true, out["approved"])

	// Approvals list reflects it
	w, out = env.admin(t, http.MethodGet, "/v1/approvals/today", nil)
	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, true, out["required"])
	approvedByChild := map[string]bool{}
	for _, item := range out["children"].([]interface{}) {
		m := item.(map[string]interface{})
		approvedByChild[m["child_id"].(string)] = m["approved"].(bool)
	}
	assert.Equal(t, map[string]bool{"simon": true, "alisa": false}, approvedByChild)

	// Now the child can start
	w, _ = env.child(t, token, http.MethodPost, "/child/sessions", map[string]interface{}{"device_id": "tv1", "minutes": 10})
	assert.Equal(t, http.StatusCreated, w.Code, w.Body.String())

	// Revoke
	w, _ = env.admin(t, http.MethodDelete, "/v1/children/simon/approval", nil)
	assert.Equal(t, http.StatusNoContent, w.Code)
	w, today = env.child(t, token, http.MethodGet, "/child/today", nil)
	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, false, today["approval"].(map[string]interface{})["approved"])
}

func TestApprovalGate_DisabledMeansNoRoutesAndNoGate(t *testing.T) {
	env := newTestEnv(t, nil)
	token := env.childToken(t, "simon", "1111")

	w, today := env.child(t, token, http.MethodGet, "/child/today", nil)
	require.Equal(t, http.StatusOK, w.Code)
	assert.NotContains(t, today, "approval")

	w, _ = env.child(t, token, http.MethodPost, "/child/sessions", map[string]interface{}{"device_id": "tv1", "minutes": 10})
	assert.Equal(t, http.StatusCreated, w.Code, w.Body.String())

	w, _ = env.admin(t, http.MethodGet, "/v1/approvals/today", nil)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestLockdown_ScopedRuleStopsOnlyMatchingSession(t *testing.T) {
	env := newTestEnv(t, nil)

	w, simonPS5 := env.admin(t, http.MethodPost, "/v1/sessions", map[string]interface{}{"device_id": "ps51", "child_ids": []string{"simon"}, "minutes": 30})
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	w, alisaTV := env.admin(t, http.MethodPost, "/v1/sessions", map[string]interface{}{"device_id": "tv1", "child_ids": []string{"alisa"}, "minutes": 30})
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())

	// Lock Simon on PS5 until end of day
	w, rule := env.admin(t, http.MethodPost, "/v1/lockdowns", map[string]string{
		"child_id": "simon", "device_id": "ps51", "until": "end_of_day", "enabled_by": "telegram",
	})
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	assert.Equal(t, float64(1), rule["stopped_sessions"])
	assert.Equal(t, "Simon", rule["child_name"])
	assert.Equal(t, "PS5", rule["device_name"])
	assert.NotNil(t, rule["expires_at"])

	ctx := context.Background()
	s1, err := env.store.GetSession(ctx, simonPS5["id"].(string))
	require.NoError(t, err)
	assert.False(t, s1.IsActive())
	s2, err := env.store.GetSession(ctx, alisaTV["id"].(string))
	require.NoError(t, err)
	assert.True(t, s2.IsActive())

	// Simon cannot start PS5 (child app 403, parent API 409), but can use TV
	token := env.childToken(t, "simon", "1111")
	w, out := env.child(t, token, http.MethodPost, "/child/sessions", map[string]interface{}{"device_id": "ps51", "minutes": 10})
	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.Equal(t, "LOCKDOWN_ACTIVE", out["code"])
	w, out = env.admin(t, http.MethodPost, "/v1/sessions", map[string]interface{}{"device_id": "ps51", "child_ids": []string{"simon"}, "minutes": 10})
	assert.Equal(t, http.StatusConflict, w.Code)
	assert.Equal(t, "LOCKDOWN_ACTIVE", out["code"])
	w, _ = env.child(t, token, http.MethodPost, "/child/sessions", map[string]interface{}{"device_id": "tv1", "minutes": 10})
	assert.Equal(t, http.StatusCreated, w.Code, w.Body.String())

	// Child device list shows PS5 as locked; /child/today is not a full lockdown
	req := httptest.NewRequest(http.MethodGet, "/child/devices", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	env.router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
	var devicesList []map[string]interface{}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &devicesList))
	locked := map[string]bool{}
	for _, d := range devicesList {
		locked[d["id"].(string)] = d["locked"].(bool)
	}
	assert.Equal(t, map[string]bool{"tv1": false, "ps51": true}, locked)
	_, today := env.child(t, token, http.MethodGet, "/child/today", nil)
	assert.Equal(t, false, today["lockdown"])

	// List and delete
	req = httptest.NewRequest(http.MethodGet, "/v1/lockdowns", nil)
	req.Header.Set("X-Metron-Key", testAPIKey)
	rec = httptest.NewRecorder()
	env.router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
	var rules []map[string]interface{}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &rules))
	require.Len(t, rules, 1)

	id := int64(rule["id"].(float64))
	w, _ = env.admin(t, http.MethodDelete, "/v1/lockdowns/"+jsonInt(id), nil)
	assert.Equal(t, http.StatusNoContent, w.Code)
	w, _ = env.admin(t, http.MethodDelete, "/v1/lockdowns/"+jsonInt(id), nil)
	assert.Equal(t, http.StatusNotFound, w.Code)

	w, _ = env.child(t, token, http.MethodPost, "/child/sessions", map[string]interface{}{"device_id": "ps51", "minutes": 10})
	assert.Equal(t, http.StatusCreated, w.Code, w.Body.String())
}

func TestLockdown_GlobalRuleAndValidation(t *testing.T) {
	env := newTestEnv(t, nil)

	// Empty body = all children, all devices, manual
	w, rule := env.admin(t, http.MethodPost, "/v1/lockdowns", nil)
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	assert.Nil(t, rule["child_id"])
	assert.Nil(t, rule["device_id"])
	assert.Nil(t, rule["expires_at"])

	token := env.childToken(t, "alisa", "2222")
	_, today := env.child(t, token, http.MethodGet, "/child/today", nil)
	assert.Equal(t, true, today["lockdown"])

	w, _ = env.admin(t, http.MethodPost, "/v1/lockdowns", map[string]string{"child_id": "nobody"})
	assert.Equal(t, http.StatusNotFound, w.Code)
	w, _ = env.admin(t, http.MethodPost, "/v1/lockdowns", map[string]string{"device_id": "xbox"})
	assert.Equal(t, http.StatusNotFound, w.Code)
	w, _ = env.admin(t, http.MethodPost, "/v1/lockdowns", map[string]string{"until": "forever"})
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func jsonInt(v int64) string {
	b, _ := json.Marshal(v)
	return string(b)
}
