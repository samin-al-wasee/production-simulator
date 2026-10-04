package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// ErrNotFound reports that a requested row does not exist.
var ErrNotFound = errors.New("store: not found")

// User is an application user (ADR-0031). It holds no credential: sign-in is
// a provider relationship recorded in oauth_accounts.
type User struct {
	ID          string
	Email       string
	DisplayName string
	AvatarURL   string
}

// UpsertUserByOAuth resolves the user behind a provider identity, creating or
// linking one on first sign-in. The identity key is
// (provider, provider_user_id); the provider profile refreshes the user's
// display fields when it supplies them.
func (s *Store) UpsertUserByOAuth(ctx context.Context, provider, providerUserID, email, displayName, avatarURL string) (User, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return User{}, fmt.Errorf("begin upsert user: %w", err)
	}
	defer tx.Rollback(ctx)

	var id string
	err = tx.QueryRow(ctx,
		`SELECT user_id::text FROM oauth_accounts WHERE provider = $1 AND provider_user_id = $2`,
		provider, providerUserID).Scan(&id)
	switch {
	case err == nil:
		// Already linked; refresh the profile below.
	case errors.Is(err, pgx.ErrNoRows):
		// First sign-in with this identity. Prefer linking to a user we already
		// have under the same email (one person, two providers), else create one.
		// ponytail: a concurrent first sign-in can race the email lookup; the
		// unique index rejects the loser. Add a retry only if it ever happens.
		if email != "" {
			_ = tx.QueryRow(ctx,
				`SELECT id::text FROM users WHERE email <> '' AND lower(email) = lower($1) LIMIT 1`,
				email).Scan(&id)
		}
		if id == "" {
			if err := tx.QueryRow(ctx,
				`INSERT INTO users (email, display_name, avatar_url) VALUES ($1, $2, $3) RETURNING id::text`,
				email, displayName, avatarURL).Scan(&id); err != nil {
				return User{}, fmt.Errorf("create user: %w", err)
			}
		}
		if _, err := tx.Exec(ctx,
			`INSERT INTO oauth_accounts (provider, provider_user_id, user_id) VALUES ($1, $2, $3::uuid)
			 ON CONFLICT (provider, provider_user_id) DO NOTHING`,
			provider, providerUserID, id); err != nil {
			return User{}, fmt.Errorf("link oauth account: %w", err)
		}
	default:
		return User{}, fmt.Errorf("lookup oauth account: %w", err)
	}

	if _, err := tx.Exec(ctx,
		`UPDATE users SET
			email        = CASE WHEN $2 <> '' THEN $2 ELSE email END,
			display_name = CASE WHEN $3 <> '' THEN $3 ELSE display_name END,
			avatar_url   = CASE WHEN $4 <> '' THEN $4 ELSE avatar_url END,
			updated_at   = now()
		 WHERE id = $1::uuid`,
		id, email, displayName, avatarURL); err != nil {
		return User{}, fmt.Errorf("update user: %w", err)
	}

	var u User
	if err := tx.QueryRow(ctx,
		`SELECT id::text, email, display_name, avatar_url FROM users WHERE id = $1::uuid`,
		id).Scan(&u.ID, &u.Email, &u.DisplayName, &u.AvatarURL); err != nil {
		return User{}, fmt.Errorf("read user: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return User{}, fmt.Errorf("commit upsert user: %w", err)
	}
	return u, nil
}

// CreateSession stores a session keyed by the SHA-256 hash of its token.
func (s *Store) CreateSession(ctx context.Context, tokenHash []byte, userID string, expiresAt time.Time) error {
	if _, err := s.pool.Exec(ctx,
		`INSERT INTO sessions (token_hash, user_id, expires_at) VALUES ($1, $2::uuid, $3)`,
		tokenHash, userID, expiresAt); err != nil {
		return fmt.Errorf("create session: %w", err)
	}
	return nil
}

// SessionUser returns the user behind a live session token hash, or ErrNotFound
// when the session is unknown or expired.
func (s *Store) SessionUser(ctx context.Context, tokenHash []byte) (User, error) {
	var u User
	err := s.pool.QueryRow(ctx,
		`SELECT u.id::text, u.email, u.display_name, u.avatar_url
		 FROM sessions s JOIN users u ON u.id = s.user_id
		 WHERE s.token_hash = $1 AND s.expires_at > now()`,
		tokenHash).Scan(&u.ID, &u.Email, &u.DisplayName, &u.AvatarURL)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrNotFound
	}
	if err != nil {
		return User{}, fmt.Errorf("lookup session: %w", err)
	}
	return u, nil
}

// DeleteSession removes a session, revoking it server-side.
func (s *Store) DeleteSession(ctx context.Context, tokenHash []byte) error {
	if _, err := s.pool.Exec(ctx, `DELETE FROM sessions WHERE token_hash = $1`, tokenHash); err != nil {
		return fmt.Errorf("delete session: %w", err)
	}
	return nil
}

// DeleteExpiredSessions clears sessions past their expiry.
func (s *Store) DeleteExpiredSessions(ctx context.Context) error {
	if _, err := s.pool.Exec(ctx, `DELETE FROM sessions WHERE expires_at <= now()`); err != nil {
		return fmt.Errorf("delete expired sessions: %w", err)
	}
	return nil
}
