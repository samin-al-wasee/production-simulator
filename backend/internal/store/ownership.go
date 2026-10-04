package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// SandboxSummary is a saved sandbox without its command log.
type SandboxSummary struct {
	ID        string
	Name      string
	Ruleset   string
	Seed      int64
	Tick      int
	UpdatedAt time.Time
}

// SavedSandbox is a summary plus the raw sandbox.Save JSON to resume from.
type SavedSandbox struct {
	SandboxSummary
	Save string
}

// ProgressEntry is one completed exercise for a user.
type ProgressEntry struct {
	ExerciseID  string
	CompletedAt time.Time
	CompletedBy string
}

func sandboxSummary(id, name, ruleset string, seed int64, tick int, updatedAt time.Time) SandboxSummary {
	return SandboxSummary{ID: id, Name: name, Ruleset: ruleset, Seed: seed, Tick: tick, UpdatedAt: updatedAt}
}

// CreateSandbox stores a new saved sandbox for a user.
func (s *Store) CreateSandbox(ctx context.Context, userID, name, ruleset string, seed int64, tick int, saveJSON string) (SandboxSummary, error) {
	var sum SandboxSummary
	err := s.pool.QueryRow(ctx,
		`INSERT INTO sandboxes (user_id, name, ruleset, seed, tick, save)
		 VALUES ($1::uuid, $2, $3, $4, $5, $6::jsonb)
		 RETURNING id::text, name, ruleset, seed, tick, updated_at`,
		userID, name, ruleset, seed, tick, saveJSON).
		Scan(&sum.ID, &sum.Name, &sum.Ruleset, &sum.Seed, &sum.Tick, &sum.UpdatedAt)
	if err != nil {
		return SandboxSummary{}, fmt.Errorf("create sandbox: %w", err)
	}
	return sum, nil
}

// UpdateSandbox overwrites an owned sandbox, or returns ErrNotFound.
func (s *Store) UpdateSandbox(ctx context.Context, userID, id, name string, tick int, saveJSON string) error {
	tag, err := s.pool.Exec(ctx,
		`UPDATE sandboxes SET name = $3, tick = $4, save = $5::jsonb, updated_at = now()
		 WHERE id = $2::uuid AND user_id = $1::uuid`,
		userID, id, name, tick, saveJSON)
	if err != nil {
		return fmt.Errorf("update sandbox: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ListSandboxes returns a user's saved sandboxes, most recently updated first.
func (s *Store) ListSandboxes(ctx context.Context, userID string) ([]SandboxSummary, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id::text, name, ruleset, seed, tick, updated_at
		 FROM sandboxes WHERE user_id = $1::uuid ORDER BY updated_at DESC`, userID)
	if err != nil {
		return nil, fmt.Errorf("list sandboxes: %w", err)
	}
	defer rows.Close()
	out := []SandboxSummary{}
	for rows.Next() {
		var sum SandboxSummary
		if err := rows.Scan(&sum.ID, &sum.Name, &sum.Ruleset, &sum.Seed, &sum.Tick, &sum.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan sandbox: %w", err)
		}
		out = append(out, sum)
	}
	return out, rows.Err()
}

// GetSandbox returns an owned sandbox with its save, or ErrNotFound.
func (s *Store) GetSandbox(ctx context.Context, userID, id string) (SavedSandbox, error) {
	var sb SavedSandbox
	err := s.pool.QueryRow(ctx,
		`SELECT id::text, name, ruleset, seed, tick, updated_at, save::text
		 FROM sandboxes WHERE id = $2::uuid AND user_id = $1::uuid`, userID, id).
		Scan(&sb.ID, &sb.Name, &sb.Ruleset, &sb.Seed, &sb.Tick, &sb.UpdatedAt, &sb.Save)
	if errors.Is(err, pgx.ErrNoRows) {
		return SavedSandbox{}, ErrNotFound
	}
	if err != nil {
		return SavedSandbox{}, fmt.Errorf("get sandbox: %w", err)
	}
	return sb, nil
}

// DeleteSandbox removes an owned sandbox, or returns ErrNotFound.
func (s *Store) DeleteSandbox(ctx context.Context, userID, id string) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM sandboxes WHERE id = $2::uuid AND user_id = $1::uuid`, userID, id)
	if err != nil {
		return fmt.Errorf("delete sandbox: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ProgressForUser returns a user's completed exercises.
func (s *Store) ProgressForUser(ctx context.Context, userID string) ([]ProgressEntry, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT exercise_id, completed_at, completed_by FROM learning_progress WHERE user_id = $1::uuid`, userID)
	if err != nil {
		return nil, fmt.Errorf("list progress: %w", err)
	}
	defer rows.Close()
	out := []ProgressEntry{}
	for rows.Next() {
		var e ProgressEntry
		if err := rows.Scan(&e.ExerciseID, &e.CompletedAt, &e.CompletedBy); err != nil {
			return nil, fmt.Errorf("scan progress: %w", err)
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// RecordProgress marks exercises done for a user, keeping the first record of
// each, and returns the ids newly completed.
func (s *Store) RecordProgress(ctx context.Context, userID string, exerciseIDs []string, by string, at time.Time) ([]string, error) {
	rows, err := s.pool.Query(ctx,
		`INSERT INTO learning_progress (user_id, exercise_id, completed_at, completed_by)
		 SELECT $1::uuid, id, $3, $4 FROM unnest($2::text[]) AS id
		 ON CONFLICT (user_id, exercise_id) DO NOTHING
		 RETURNING exercise_id`,
		userID, exerciseIDs, at.UTC(), by)
	if err != nil {
		return nil, fmt.Errorf("record progress: %w", err)
	}
	defer rows.Close()
	var done []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan progress: %w", err)
		}
		done = append(done, id)
	}
	return done, rows.Err()
}
