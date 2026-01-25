package sqlite

import (
	"context"
	"database/sql"
	"metron/internal/core"
	"time"
)

// GetMovieTimeUsage retrieves movie time usage for a specific date
func (s *SQLiteStorage) GetMovieTimeUsage(ctx context.Context, date time.Time) (*core.MovieTimeUsage, error) {
	normalizedDate := s.normalizeDate(date)

	var usage core.MovieTimeUsage
	var sessionID sql.NullString
	var startedAt sql.NullTime
	var startedBy sql.NullString

	err := s.db.QueryRowContext(ctx, `
		SELECT date, session_id, started_at, started_by, status, created_at, updated_at
		FROM movie_time_usage WHERE date = ?
	`, normalizedDate).Scan(&usage.Date, &sessionID, &startedAt, &startedBy, &usage.Status, &usage.CreatedAt, &usage.UpdatedAt)

	if err == sql.ErrNoRows {
		return nil, nil // No usage record for this date
	}
	if err != nil {
		return nil, err
	}

	if sessionID.Valid {
		usage.SessionID = sessionID.String
	}
	if startedAt.Valid {
		usage.StartedAt = &startedAt.Time
	}
	if startedBy.Valid {
		usage.StartedBy = startedBy.String
	}

	return &usage, nil
}

// SaveMovieTimeUsage saves or updates movie time usage for a date
func (s *SQLiteStorage) SaveMovieTimeUsage(ctx context.Context, usage *core.MovieTimeUsage) error {
	normalizedDate := s.normalizeDate(usage.Date)
	now := time.Now()

	var sessionID sql.NullString
	if usage.SessionID != "" {
		sessionID = sql.NullString{String: usage.SessionID, Valid: true}
	}

	var startedAt sql.NullTime
	if usage.StartedAt != nil {
		startedAt = sql.NullTime{Time: *usage.StartedAt, Valid: true}
	}

	var startedBy sql.NullString
	if usage.StartedBy != "" {
		startedBy = sql.NullString{String: usage.StartedBy, Valid: true}
	}

	// Use upsert pattern
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO movie_time_usage (date, session_id, started_at, started_by, status, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(date) DO UPDATE SET
			session_id = excluded.session_id,
			started_at = excluded.started_at,
			started_by = excluded.started_by,
			status = excluded.status,
			updated_at = excluded.updated_at
	`, normalizedDate, sessionID, startedAt, startedBy, usage.Status, now, now)

	return err
}

// CreateMovieTimeBypass creates a new movie time bypass period
func (s *SQLiteStorage) CreateMovieTimeBypass(ctx context.Context, bypass *core.MovieTimeBypass) error {
	now := time.Now()
	startDate := s.normalizeDate(bypass.StartDate)
	endDate := s.normalizeDate(bypass.EndDate)

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO movie_time_bypass (id, reason, start_date, end_date, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, bypass.ID, bypass.Reason, startDate, endDate, now, now)

	return err
}

// GetMovieTimeBypass retrieves a movie time bypass by ID
func (s *SQLiteStorage) GetMovieTimeBypass(ctx context.Context, id string) (*core.MovieTimeBypass, error) {
	var bypass core.MovieTimeBypass

	err := s.db.QueryRowContext(ctx, `
		SELECT id, reason, start_date, end_date, created_at, updated_at
		FROM movie_time_bypass WHERE id = ?
	`, id).Scan(&bypass.ID, &bypass.Reason, &bypass.StartDate, &bypass.EndDate, &bypass.CreatedAt, &bypass.UpdatedAt)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return &bypass, nil
}

// ListMovieTimeBypasses retrieves all movie time bypass periods
func (s *SQLiteStorage) ListMovieTimeBypasses(ctx context.Context) ([]*core.MovieTimeBypass, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, reason, start_date, end_date, created_at, updated_at
		FROM movie_time_bypass
		ORDER BY start_date DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var bypasses []*core.MovieTimeBypass
	for rows.Next() {
		var bypass core.MovieTimeBypass
		if err := rows.Scan(&bypass.ID, &bypass.Reason, &bypass.StartDate, &bypass.EndDate, &bypass.CreatedAt, &bypass.UpdatedAt); err != nil {
			return nil, err
		}
		bypasses = append(bypasses, &bypass)
	}

	return bypasses, rows.Err()
}

// ListActiveMovieTimeBypasses retrieves bypass periods that are active for a specific date
func (s *SQLiteStorage) ListActiveMovieTimeBypasses(ctx context.Context, date time.Time) ([]*core.MovieTimeBypass, error) {
	normalizedDate := s.normalizeDate(date)

	rows, err := s.db.QueryContext(ctx, `
		SELECT id, reason, start_date, end_date, created_at, updated_at
		FROM movie_time_bypass
		WHERE start_date <= ? AND end_date >= ?
		ORDER BY start_date DESC
	`, normalizedDate, normalizedDate)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var bypasses []*core.MovieTimeBypass
	for rows.Next() {
		var bypass core.MovieTimeBypass
		if err := rows.Scan(&bypass.ID, &bypass.Reason, &bypass.StartDate, &bypass.EndDate, &bypass.CreatedAt, &bypass.UpdatedAt); err != nil {
			return nil, err
		}
		bypasses = append(bypasses, &bypass)
	}

	return bypasses, rows.Err()
}

// DeleteMovieTimeBypass deletes a movie time bypass by ID
func (s *SQLiteStorage) DeleteMovieTimeBypass(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM movie_time_bypass WHERE id = ?`, id)
	return err
}
