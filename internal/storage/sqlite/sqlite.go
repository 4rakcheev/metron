package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"metron/internal/core"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// SQLiteStorage implements storage.Storage using SQLite
type SQLiteStorage struct {
	db *sql.DB
}

// New creates a new SQLite storage instance
func New(dbPath string) (*SQLiteStorage, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Enable foreign keys
	if _, err := db.Exec("PRAGMA foreign_keys = ON"); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to enable foreign keys: %w", err)
	}

	storage := &SQLiteStorage{db: db}

	if err := storage.migrate(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to migrate database: %w", err)
	}

	return storage, nil
}

// migrate creates the database schema
func (s *SQLiteStorage) migrate() error {
	schema := `
		CREATE TABLE IF NOT EXISTS children (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			weekday_limit INTEGER NOT NULL,
			weekend_limit INTEGER NOT NULL,
			break_rule TEXT,
			created_at DATETIME NOT NULL,
			updated_at DATETIME NOT NULL
		);

		CREATE TABLE IF NOT EXISTS sessions (
			id TEXT PRIMARY KEY,
			device_type TEXT NOT NULL,
			device_id TEXT NOT NULL,
			start_time DATETIME NOT NULL,
			expected_duration INTEGER NOT NULL,
			remaining_minutes INTEGER NOT NULL,
			status TEXT NOT NULL,
			last_break_at DATETIME,
			break_ends_at DATETIME,
			created_at DATETIME NOT NULL,
			updated_at DATETIME NOT NULL
		);

		CREATE TABLE IF NOT EXISTS session_children (
			session_id TEXT NOT NULL,
			child_id TEXT NOT NULL,
			PRIMARY KEY (session_id, child_id),
			FOREIGN KEY (session_id) REFERENCES sessions(id) ON DELETE CASCADE,
			FOREIGN KEY (child_id) REFERENCES children(id) ON DELETE CASCADE
		);

		CREATE TABLE IF NOT EXISTS daily_usage (
			child_id TEXT NOT NULL,
			date DATE NOT NULL,
			minutes_used INTEGER NOT NULL DEFAULT 0,
			session_count INTEGER NOT NULL DEFAULT 0,
			created_at DATETIME NOT NULL,
			updated_at DATETIME NOT NULL,
			PRIMARY KEY (child_id, date),
			FOREIGN KEY (child_id) REFERENCES children(id) ON DELETE CASCADE
		);

		CREATE INDEX IF NOT EXISTS idx_sessions_status ON sessions(status);
		CREATE INDEX IF NOT EXISTS idx_sessions_device ON sessions(device_type, device_id);
		CREATE INDEX IF NOT EXISTS idx_daily_usage_date ON daily_usage(date);
	`

	_, err := s.db.Exec(schema)
	return err
}

// CreateChild creates a new child
func (s *SQLiteStorage) CreateChild(ctx context.Context, child *core.Child) error {
	if err := child.Validate(); err != nil {
		return err
	}

	now := time.Now()
	child.CreatedAt = now
	child.UpdatedAt = now

	var breakRuleJSON sql.NullString
	if child.BreakRule != nil {
		data, err := json.Marshal(child.BreakRule)
		if err != nil {
			return fmt.Errorf("failed to marshal break rule: %w", err)
		}
		breakRuleJSON = sql.NullString{String: string(data), Valid: true}
	}

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO children (id, name, weekday_limit, weekend_limit, break_rule, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, child.ID, child.Name, child.WeekdayLimit, child.WeekendLimit, breakRuleJSON, child.CreatedAt, child.UpdatedAt)

	return err
}

