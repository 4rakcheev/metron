package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"metron/internal/core"
	"metron/internal/drivers/aqara"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// SQLiteStorage implements storage.Storage using SQLite
type SQLiteStorage struct {
	db       *sql.DB
	timezone *time.Location
}

// New creates a new SQLite storage instance
func New(dbPath string, timezone *time.Location) (*SQLiteStorage, error) {
	if timezone == nil {
		timezone = time.UTC // Fallback to UTC
	}

	// SQLite will store times as UTC strings, we'll convert in app layer
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Enable foreign keys
	if _, err := db.Exec("PRAGMA foreign_keys = ON"); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to enable foreign keys: %w", err)
	}

	storage := &SQLiteStorage{
		db:       db,
		timezone: timezone,
	}

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
			pin TEXT NOT NULL DEFAULT '',
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
			status TEXT NOT NULL,
			last_break_at DATETIME,
			break_ends_at DATETIME,
			warning_sent_at DATETIME,
			last_extended_at DATETIME,
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

		CREATE TABLE IF NOT EXISTS aqara_tokens (
			id INTEGER PRIMARY KEY CHECK (id = 1),
			refresh_token TEXT NOT NULL,
			access_token TEXT,
			access_token_expires_at DATETIME,
			created_at DATETIME NOT NULL,
			updated_at DATETIME NOT NULL
		);
	`

	if _, err := s.db.Exec(schema); err != nil {
		return err
	}

	// Run migrations for schema changes
	return s.runMigrations()
}

// runMigrations applies incremental schema changes
func (s *SQLiteStorage) runMigrations() error {
	// Add warning_sent_at column if it doesn't exist (for existing databases)
	_, err := s.db.Exec(`
		ALTER TABLE sessions ADD COLUMN warning_sent_at DATETIME;
	`)
	// Ignore error if column already exists
	if err != nil && err.Error() != "duplicate column name: warning_sent_at" {
		// Check if it's a "duplicate column" error (SQLite specific)
		// If not, it might be a real error
		// For now, we'll ignore all errors as the column might already exist
	}

	// Add last_extended_at column if it doesn't exist (for rate limiting exploit fix)
	_, err = s.db.Exec(`
		ALTER TABLE sessions ADD COLUMN last_extended_at DATETIME;
	`)
	// Ignore error if column already exists
	if err != nil && err.Error() != "duplicate column name: last_extended_at" {
		// Column might already exist, which is fine
	}

	// Add pin column to children table if it doesn't exist (for existing databases)
	_, err = s.db.Exec(`
		ALTER TABLE children ADD COLUMN pin TEXT NOT NULL DEFAULT '';
	`)
	// Ignore error if column already exists
	if err != nil && err.Error() != "duplicate column name: pin" {
		// Column might already exist, which is fine
	}

	// Add reward_minutes_granted column to daily_usage table if it doesn't exist
	_, err = s.db.Exec(`
		ALTER TABLE daily_usage ADD COLUMN reward_minutes_granted INTEGER NOT NULL DEFAULT 0;
	`)
	// Ignore error if column already exists
	if err != nil && err.Error() != "duplicate column name: reward_minutes_granted" {
		// Column might already exist, which is fine
	}

	// Check if sessions table has remaining_minutes column
	var hasRemainingMinutes bool
	row := s.db.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('sessions') WHERE name='remaining_minutes'`)
	row.Scan(&hasRemainingMinutes)

	// Remove remaining_minutes column if it exists (we calculate it dynamically)
	if hasRemainingMinutes {
		// SQLite doesn't support DROP COLUMN in older versions, so we recreate the table
		_, err = s.db.Exec(`
			PRAGMA foreign_keys=off;

			-- Create new sessions table without remaining_minutes
			CREATE TABLE sessions_new (
				id TEXT PRIMARY KEY,
				device_type TEXT NOT NULL,
				device_id TEXT NOT NULL,
				start_time DATETIME NOT NULL,
				expected_duration INTEGER NOT NULL,
				status TEXT NOT NULL,
				last_break_at DATETIME,
				break_ends_at DATETIME,
				warning_sent_at DATETIME,
				created_at DATETIME NOT NULL,
				updated_at DATETIME NOT NULL
			);

			-- Copy all session data
			INSERT INTO sessions_new (id, device_type, device_id, start_time, expected_duration,
				status, last_break_at, break_ends_at, warning_sent_at, created_at, updated_at)
			SELECT id, device_type, device_id, start_time, expected_duration,
				status, last_break_at, break_ends_at, warning_sent_at, created_at, updated_at
			FROM sessions;

			-- Drop old table
			DROP TABLE sessions;

			-- Rename new table
			ALTER TABLE sessions_new RENAME TO sessions;

			-- Recreate session_children table to restore foreign keys
			CREATE TABLE session_children_new (
				session_id TEXT NOT NULL,
				child_id TEXT NOT NULL,
				PRIMARY KEY (session_id, child_id),
				FOREIGN KEY (session_id) REFERENCES sessions(id) ON DELETE CASCADE,
				FOREIGN KEY (child_id) REFERENCES children(id) ON DELETE CASCADE
			);

			-- Copy session_children data
			INSERT INTO session_children_new (session_id, child_id)
			SELECT session_id, child_id FROM session_children;

			-- Drop old session_children
			DROP TABLE session_children;

			-- Rename
			ALTER TABLE session_children_new RENAME TO session_children;

			PRAGMA foreign_keys=on;
		`)
		// Log error but don't fail startup if migration has issues
		if err != nil {
			// Migration failed, but app can continue if schema is already correct
			return nil
		}
	}

	// Migration: Refactor to separate allocation and consumption concerns
	// Drop old daily_usage table and create new tables
	_, err = s.db.Exec(`
		-- Drop old daily_usage table (replaced by new architecture)
		DROP TABLE IF EXISTS daily_usage;

		-- Create daily_time_allocations table
		CREATE TABLE IF NOT EXISTS daily_time_allocations (
			child_id TEXT NOT NULL,
			date DATE NOT NULL,
			base_limit INTEGER NOT NULL,
			bonus_granted INTEGER NOT NULL DEFAULT 0,
			created_at DATETIME NOT NULL,
			updated_at DATETIME NOT NULL,
			PRIMARY KEY (child_id, date),
			FOREIGN KEY (child_id) REFERENCES children(id) ON DELETE CASCADE
		);

		-- Create daily_usage_summaries table
		CREATE TABLE IF NOT EXISTS daily_usage_summaries (
			child_id TEXT NOT NULL,
			date DATE NOT NULL,
			minutes_used INTEGER NOT NULL DEFAULT 0,
			session_count INTEGER NOT NULL DEFAULT 0,
			created_at DATETIME NOT NULL,
			updated_at DATETIME NOT NULL,
			PRIMARY KEY (child_id, date),
			FOREIGN KEY (child_id) REFERENCES children(id) ON DELETE CASCADE
		);

		-- Create indexes for new tables
		CREATE INDEX IF NOT EXISTS idx_daily_allocations_date ON daily_time_allocations(date);
		CREATE INDEX IF NOT EXISTS idx_daily_usage_summaries_date ON daily_usage_summaries(date);

		-- Recreate old daily_usage table for backwards compatibility with tests
		-- This table is deprecated - new code uses daily_time_allocations and daily_usage_summaries
		CREATE TABLE IF NOT EXISTS daily_usage (
			child_id TEXT NOT NULL,
			date DATE NOT NULL,
			minutes_used INTEGER NOT NULL DEFAULT 0,
			reward_minutes_granted INTEGER NOT NULL DEFAULT 0,
			session_count INTEGER NOT NULL DEFAULT 0,
			created_at DATETIME NOT NULL,
			updated_at DATETIME NOT NULL,
			PRIMARY KEY (child_id, date),
			FOREIGN KEY (child_id) REFERENCES children(id) ON DELETE CASCADE
		);
		CREATE INDEX IF NOT EXISTS idx_daily_usage_date ON daily_usage(date);
	`)
	if err != nil {
		// Ignore if tables already exist
	}

	// Add actual_duration column to sessions table
	_, err = s.db.Exec(`
		ALTER TABLE sessions ADD COLUMN actual_duration INTEGER;
	`)
	// Ignore error if column already exists
	if err != nil && err.Error() != "duplicate column name: actual_duration" {
		// Column might already exist, which is fine
	}

	// Add downtime_enabled column to children table
	_, err = s.db.Exec(`
		ALTER TABLE children ADD COLUMN downtime_enabled BOOLEAN NOT NULL DEFAULT 0;
	`)
	// Ignore error if column already exists
	if err != nil && err.Error() != "duplicate column name: downtime_enabled" {
		// Column might already exist, which is fine
	}

	// Add emoji column to children table
	_, err = s.db.Exec(`
		ALTER TABLE children ADD COLUMN emoji TEXT NOT NULL DEFAULT '👶';
	`)
	// Ignore error if column already exists
	if err != nil && err.Error() != "duplicate column name: emoji" {
		// Column might already exist, which is fine
	}

	// Migrate data from daily_usage to daily_usage_summaries
	// This is a one-time migration to handle the transition from old to new table
	_, err = s.db.Exec(`
		INSERT OR REPLACE INTO daily_usage_summaries (child_id, date, minutes_used, session_count, created_at, updated_at)
		SELECT child_id, date, minutes_used, session_count, created_at, updated_at
		FROM daily_usage
		WHERE NOT EXISTS (
			SELECT 1 FROM daily_usage_summaries
			WHERE daily_usage_summaries.child_id = daily_usage.child_id
			AND daily_usage_summaries.date = daily_usage.date
		);
	`)
	// Ignore errors - this migration is best-effort
	if err != nil {
		// Migration failed, but we can continue - new sessions will write to summaries
	}

	// Create downtime_skip table for storing skip downtime dates
	_, err = s.db.Exec(`
		CREATE TABLE IF NOT EXISTS downtime_skip (
			id INTEGER PRIMARY KEY CHECK (id = 1),
			skip_date DATE NOT NULL,
			created_at DATETIME NOT NULL
		);
	`)
	if err != nil {
		return fmt.Errorf("failed to create downtime_skip table: %w", err)
	}

	// Create device_bypass table for agent-controlled devices
	_, err = s.db.Exec(`
		CREATE TABLE IF NOT EXISTS device_bypass (
			device_id TEXT PRIMARY KEY,
			enabled BOOLEAN NOT NULL DEFAULT 0,
			reason TEXT,
			enabled_at DATETIME NOT NULL,
			enabled_by TEXT,
			expires_at DATETIME
		);
	`)
	if err != nil {
		return fmt.Errorf("failed to create device_bypass table: %w", err)
	}

	// Create movie_time_usage table for weekend shared movie time
	_, err = s.db.Exec(`
		CREATE TABLE IF NOT EXISTS movie_time_usage (
			date DATE PRIMARY KEY,
			session_id TEXT,
			started_at DATETIME,
			started_by TEXT,
			status TEXT NOT NULL DEFAULT 'available',
			created_at DATETIME NOT NULL,
			updated_at DATETIME NOT NULL
		);
	`)
	if err != nil {
		return fmt.Errorf("failed to create movie_time_usage table: %w", err)
	}

	// Add is_movie_session column to sessions table
	_, err = s.db.Exec(`
		ALTER TABLE sessions ADD COLUMN is_movie_session INTEGER NOT NULL DEFAULT 0;
	`)
	// Ignore error if column already exists
	if err != nil && err.Error() != "duplicate column name: is_movie_session" {
		// Column might already exist, which is fine
	}

	// Create movie_time_bypass table for holiday/vacation bypass periods
	_, err = s.db.Exec(`
		CREATE TABLE IF NOT EXISTS movie_time_bypass (
			id TEXT PRIMARY KEY,
			reason TEXT NOT NULL,
			start_date DATE NOT NULL,
			end_date DATE NOT NULL,
			created_at DATETIME NOT NULL,
			updated_at DATETIME NOT NULL
		);
	`)
	if err != nil {
		return fmt.Errorf("failed to create movie_time_bypass table: %w", err)
	}

	return nil
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
		INSERT INTO children (id, name, emoji, pin, weekday_limit, weekend_limit, break_rule, downtime_enabled, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, child.ID, child.Name, child.Emoji, child.PIN, child.WeekdayLimit, child.WeekendLimit, breakRuleJSON, child.DowntimeEnabled, child.CreatedAt, child.UpdatedAt)

	return err
}

