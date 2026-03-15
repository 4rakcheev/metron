package notify

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"metron/internal/core"
	"metron/internal/devices"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockSender records all sent messages.
type mockSender struct {
	messages []sentMessage
	failErr  error
}

type sentMessage struct {
	ChatID      int64
	Text        string
	ReplyMarkup interface{}
}

func (m *mockSender) SendMessage(_ context.Context, chatID int64, text string, replyMarkup interface{}) error {
	if m.failErr != nil {
		return m.failErr
	}
	m.messages = append(m.messages, sentMessage{
		ChatID:      chatID,
		Text:        text,
		ReplyMarkup: replyMarkup,
	})
	return nil
}

// mockChildLookup returns children from a map.
type mockChildLookup struct {
	children map[string]*core.Child
}

func (m *mockChildLookup) GetChild(_ context.Context, id string) (*core.Child, error) {
	child, ok := m.children[id]
	if !ok {
		return nil, core.ErrChildNotFound
	}
	return child, nil
}

func setupTestDriver(t *testing.T) (*Driver, *mockSender, *devices.Registry) {
	t.Helper()

	sender := &mockSender{}
	childLookup := &mockChildLookup{
		children: map[string]*core.Child{
			"child1": {ID: "child1", Name: "Masha", Emoji: "\U0001f467"},
			"child2": {ID: "child2", Name: "Petya", Emoji: "\U0001f466"},
		},
	}

	deviceReg := devices.NewRegistry()
	err := deviceReg.Register(&devices.Device{
		ID:     "phone1",
		Name:   "Android Phone",
		Type:   "phone",
		Emoji:  "\U0001f4f1",
		Driver: DriverName,
		Parameters: map[string]interface{}{
			"app_url":  "https://familylink.google.com",
			"app_name": "Family Link",
		},
	})
	require.NoError(t, err)

	driver := &Driver{
		config: Config{
			TelegramToken: "test-token",
			ChatIDs:       []int64{111, 222},
		},
		childLookup:    childLookup,
		deviceRegistry: deviceReg,
		sender:         sender,
		logger:         noopLogger(),
	}

	return driver, sender, deviceReg
}

func noopLogger() *slog.Logger {
	return slog.Default()
}

func testSession() *core.Session {
	return &core.Session{
		ID:               "sess1",
		DeviceID:         "phone1",
		DeviceType:       "phone",
		ChildIDs:         []string{"child1"},
		StartTime:        time.Now(),
		ExpectedDuration: 30,
		Status:           core.SessionStatusActive,
	}
}

func TestStartSession_ChildInitiated(t *testing.T) {
	driver, sender, _ := setupTestDriver(t)

	session := testSession()
	ctx := context.Background()

	err := driver.StartSession(ctx, session)
	assert.NoError(t, err)

	// Should send to all chat IDs
	assert.Len(t, sender.messages, 2)

	msg := sender.messages[0]
	assert.Equal(t, int64(111), msg.ChatID)
	assert.Contains(t, msg.Text, "Session Request")
	assert.Contains(t, msg.Text, "Masha requested 30 min")
	assert.Contains(t, msg.Text, "Android Phone")
	assert.Contains(t, msg.Text, "Please grant time in Family Link")

	// Should have URL button
	assert.NotNil(t, msg.ReplyMarkup)
}

func TestStartSession_ParentOverride(t *testing.T) {
	driver, sender, _ := setupTestDriver(t)

	session := testSession()
	ctx := context.WithValue(context.Background(), "parent_override", true)

	err := driver.StartSession(ctx, session)
	assert.NoError(t, err)

	assert.Len(t, sender.messages, 2)

	msg := sender.messages[0]
	assert.Contains(t, msg.Text, "Session Started")
	assert.Contains(t, msg.Text, "Don't forget to grant time")
	assert.NotContains(t, msg.Text, "requested")
}