// GetChild retrieves a child by ID
func (s *SQLiteStorage) GetChild(ctx context.Context, id string) (*core.Child, error) {
	var child core.Child
	var breakRuleJSON sql.NullString

	err := s.db.QueryRowContext(ctx, `
		SELECT id, name, weekday_limit, weekend_limit, break_rule, created_at, updated_at
		FROM children WHERE id = ?
	`, id).Scan(&child.ID, &child.Name, &child.WeekdayLimit, &child.WeekendLimit,
		&breakRuleJSON, &child.CreatedAt, &child.UpdatedAt)

	if err == sql.ErrNoRows {
		return nil, core.ErrChildNotFound
	}
	if err != nil {
		return nil, err
	}

	if breakRuleJSON.Valid {
		var breakRule core.BreakRule
		if err := json.Unmarshal([]byte(breakRuleJSON.String), &breakRule); err != nil {
			return nil, fmt.Errorf("failed to unmarshal break rule: %w", err)
		}
		child.BreakRule = &breakRule
	}

	return &child, nil
}

// ListChildren retrieves all children
func (s *SQLiteStorage) ListChildren(ctx context.Context) ([]*core.Child, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, name, weekday_limit, weekend_limit, break_rule, created_at, updated_at
		FROM children ORDER BY name
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var children []*core.Child
	for rows.Next() {
		var child core.Child
		var breakRuleJSON sql.NullString

		if err := rows.Scan(&child.ID, &child.Name, &child.WeekdayLimit, &child.WeekendLimit,
			&breakRuleJSON, &child.CreatedAt, &child.UpdatedAt); err != nil {
			return nil, err
		}

		if breakRuleJSON.Valid {
			var breakRule core.BreakRule
			if err := json.Unmarshal([]byte(breakRuleJSON.String), &breakRule); err != nil {
				return nil, fmt.Errorf("failed to unmarshal break rule: %w", err)
			}
			child.BreakRule = &breakRule
		}

		children = append(children, &child)
	}

	return children, rows.Err()
}

// UpdateChild updates an existing child
func (s *SQLiteStorage) UpdateChild(ctx context.Context, child *core.Child) error {
	if err := child.Validate(); err != nil {
		return err
	}

	child.UpdatedAt = time.Now()

	var breakRuleJSON sql.NullString
	if child.BreakRule != nil {
		data, err := json.Marshal(child.BreakRule)
		if err != nil {
			return fmt.Errorf("failed to marshal break rule: %w", err)
		}
		breakRuleJSON = sql.NullString{String: string(data), Valid: true}
	}

	result, err := s.db.ExecContext(ctx, `
		UPDATE children
		SET name = ?, weekday_limit = ?, weekend_limit = ?, break_rule = ?, updated_at = ?
		WHERE id = ?
	`, child.Name, child.WeekdayLimit, child.WeekendLimit, breakRuleJSON, child.UpdatedAt, child.ID)

	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return core.ErrChildNotFound
	}

	return nil
}

// DeleteChild deletes a child
func (s *SQLiteStorage) DeleteChild(ctx context.Context, id string) error {
	result, err := s.db.ExecContext(ctx, "DELETE FROM children WHERE id = ?", id)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return core.ErrChildNotFound
	}

	return nil
}