// GetChild retrieves a child by ID
func (s *SQLiteStorage) GetChild(ctx context.Context, id string) (*core.Child, error) {
	var child core.Child
	var breakRuleJSON sql.NullString

	err := s.db.QueryRowContext(ctx, `
		SELECT id, name, emoji, pin, weekday_limit, weekend_limit, break_rule, downtime_enabled, created_at, updated_at
		FROM children WHERE id = ?
	`, id).Scan(&child.ID, &child.Name, &child.Emoji, &child.PIN, &child.WeekdayLimit, &child.WeekendLimit,
		&breakRuleJSON, &child.DowntimeEnabled, &child.CreatedAt, &child.UpdatedAt)

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
		SELECT id, name, emoji, pin, weekday_limit, weekend_limit, break_rule, downtime_enabled, created_at, updated_at
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

		if err := rows.Scan(&child.ID, &child.Name, &child.Emoji, &child.PIN, &child.WeekdayLimit, &child.WeekendLimit,
			&breakRuleJSON, &child.DowntimeEnabled, &child.CreatedAt, &child.UpdatedAt); err != nil {
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
		SET name = ?, emoji = ?, pin = ?, weekday_limit = ?, weekend_limit = ?, break_rule = ?, downtime_enabled = ?, updated_at = ?
		WHERE id = ?
	`, child.Name, child.Emoji, child.PIN, child.WeekdayLimit, child.WeekendLimit, breakRuleJSON, child.DowntimeEnabled, child.UpdatedAt, child.ID)

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

	var lastBreakAt, breakEndsAt, warningSentAt, lastExtendedAt sql.NullTime
	if session.LastBreakAt != nil {
		lastBreakAt = sql.NullTime{Time: *session.LastBreakAt, Valid: true}
	}
	if session.BreakEndsAt != nil {
		breakEndsAt = sql.NullTime{Time: *session.BreakEndsAt, Valid: true}
	}
	if session.WarningSentAt != nil {
		warningSentAt = sql.NullTime{Time: *session.WarningSentAt, Valid: true}
	}
	if session.LastExtendedAt != nil {
		lastExtendedAt = sql.NullTime{Time: *session.LastExtendedAt, Valid: true}
	}

	_, err = tx.ExecContext(ctx, `
		INSERT INTO sessions (id, device_type, device_id, start_time, expected_duration,
			status, last_break_at, break_ends_at, warning_sent_at, last_extended_at, is_movie_session, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, session.ID, session.DeviceType, session.DeviceID, session.StartTime, session.ExpectedDuration,
		session.Status, lastBreakAt, breakEndsAt, warningSentAt, lastExtendedAt, session.IsMovieSession, session.CreatedAt, session.UpdatedAt)

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
	var lastBreakAt, breakEndsAt, warningSentAt, lastExtendedAt sql.NullTime

	err := s.db.QueryRowContext(ctx, `
		SELECT id, device_type, device_id, start_time, expected_duration,
			status, last_break_at, break_ends_at, warning_sent_at, last_extended_at, is_movie_session, created_at, updated_at
		FROM sessions WHERE id = ?
	`, id).Scan(&session.ID, &session.DeviceType, &session.DeviceID, &session.StartTime,
		&session.ExpectedDuration, &session.Status,
		&lastBreakAt, &breakEndsAt, &warningSentAt, &lastExtendedAt, &session.IsMovieSession, &session.CreatedAt, &session.UpdatedAt)

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
	if warningSentAt.Valid {
		session.WarningSentAt = &warningSentAt.Time
	}
	if lastExtendedAt.Valid {
		session.LastExtendedAt = &lastExtendedAt.Time
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

// ListAllSessions retrieves all sessions regardless of status
func (s *SQLiteStorage) ListAllSessions(ctx context.Context) ([]*core.Session, error) {
	return s.listSessionsByCondition(ctx, "1=1")
}

// ListSessionsByChild retrieves all sessions for a specific child
func (s *SQLiteStorage) ListSessionsByChild(ctx context.Context, childID string) ([]*core.Session, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT s.id, s.device_type, s.device_id, s.start_time, s.expected_duration,
			s.status, s.last_break_at, s.break_ends_at, s.warning_sent_at, s.last_extended_at, s.is_movie_session, s.created_at, s.updated_at
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

	var lastBreakAt, breakEndsAt, warningSentAt, lastExtendedAt sql.NullTime
	if session.LastBreakAt != nil {
		lastBreakAt = sql.NullTime{Time: *session.LastBreakAt, Valid: true}
	}
	if session.BreakEndsAt != nil {
		breakEndsAt = sql.NullTime{Time: *session.BreakEndsAt, Valid: true}
	}
	if session.WarningSentAt != nil {
		warningSentAt = sql.NullTime{Time: *session.WarningSentAt, Valid: true}
	}
	if session.LastExtendedAt != nil {
		lastExtendedAt = sql.NullTime{Time: *session.LastExtendedAt, Valid: true}
	}

	result, err := s.db.ExecContext(ctx, `
		UPDATE sessions
		SET device_type = ?, device_id = ?, expected_duration = ?, status = ?,
			last_break_at = ?, break_ends_at = ?, warning_sent_at = ?, last_extended_at = ?, updated_at = ?
		WHERE id = ?
	`, session.DeviceType, session.DeviceID, session.ExpectedDuration, session.Status,
		lastBreakAt, breakEndsAt, warningSentAt, lastExtendedAt, session.UpdatedAt, session.ID)

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
// DEPRECATED: This method is kept for backward compatibility with tests only.
// Production code should use GetDailyUsageSummary instead.
// The daily_usage table has been dropped and replaced with daily_usage_summaries.
func (s *SQLiteStorage) GetDailyUsage(ctx context.Context, childID string, date time.Time) (*core.DailyUsage, error) {
	// DEPRECATED: Delegates to new table methods for backward compatibility
	normalizedDate := s.normalizeDate(date)

	// Get usage summary from new table
	summary, err := s.GetDailyUsageSummary(ctx, childID, normalizedDate)
	if err != nil {
		return nil, err
	}

	// Get allocation to find reward minutes
	allocation, err := s.GetDailyAllocation(ctx, childID, normalizedDate)
	rewardMinutes := 0
	if err == nil {
		rewardMinutes = allocation.BonusGranted
	}

	return &core.DailyUsage{
		ChildID:              summary.ChildID,
		Date:                 summary.Date,
		MinutesUsed:          summary.MinutesUsed,
		RewardMinutesGranted: rewardMinutes,
		SessionCount:         summary.SessionCount,
		CreatedAt:            summary.CreatedAt,
		UpdatedAt:            summary.UpdatedAt,
	}, nil
}

// UpdateDailyUsage updates daily usage
// DEPRECATED: Delegates to new table methods for backward compatibility
func (s *SQLiteStorage) UpdateDailyUsage(ctx context.Context, usage *core.DailyUsage) error {
	normalizedDate := s.normalizeDate(usage.Date)
	now := time.Now()

	// Update summary table
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO daily_usage_summaries (child_id, date, minutes_used, session_count, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(child_id, date) DO UPDATE SET
			minutes_used = excluded.minutes_used,
			session_count = excluded.session_count,
			updated_at = excluded.updated_at
	`, usage.ChildID, normalizedDate, usage.MinutesUsed, usage.SessionCount, now, now)

	if err != nil {
		return err
	}

	// Update allocation table if there are reward minutes
	if usage.RewardMinutesGranted > 0 {
		child, err := s.GetChild(ctx, usage.ChildID)
		if err != nil {
			return err
		}

		baseLimit := child.GetDailyLimit(normalizedDate)

		_, err = s.db.ExecContext(ctx, `
			INSERT INTO daily_time_allocations (child_id, date, base_limit, bonus_granted, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?)
			ON CONFLICT(child_id, date) DO UPDATE SET
				bonus_granted = excluded.bonus_granted,
				updated_at = excluded.updated_at
		`, usage.ChildID, normalizedDate, baseLimit, usage.RewardMinutesGranted, now, now)

		if err != nil {
			return err
		}
	}

	return nil
}

// IncrementDailyUsage increments the daily usage for a child
// DEPRECATED: Delegates to IncrementDailyUsageSummary for backward compatibility
func (s *SQLiteStorage) IncrementDailyUsage(ctx context.Context, childID string, date time.Time, minutes int) error {
	return s.IncrementDailyUsageSummary(ctx, childID, date, minutes)
}

// IncrementSessionCount increments the session count for a child on a given date
// DEPRECATED: Delegates to IncrementSessionCountSummary for backward compatibility
func (s *SQLiteStorage) IncrementSessionCount(ctx context.Context, childID string, date time.Time) error {
	return s.IncrementSessionCountSummary(ctx, childID, date)
}

// GrantRewardMinutes grants reward minutes to a child for a specific day
// DEPRECATED: Not used in production. GrantRewardMinutes in manager.go uses allocation system.
func (s *SQLiteStorage) GrantRewardMinutes(ctx context.Context, childID string, date time.Time, minutes int) error {
	// This method is no longer used - manager uses UpdateDailyAllocation
	return nil
}

// ============================================================================
// NEW STORAGE METHODS - Refactored Architecture
// ============================================================================

// GetDailyAllocation retrieves the daily time allocation for a child
func (s *SQLiteStorage) GetDailyAllocation(ctx context.Context, childID string, date time.Time) (*core.DailyTimeAllocation, error) {
	normalizedDate := s.normalizeDate(date)

	var allocation core.DailyTimeAllocation
	err := s.db.QueryRowContext(ctx, `
		SELECT child_id, date, base_limit, bonus_granted, created_at, updated_at
		FROM daily_time_allocations WHERE child_id = ? AND date = ?
	`, childID, normalizedDate).Scan(&allocation.ChildID, &allocation.Date, &allocation.BaseLimit,
		&allocation.BonusGranted, &allocation.CreatedAt, &allocation.UpdatedAt)

	if err == sql.ErrNoRows {
		return nil, core.ErrAllocationNotFound
	}
	if err != nil {
		return nil, err
	}

	return &allocation, nil
}

// CreateDailyAllocation creates a new daily time allocation
func (s *SQLiteStorage) CreateDailyAllocation(ctx context.Context, allocation *core.DailyTimeAllocation) error {
	allocation.Date = s.normalizeDate(allocation.Date)
	allocation.CreatedAt = time.Now()
	allocation.UpdatedAt = time.Now()

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO daily_time_allocations (child_id, date, base_limit, bonus_granted, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, allocation.ChildID, allocation.Date, allocation.BaseLimit, allocation.BonusGranted, allocation.CreatedAt, allocation.UpdatedAt)

	return err
}

// UpdateDailyAllocation updates an existing daily time allocation
func (s *SQLiteStorage) UpdateDailyAllocation(ctx context.Context, allocation *core.DailyTimeAllocation) error {
	allocation.Date = s.normalizeDate(allocation.Date)
	allocation.UpdatedAt = time.Now()

	_, err := s.db.ExecContext(ctx, `
		UPDATE daily_time_allocations
		SET base_limit = ?, bonus_granted = ?, updated_at = ?
		WHERE child_id = ? AND date = ?
	`, allocation.BaseLimit, allocation.BonusGranted, allocation.UpdatedAt, allocation.ChildID, allocation.Date)

	return err
}

// GrantRewardMinutesNew grants reward minutes to a child's daily allocation
// This updates the daily_time_allocations table
func (s *SQLiteStorage) GrantRewardMinutesNew(ctx context.Context, childID string, date time.Time, minutes int) error {
	normalizedDate := s.normalizeDate(date)
	now := time.Now()

	// Try to update existing allocation first
	result, err := s.db.ExecContext(ctx, `
		UPDATE daily_time_allocations
		SET bonus_granted = bonus_granted + ?, updated_at = ?
		WHERE child_id = ? AND date = ?
	`, minutes, now, childID, normalizedDate)

	if err != nil {
		return err
	}

	// Check if update affected any rows
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	// If no rows were updated, allocation doesn't exist yet - will be created lazily by calculator
	// This is okay - the calculator's getOrCreateAllocation will handle it
	if rowsAffected == 0 {
		// Create the allocation now
		child, err := s.GetChild(ctx, childID)
		if err != nil {
			return err
		}

		baseLimit := child.GetDailyLimit(normalizedDate)

		_, err = s.db.ExecContext(ctx, `
			INSERT INTO daily_time_allocations (child_id, date, base_limit, bonus_granted, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?)
		`, childID, normalizedDate, baseLimit, minutes, now, now)

		return err
	}

	return nil
}

// GetDailyUsageSummary retrieves the daily usage summary for a child
func (s *SQLiteStorage) GetDailyUsageSummary(ctx context.Context, childID string, date time.Time) (*core.DailyUsageSummary, error) {
	normalizedDate := s.normalizeDate(date)

	var summary core.DailyUsageSummary
	err := s.db.QueryRowContext(ctx, `
		SELECT child_id, date, minutes_used, session_count, created_at, updated_at
		FROM daily_usage_summaries WHERE child_id = ? AND date = ?
	`, childID, normalizedDate).Scan(&summary.ChildID, &summary.Date, &summary.MinutesUsed,
		&summary.SessionCount, &summary.CreatedAt, &summary.UpdatedAt)

	if err == sql.ErrNoRows {
		// Return zero summary if not found
		return &core.DailyUsageSummary{
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

	return &summary, nil
}

// IncrementDailyUsageSummary increments the daily usage summary
func (s *SQLiteStorage) IncrementDailyUsageSummary(ctx context.Context, childID string, date time.Time, minutes int) error {
	normalizedDate := s.normalizeDate(date)
	now := time.Now()

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO daily_usage_summaries (child_id, date, minutes_used, session_count, created_at, updated_at)
		VALUES (?, ?, ?, 0, ?, ?)
		ON CONFLICT(child_id, date) DO UPDATE SET
			minutes_used = minutes_used + ?,
			updated_at = ?
	`, childID, normalizedDate, minutes, now, now, minutes, now)

	return err
}

// IncrementSessionCountSummary increments the session count in daily usage summary
func (s *SQLiteStorage) IncrementSessionCountSummary(ctx context.Context, childID string, date time.Time) error {
	normalizedDate := s.normalizeDate(date)
	now := time.Now()

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO daily_usage_summaries (child_id, date, minutes_used, session_count, created_at, updated_at)
		VALUES (?, ?, 0, 1, ?, ?)
		ON CONFLICT(child_id, date) DO UPDATE SET
			session_count = session_count + 1,
			updated_at = ?
	`, childID, normalizedDate, now, now, now)

	return err
}

// ListActiveSessionRecords retrieves all active session usage records
func (s *SQLiteStorage) ListActiveSessionRecords(ctx context.Context) ([]*core.SessionUsageRecord, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, device_type, device_id, start_time, expected_duration, actual_duration, status,
			last_break_at, break_ends_at, warning_sent_at, is_movie_session, created_at, updated_at
		FROM sessions WHERE status = ?
	`, core.SessionStatusActive)

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sessions []*core.SessionUsageRecord
	for rows.Next() {
		var session core.SessionUsageRecord
		var actualDuration sql.NullInt64

		err := rows.Scan(&session.ID, &session.DeviceType, &session.DeviceID, &session.StartTime,
			&session.ExpectedDuration, &actualDuration, &session.Status, &session.LastBreakAt,
			&session.BreakEndsAt, &session.WarningSentAt, &session.IsMovieSession, &session.CreatedAt, &session.UpdatedAt)

		if err != nil {
			return nil, err
		}

		// Convert NULL to nil
		if actualDuration.Valid {
			duration := int(actualDuration.Int64)
			session.ActualDuration = &duration
		}

		// Get child IDs for this session
		childRows, err := s.db.QueryContext(ctx, `
			SELECT child_id FROM session_children WHERE session_id = ?
		`, session.ID)
		if err != nil {
			return nil, err
		}

		var childIDs []string
		for childRows.Next() {
			var childID string
			if err := childRows.Scan(&childID); err != nil {
				childRows.Close()
				return nil, err
			}
			childIDs = append(childIDs, childID)
		}
		childRows.Close()

		session.ChildIDs = childIDs
		sessions = append(sessions, &session)
	}

	return sessions, rows.Err()
}

// Close closes the database connection
func (s *SQLiteStorage) Close() error {
	return s.db.Close()
}

// Helper functions

func (s *SQLiteStorage) listSessionsByCondition(ctx context.Context, condition string, args ...interface{}) ([]*core.Session, error) {
	query := `
		SELECT id, device_type, device_id, start_time, expected_duration,
			status, last_break_at, break_ends_at, warning_sent_at, last_extended_at, is_movie_session, created_at, updated_at
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
		var lastBreakAt, breakEndsAt, warningSentAt, lastExtendedAt sql.NullTime

		if err := rows.Scan(&session.ID, &session.DeviceType, &session.DeviceID, &session.StartTime,
			&session.ExpectedDuration, &session.Status,
			&lastBreakAt, &breakEndsAt, &warningSentAt, &lastExtendedAt, &session.IsMovieSession, &session.CreatedAt, &session.UpdatedAt); err != nil {
			return nil, err
		}

		if lastBreakAt.Valid {
			session.LastBreakAt = &lastBreakAt.Time
		}
		if breakEndsAt.Valid {
			session.BreakEndsAt = &breakEndsAt.Time
		}
		if warningSentAt.Valid {
			session.WarningSentAt = &warningSentAt.Time
		}
		if lastExtendedAt.Valid {
			session.LastExtendedAt = &lastExtendedAt.Time
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

func (s *SQLiteStorage) normalizeDate(t time.Time) time.Time {
	// Convert to configured timezone and normalize to midnight
	// This ensures dates match the user's local calendar day
	inTZ := t.In(s.timezone)
	year, month, day := inTZ.Date()
	return time.Date(year, month, day, 0, 0, 0, 0, s.timezone)
}

// GetAqaraTokens retrieves the stored Aqara tokens
// Implements aqara.AqaraTokenStorage interface
func (s *SQLiteStorage) GetAqaraTokens(ctx context.Context) (*aqara.AqaraTokens, error) {
	var tokens aqara.AqaraTokens
	var expiresAt sql.NullTime

	err := s.db.QueryRowContext(ctx, `
		SELECT refresh_token, access_token, access_token_expires_at, created_at, updated_at
		FROM aqara_tokens WHERE id = 1
	`).Scan(&tokens.RefreshToken, &tokens.AccessToken, &expiresAt, &tokens.CreatedAt, &tokens.UpdatedAt)

	if err == sql.ErrNoRows {
		return nil, nil // No tokens stored yet
	}
	if err != nil {
		return nil, err
	}

	if expiresAt.Valid {
		tokens.AccessTokenExpiresAt = &expiresAt.Time
	}

	return &tokens, nil
}

// SaveAqaraTokens saves or updates the Aqara tokens
// Implements aqara.AqaraTokenStorage interface
func (s *SQLiteStorage) SaveAqaraTokens(ctx context.Context, tokens *aqara.AqaraTokens) error {
	now := time.Now()
	tokens.UpdatedAt = now

	var expiresAt sql.NullTime
	if tokens.AccessTokenExpiresAt != nil {
		expiresAt = sql.NullTime{Time: *tokens.AccessTokenExpiresAt, Valid: true}
	}

	// Check if tokens exist
	var exists bool
	err := s.db.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM aqara_tokens WHERE id = 1)").Scan(&exists)
	if err != nil {
		return err
	}

	if exists {
		// Update existing tokens
		_, err = s.db.ExecContext(ctx, `
			UPDATE aqara_tokens
			SET refresh_token = ?, access_token = ?, access_token_expires_at = ?, updated_at = ?
			WHERE id = 1
		`, tokens.RefreshToken, tokens.AccessToken, expiresAt, tokens.UpdatedAt)
	} else {
		// Insert new tokens
		tokens.CreatedAt = now
		_, err = s.db.ExecContext(ctx, `
			INSERT INTO aqara_tokens (id, refresh_token, access_token, access_token_expires_at, created_at, updated_at)
			VALUES (1, ?, ?, ?, ?, ?)
		`, tokens.RefreshToken, tokens.AccessToken, expiresAt, tokens.CreatedAt, tokens.UpdatedAt)
	}

	return err
}

// ============================================================================
// DOWNTIME SKIP STORAGE - Implements core.DowntimeSkipStorage interface
// ============================================================================

// GetDowntimeSkipDate retrieves the stored skip date for downtime
// Returns nil if no skip date is set
func (s *SQLiteStorage) GetDowntimeSkipDate(ctx context.Context) (*time.Time, error) {
	var skipDate time.Time

	err := s.db.QueryRowContext(ctx, `
		SELECT skip_date FROM downtime_skip WHERE id = 1
	`).Scan(&skipDate)

	if err == sql.ErrNoRows {
		return nil, nil // No skip date set
	}
	if err != nil {
		return nil, err
	}

	return &skipDate, nil
}

// SetDowntimeSkipDate sets the skip date for downtime
// Uses upsert pattern to insert or update the single-row table
func (s *SQLiteStorage) SetDowntimeSkipDate(ctx context.Context, date time.Time) error {
	normalizedDate := s.normalizeDate(date)
	now := time.Now()

	// Check if skip date exists
	var exists bool
	err := s.db.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM downtime_skip WHERE id = 1)").Scan(&exists)
	if err != nil {
		return err
	}

	if exists {
		// Update existing skip date
		_, err = s.db.ExecContext(ctx, `
			UPDATE downtime_skip SET skip_date = ? WHERE id = 1
		`, normalizedDate)
	} else {
		// Insert new skip date
		_, err = s.db.ExecContext(ctx, `
			INSERT INTO downtime_skip (id, skip_date, created_at)
			VALUES (1, ?, ?)
		`, normalizedDate, now)
	}

	return err
}

// ============================================================================
// DEVICE BYPASS STORAGE - For agent-controlled devices
// ============================================================================

// GetDeviceBypass retrieves the bypass status for a device
func (s *SQLiteStorage) GetDeviceBypass(ctx context.Context, deviceID string) (*core.DeviceBypass, error) {
	var bypass core.DeviceBypass
	var reason sql.NullString
	var enabledBy sql.NullString
	var expiresAt sql.NullTime

	err := s.db.QueryRowContext(ctx, `
		SELECT device_id, enabled, reason, enabled_at, enabled_by, expires_at
		FROM device_bypass WHERE device_id = ?
	`, deviceID).Scan(&bypass.DeviceID, &bypass.Enabled, &reason, &bypass.EnabledAt, &enabledBy, &expiresAt)

	if err == sql.ErrNoRows {
		return nil, nil // No bypass configured for this device
	}
	if err != nil {
		return nil, err
	}

	if reason.Valid {
		bypass.Reason = reason.String
	}
	if enabledBy.Valid {
		bypass.EnabledBy = enabledBy.String
	}
	if expiresAt.Valid {
		bypass.ExpiresAt = &expiresAt.Time
	}

	return &bypass, nil
}

// SetDeviceBypass sets or updates the bypass status for a device
func (s *SQLiteStorage) SetDeviceBypass(ctx context.Context, bypass *core.DeviceBypass) error {
	var expiresAt sql.NullTime
	if bypass.ExpiresAt != nil {
		expiresAt = sql.NullTime{Time: *bypass.ExpiresAt, Valid: true}
	}

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO device_bypass (device_id, enabled, reason, enabled_at, enabled_by, expires_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(device_id) DO UPDATE SET
			enabled = excluded.enabled,
			reason = excluded.reason,
			enabled_at = excluded.enabled_at,
			enabled_by = excluded.enabled_by,
			expires_at = excluded.expires_at
	`, bypass.DeviceID, bypass.Enabled, bypass.Reason, bypass.EnabledAt, bypass.EnabledBy, expiresAt)

	return err
}

// ClearDeviceBypass removes the bypass for a device
func (s *SQLiteStorage) ClearDeviceBypass(ctx context.Context, deviceID string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM device_bypass WHERE device_id = ?`, deviceID)
	return err
}

// ListActiveBypassDevices retrieves all devices with active bypass (enabled and not expired)
func (s *SQLiteStorage) ListActiveBypassDevices(ctx context.Context) ([]*core.DeviceBypass, error) {
	now := time.Now()

	rows, err := s.db.QueryContext(ctx, `
		SELECT device_id, enabled, reason, enabled_at, enabled_by, expires_at
		FROM device_bypass
		WHERE enabled = 1 AND (expires_at IS NULL OR expires_at > ?)
		ORDER BY enabled_at DESC
	`, now)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var bypasses []*core.DeviceBypass
	for rows.Next() {
		var bypass core.DeviceBypass
		var reason sql.NullString
		var enabledBy sql.NullString
		var expiresAt sql.NullTime

		if err := rows.Scan(&bypass.DeviceID, &bypass.Enabled, &reason, &bypass.EnabledAt, &enabledBy, &expiresAt); err != nil {
			return nil, err
		}

		if reason.Valid {
			bypass.Reason = reason.String
		}
		if enabledBy.Valid {
			bypass.EnabledBy = enabledBy.String
		}
		if expiresAt.Valid {
			bypass.ExpiresAt = &expiresAt.Time
		}

		bypasses = append(bypasses, &bypass)
	}

	return bypasses, rows.Err()
}