func TestStopSession(t *testing.T) {
	driver, sender, _ := setupTestDriver(t)

	session := testSession()
	session.StartTime = time.Now().Add(-28 * time.Minute)

	err := driver.StopSession(context.Background(), session)
	assert.NoError(t, err)

	assert.Len(t, sender.messages, 2)

	msg := sender.messages[0]
	assert.Contains(t, msg.Text, "Session Ended")
	assert.Contains(t, msg.Text, "Masha")
	assert.Contains(t, msg.Text, "Android Phone")
	assert.Contains(t, msg.Text, "Revoke bonus time in Family Link")
	assert.NotNil(t, msg.ReplyMarkup)
}

func TestApplyWarning(t *testing.T) {
	driver, sender, _ := setupTestDriver(t)

	session := testSession()

	err := driver.ApplyWarning(context.Background(), session, 5)
	assert.NoError(t, err)

	assert.Len(t, sender.messages, 2)

	msg := sender.messages[0]
	assert.Contains(t, msg.Text, "5 min remaining")
	assert.Contains(t, msg.Text, "Masha")
	assert.Contains(t, msg.Text, "Android Phone")
	assert.Nil(t, msg.ReplyMarkup)
}

func TestNotificationFailure_DoesNotBlockSession(t *testing.T) {
	driver, sender, _ := setupTestDriver(t)
	sender.failErr = errors.New("telegram unavailable")

	session := testSession()

	// All methods should return nil even when sender fails
	assert.NoError(t, driver.StartSession(context.Background(), session))
	assert.NoError(t, driver.StopSession(context.Background(), session))
	assert.NoError(t, driver.ApplyWarning(context.Background(), session, 5))
}

func TestMultipleChatIDs(t *testing.T) {
	driver, sender, _ := setupTestDriver(t)

	session := testSession()
	err := driver.StartSession(context.Background(), session)
	assert.NoError(t, err)

	assert.Len(t, sender.messages, 2)
	assert.Equal(t, int64(111), sender.messages[0].ChatID)
	assert.Equal(t, int64(222), sender.messages[1].ChatID)
}

func TestChildNameResolution(t *testing.T) {
	driver, sender, _ := setupTestDriver(t)

	// Session with multiple children
	session := testSession()
	session.ChildIDs = []string{"child1", "child2"}

	err := driver.StartSession(context.Background(), session)
	assert.NoError(t, err)

	msg := sender.messages[0]
	assert.Contains(t, msg.Text, "Masha, Petya")
}

func TestChildNameResolution_UnknownChild(t *testing.T) {
	driver, sender, _ := setupTestDriver(t)

	session := testSession()
	session.ChildIDs = []string{"unknown-child"}

	err := driver.StartSession(context.Background(), session)
	assert.NoError(t, err)

	// Falls back to child ID
	msg := sender.messages[0]
	assert.Contains(t, msg.Text, "unknown-child")
}

func TestGetLiveState(t *testing.T) {
	driver, _, _ := setupTestDriver(t)

	state, err := driver.GetLiveState(context.Background(), "phone1")
	assert.NoError(t, err)
	assert.Nil(t, state)
}

func TestCapabilities(t *testing.T) {
	driver, _, _ := setupTestDriver(t)

	caps := driver.Capabilities()
	assert.True(t, caps.SupportsWarnings)
	assert.False(t, caps.SupportsLiveState)
	assert.True(t, caps.SupportsScheduling)
}

func TestName(t *testing.T) {
	driver, _, _ := setupTestDriver(t)
	assert.Equal(t, "notify", driver.Name())
}

func TestNoAppURL_NoButton(t *testing.T) {
	driver, sender, deviceReg := setupTestDriver(t)

	// Register a device without app_url
	err := deviceReg.Register(&devices.Device{
		ID:     "phone2",
		Name:   "Basic Phone",
		Type:   "phone",
		Driver: DriverName,
		Parameters: map[string]interface{}{
			"app_name": "Some App",
		},
	})
	require.NoError(t, err)

	session := testSession()
	session.DeviceID = "phone2"

	err = driver.StartSession(context.Background(), session)
	assert.NoError(t, err)

	msg := sender.messages[0]
	assert.Nil(t, msg.ReplyMarkup)
}