// CreateSession creates a new session
func (s *SQLiteStorage) CreateSession(ctx context.Context, session *core.Session) error {
	if err := session.Validate(); err != nil {
		return err
	}

	now := time.Now()
	session.CreatedAt = now
	session.UpdatedAt = now

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var lastBreakAt, breakEndsAt sql.NullTime
	if session.LastBreakAt != nil {
		lastBreakAt = sql.NullTime{Time: *session.LastBreakAt, Valid: true}
	}
	if session.BreakEndsAt != nil {
		breakEndsAt = sql.NullTime{Time: *session.BreakEndsAt, Valid: true}
	}

	_, err = tx.ExecContext(ctx, `
		INSERT INTO sessions (id, device_type, device_id, start_time, expected_duration,
			remaining_minutes, status, last_break_at, break_ends_at, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, session.ID, session.DeviceType, session.DeviceID, session.StartTime, session.ExpectedDuration,
		session.RemainingMinutes, session.Status, lastBreakAt, breakEndsAt, session.CreatedAt, session.UpdatedAt)

	if err != nil {
		return err
	}

	// Insert session-child associations
	for _, childID := range session.ChildIDs {
		_, err = tx.ExecContext(ctx, `
			INSERT INTO session_children (session_id, child_id) VALUES (?, ?)
		`, session.ID, childID)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

// GetSession retrieves a session by ID
func (s *SQLiteStorage) GetSession(ctx context.Context, id string) (*core.Session, error) {
	var session core.Session
	var lastBreakAt, breakEndsAt sql.NullTime

	err := s.db.QueryRowContext(ctx, `
		SELECT id, device_type, device_id, start_time, expected_duration,
			remaining_minutes, status, last_break_at, break_ends_at, created_at, updated_at
		FROM sessions WHERE id = ?
	`, id).Scan(&session.ID, &session.DeviceType, &session.DeviceID, &session.StartTime,
		&session.ExpectedDuration, &session.RemainingMinutes, &session.Status,
		&lastBreakAt, &breakEndsAt, &session.CreatedAt, &session.UpdatedAt)

	if err == sql.ErrNoRows {
		return nil, core.ErrSessionNotFound
	}
	if err != nil {
		return nil, err
	}

	if lastBreakAt.Valid {
		session.LastBreakAt = &lastBreakAt.Time
	}
	if breakEndsAt.Valid {
		session.BreakEndsAt = &breakEndsAt.Time
	}

	// Load child IDs
	rows, err := s.db.QueryContext(ctx, `
		SELECT child_id FROM session_children WHERE session_id = ?
	`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var childID string
		if err := rows.Scan(&childID); err != nil {
			return nil, err
		}
		session.ChildIDs = append(session.ChildIDs, childID)
	}

	return &session, rows.Err()
}

// ListActiveSessions retrieves all active sessions
func (s *SQLiteStorage) ListActiveSessions(ctx context.Context) ([]*core.Session, error) {
	return s.listSessionsByCondition(ctx, "status = ?", core.SessionStatusActive)
}

// ListSessionsByChild retrieves all sessions for a specific child
func (s *SQLiteStorage) ListSessionsByChild(ctx context.Context, childID string) ([]*core.Session, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT s.id, s.device_type, s.device_id, s.start_time, s.expected_duration,
			s.remaining_minutes, s.status, s.last_break_at, s.break_ends_at, s.created_at, s.updated_at
		FROM sessions s
		JOIN session_children sc ON s.id = sc.session_id
		WHERE sc.child_id = ?
		ORDER BY s.start_time DESC
	`, childID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return s.scanSessions(ctx, rows)
}

// UpdateSession updates an existing session
func (s *SQLiteStorage) UpdateSession(ctx context.Context, session *core.Session) error {
	session.UpdatedAt = time.Now()

	var lastBreakAt, breakEndsAt sql.NullTime
	if session.LastBreakAt != nil {
		lastBreakAt = sql.NullTime{Time: *session.LastBreakAt, Valid: true}
	}
	if session.BreakEndsAt != nil {
		breakEndsAt = sql.NullTime{Time: *session.BreakEndsAt, Valid: true}
	}

	result, err := s.db.ExecContext(ctx, `
		UPDATE sessions
		SET device_type = ?, device_id = ?, remaining_minutes = ?, status = ?,
			last_break_at = ?, break_ends_at = ?, updated_at = ?
		WHERE id = ?
	`, session.DeviceType, session.DeviceID, session.RemainingMinutes, session.Status,
		lastBreakAt, breakEndsAt, session.UpdatedAt, session.ID)

	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return core.ErrSessionNotFound
	}

	return nil
}

// DeleteSession deletes a session
func (s *SQLiteStorage) DeleteSession(ctx context.Context, id string) error {
	result, err := s.db.ExecContext(ctx, "DELETE FROM sessions WHERE id = ?", id)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return core.ErrSessionNotFound
	}

	return nil
}

