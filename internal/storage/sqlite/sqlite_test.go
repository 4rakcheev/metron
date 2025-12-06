package sqlite

import (
	"context"
	"metron/internal/core"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestDB(t *testing.T) *SQLiteStorage {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	storage, err := New(dbPath)
	require.NoError(t, err)

	t.Cleanup(func() {
		storage.Close()
	})

	return storage
}

func TestSQLiteStorage_Children(t *testing.T) {
	storage := setupTestDB(t)
	ctx := context.Background()

	// Test CreateChild
	child := &core.Child{
		ID:           "child1",
		Name:         "Alice",
		WeekdayLimit: 60,
		WeekendLimit: 120,
		BreakRule: &core.BreakRule{
			BreakAfterMinutes:   30,
			BreakDurationMinutes: 10,
		},
	}

	err := storage.CreateChild(ctx, child)
	require.NoError(t, err)

	// Test GetChild
	retrieved, err := storage.GetChild(ctx, "child1")
	require.NoError(t, err)
	assert.Equal(t, child.ID, retrieved.ID)
	assert.Equal(t, child.Name, retrieved.Name)
	assert.Equal(t, child.WeekdayLimit, retrieved.WeekdayLimit)
	assert.Equal(t, child.WeekendLimit, retrieved.WeekendLimit)
	require.NotNil(t, retrieved.BreakRule)
	assert.Equal(t, child.BreakRule.BreakAfterMinutes, retrieved.BreakRule.BreakAfterMinutes)

	// Test GetChild - not found
	_, err = storage.GetChild(ctx, "nonexistent")
	assert.ErrorIs(t, err, core.ErrChildNotFound)

	// Test ListChildren
	child2 := &core.Child{
		ID:           "child2",
		Name:         "Bob",
		WeekdayLimit: 45,
		WeekendLimit: 90,
	}
	err = storage.CreateChild(ctx, child2)
	require.NoError(t, err)

	children, err := storage.ListChildren(ctx)
	require.NoError(t, err)
	assert.Len(t, children, 2)

	// Test UpdateChild
	retrieved.Name = "Alice Updated"
	retrieved.WeekdayLimit = 70
	err = storage.UpdateChild(ctx, retrieved)
	require.NoError(t, err)

	updated, err := storage.GetChild(ctx, "child1")
	require.NoError(t, err)
	assert.Equal(t, "Alice Updated", updated.Name)
	assert.Equal(t, 70, updated.WeekdayLimit)

	// Test UpdateChild - not found
	nonExistent := &core.Child{
		ID:           "nonexistent",
		Name:         "Nobody",
		WeekdayLimit: 60,
		WeekendLimit: 120,
	}
	err = storage.UpdateChild(ctx, nonExistent)
	assert.ErrorIs(t, err, core.ErrChildNotFound)

	// Test DeleteChild
	err = storage.DeleteChild(ctx, "child2")
	require.NoError(t, err)

	_, err = storage.GetChild(ctx, "child2")
	assert.ErrorIs(t, err, core.ErrChildNotFound)

	// Test DeleteChild - not found
	err = storage.DeleteChild(ctx, "nonexistent")
	assert.ErrorIs(t, err, core.ErrChildNotFound)
}

func TestSQLiteStorage_Sessions(t *testing.T) {
	storage := setupTestDB(t)
	ctx := context.Background()

	// Create test children first
	child1 := &core.Child{
		ID:           "child1",
		Name:         "Alice",
		WeekdayLimit: 60,
		WeekendLimit: 120,
	}
	err := storage.CreateChild(ctx, child1)
	require.NoError(t, err)

	child2 := &core.Child{
		ID:           "child2",
		Name:         "Bob",
		WeekdayLimit: 45,
		WeekendLimit: 90,
	}
	err = storage.CreateChild(ctx, child2)
	require.NoError(t, err)

	// Test CreateSession
	now := time.Now()
	session := &core.Session{
		ID:               "session1",
		DeviceType:       "tv",
		DeviceID:         "tv1",
		ChildIDs:         []string{"child1", "child2"},
		StartTime:        now,
		ExpectedDuration: 30,
		RemainingMinutes: 30,
		Status:           core.SessionStatusActive,
	}

	err = storage.CreateSession(ctx, session)
	require.NoError(t, err)

	// Test GetSession
	retrieved, err := storage.GetSession(ctx, "session1")
	require.NoError(t, err)
	assert.Equal(t, session.ID, retrieved.ID)
	assert.Equal(t, session.DeviceType, retrieved.DeviceType)
	assert.Equal(t, session.DeviceID, retrieved.DeviceID)
	assert.Equal(t, session.ExpectedDuration, retrieved.ExpectedDuration)
	assert.Equal(t, session.RemainingMinutes, retrieved.RemainingMinutes)
	assert.Equal(t, session.Status, retrieved.Status)
	assert.Len(t, retrieved.ChildIDs, 2)
	assert.Contains(t, retrieved.ChildIDs, "child1")
	assert.Contains(t, retrieved.ChildIDs, "child2")

	// Test GetSession - not found
	_, err = storage.GetSession(ctx, "nonexistent")
	assert.ErrorIs(t, err, core.ErrSessionNotFound)

	// Test ListActiveSessions
	activeSessions, err := storage.ListActiveSessions(ctx)
	require.NoError(t, err)
	assert.Len(t, activeSessions, 1)
	assert.Equal(t, "session1", activeSessions[0].ID)

	// Create another session (completed)
	session2 := &core.Session{
		ID:               "session2",
		DeviceType:       "tv",
		DeviceID:         "tv1",
		ChildIDs:         []string{"child1"},
		StartTime:        now.Add(-1 * time.Hour),
		ExpectedDuration: 30,
		RemainingMinutes: 0,
		Status:           core.SessionStatusCompleted,
	}
	err = storage.CreateSession(ctx, session2)
	require.NoError(t, err)

	// Active sessions should still be 1
	activeSessions, err = storage.ListActiveSessions(ctx)
	require.NoError(t, err)
	assert.Len(t, activeSessions, 1)

	// Test ListSessionsByChild
	child1Sessions, err := storage.ListSessionsByChild(ctx, "child1")
	require.NoError(t, err)
	assert.Len(t, child1Sessions, 2)

	child2Sessions, err := storage.ListSessionsByChild(ctx, "child2")
	require.NoError(t, err)
	assert.Len(t, child2Sessions, 1)

	// Test UpdateSession
	retrieved.RemainingMinutes = 20
	retrieved.Status = core.SessionStatusPaused
	breakTime := time.Now()
	retrieved.LastBreakAt = &breakTime
	err = storage.UpdateSession(ctx, retrieved)
	require.NoError(t, err)

	updated, err := storage.GetSession(ctx, "session1")
	require.NoError(t, err)
	assert.Equal(t, 20, updated.RemainingMinutes)
	assert.Equal(t, core.SessionStatusPaused, updated.Status)
	require.NotNil(t, updated.LastBreakAt)

	// Test UpdateSession - not found
	nonExistent := &core.Session{
		ID:               "nonexistent",
		DeviceType:       "tv",
		DeviceID:         "tv1",
		ChildIDs:         []string{"child1"},
		StartTime:        now,
		ExpectedDuration: 30,
		RemainingMinutes: 30,
		Status:           core.SessionStatusActive,
	}
	err = storage.UpdateSession(ctx, nonExistent)
	assert.ErrorIs(t, err, core.ErrSessionNotFound)

	// Test DeleteSession
	err = storage.DeleteSession(ctx, "session2")
	require.NoError(t, err)

	_, err = storage.GetSession(ctx, "session2")
	assert.ErrorIs(t, err, core.ErrSessionNotFound)

	// Test DeleteSession - not found
	err = storage.DeleteSession(ctx, "nonexistent")
	assert.ErrorIs(t, err, core.ErrSessionNotFound)
}

func TestSQLiteStorage_DailyUsage(t *testing.T) {
	storage := setupTestDB(t)
	ctx := context.Background()

	// Create test child
	child := &core.Child{
		ID:           "child1",
		Name:         "Alice",
		WeekdayLimit: 60,
		WeekendLimit: 120,
	}
	err := storage.CreateChild(ctx, child)
	require.NoError(t, err)

	today := time.Now()

	// Test GetDailyUsage - no usage yet
	usage, err := storage.GetDailyUsage(ctx, "child1", today)
	require.NoError(t, err)
	assert.Equal(t, "child1", usage.ChildID)
	assert.Equal(t, 0, usage.MinutesUsed)
	assert.Equal(t, 0, usage.SessionCount)

	// Test UpdateDailyUsage
	usage.MinutesUsed = 30
	usage.SessionCount = 1
	err = storage.UpdateDailyUsage(ctx, usage)
	require.NoError(t, err)

	// Verify update
	updated, err := storage.GetDailyUsage(ctx, "child1", today)
	require.NoError(t, err)
	assert.Equal(t, 30, updated.MinutesUsed)
	assert.Equal(t, 1, updated.SessionCount)

	// Test IncrementDailyUsage
	err = storage.IncrementDailyUsage(ctx, "child1", today, 15)
	require.NoError(t, err)

	incremented, err := storage.GetDailyUsage(ctx, "child1", today)
	require.NoError(t, err)
	assert.Equal(t, 45, incremented.MinutesUsed)

	// Test with different date
	yesterday := today.Add(-24 * time.Hour)
	yesterdayUsage, err := storage.GetDailyUsage(ctx, "child1", yesterday)
	require.NoError(t, err)
	assert.Equal(t, 0, yesterdayUsage.MinutesUsed)
}

func TestSQLiteStorage_ForeignKeyConstraints(t *testing.T) {
	storage := setupTestDB(t)
	ctx := context.Background()

	// Create two children
	child1 := &core.Child{
		ID:           "child1",
		Name:         "Alice",
		WeekdayLimit: 60,
		WeekendLimit: 120,
	}
	err := storage.CreateChild(ctx, child1)
	require.NoError(t, err)

	child2 := &core.Child{
		ID:           "child2",
		Name:         "Bob",
		WeekdayLimit: 45,
		WeekendLimit: 90,
	}
	err = storage.CreateChild(ctx, child2)
	require.NoError(t, err)

	// Create session with both children
	session := &core.Session{
		ID:               "session1",
		DeviceType:       "tv",
		DeviceID:         "tv1",
		ChildIDs:         []string{"child1", "child2"},
		StartTime:        time.Now(),
		ExpectedDuration: 30,
		RemainingMinutes: 30,
		Status:           core.SessionStatusActive,
	}
	err = storage.CreateSession(ctx, session)
	require.NoError(t, err)

	// Verify session exists with both children
	retrieved, err := storage.GetSession(ctx, "session1")
	require.NoError(t, err)
	assert.Len(t, retrieved.ChildIDs, 2)

	// Delete child1 - CASCADE should remove the session_children entry
	err = storage.DeleteChild(ctx, "child1")
	require.NoError(t, err)

	// Session should still exist but only have child2
	retrieved, err = storage.GetSession(ctx, "session1")
	require.NoError(t, err)
	assert.Len(t, retrieved.ChildIDs, 1)
	assert.Equal(t, "child2", retrieved.ChildIDs[0])
}