// GetDailyUsage retrieves daily usage for a child on a specific date
func (s *SQLiteStorage) GetDailyUsage(ctx context.Context, childID string, date time.Time) (*core.DailyUsage, error) {
	normalizedDate := normalizeDate(date)

	var usage core.DailyUsage
	err := s.db.QueryRowContext(ctx, `
		SELECT child_id, date, minutes_used, session_count, created_at, updated_at
		FROM daily_usage WHERE child_id = ? AND date = ?
	`, childID, normalizedDate).Scan(&usage.ChildID, &usage.Date, &usage.MinutesUsed,
		&usage.SessionCount, &usage.CreatedAt, &usage.UpdatedAt)

	if err == sql.ErrNoRows {
		// Return zero usage if not found
		return &core.DailyUsage{
			ChildID:      childID,
			Date:         normalizedDate,
			MinutesUsed:  0,
			SessionCount: 0,
			CreatedAt:    time.Now(),
			UpdatedAt:    time.Now(),
		}, nil
	}
	if err != nil {
		return nil, err
	}

	return &usage, nil
}

// UpdateDailyUsage updates daily usage
func (s *SQLiteStorage) UpdateDailyUsage(ctx context.Context, usage *core.DailyUsage) error {
	usage.Date = normalizeDate(usage.Date)
	usage.UpdatedAt = time.Now()

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO daily_usage (child_id, date, minutes_used, session_count, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(child_id, date) DO UPDATE SET
			minutes_used = excluded.minutes_used,
			session_count = excluded.session_count,
			updated_at = excluded.updated_at
	`, usage.ChildID, usage.Date, usage.MinutesUsed, usage.SessionCount, usage.CreatedAt, usage.UpdatedAt)

	return err
}

// IncrementDailyUsage increments the daily usage for a child
func (s *SQLiteStorage) IncrementDailyUsage(ctx context.Context, childID string, date time.Time, minutes int) error {
	normalizedDate := normalizeDate(date)
	now := time.Now()

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO daily_usage (child_id, date, minutes_used, session_count, created_at, updated_at)
		VALUES (?, ?, ?, 0, ?, ?)
		ON CONFLICT(child_id, date) DO UPDATE SET
			minutes_used = minutes_used + ?,
			updated_at = ?
	`, childID, normalizedDate, minutes, now, now, minutes, now)

	return err
}

// Close closes the database connection
func (s *SQLiteStorage) Close() error {
	return s.db.Close()
}

// Helper functions

func (s *SQLiteStorage) listSessionsByCondition(ctx context.Context, condition string, args ...interface{}) ([]*core.Session, error) {
	query := `
		SELECT id, device_type, device_id, start_time, expected_duration,
			remaining_minutes, status, last_break_at, break_ends_at, created_at, updated_at
		FROM sessions WHERE ` + condition + ` ORDER BY start_time DESC
	`

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return s.scanSessions(ctx, rows)
}

func (s *SQLiteStorage) scanSessions(ctx context.Context, rows *sql.Rows) ([]*core.Session, error) {
	var sessions []*core.Session

	for rows.Next() {
		var session core.Session
		var lastBreakAt, breakEndsAt sql.NullTime

		if err := rows.Scan(&session.ID, &session.DeviceType, &session.DeviceID, &session.StartTime,
			&session.ExpectedDuration, &session.RemainingMinutes, &session.Status,
			&lastBreakAt, &breakEndsAt, &session.CreatedAt, &session.UpdatedAt); err != nil {
			return nil, err
		}

		if lastBreakAt.Valid {
			session.LastBreakAt = &lastBreakAt.Time
		}
		if breakEndsAt.Valid {
			session.BreakEndsAt = &breakEndsAt.Time
		}

		// Load child IDs
		childRows, err := s.db.QueryContext(ctx, `
			SELECT child_id FROM session_children WHERE session_id = ?
		`, session.ID)
		if err != nil {
			return nil, err
		}

		for childRows.Next() {
			var childID string
			if err := childRows.Scan(&childID); err != nil {
				childRows.Close()
				return nil, err
			}
			session.ChildIDs = append(session.ChildIDs, childID)
		}
		childRows.Close()

		sessions = append(sessions, &session)
	}

	return sessions, rows.Err()
}

func normalizeDate(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}
