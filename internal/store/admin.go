package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

type Admin struct {
	ID           string
	Username     string
	PasswordHash string
	DisabledAt   *time.Time
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type AdminSession struct {
	ID         string
	AdminID    string
	Username   string
	TokenHash  []byte
	CSRFToken  string
	ExpiresAt  time.Time
	LastSeenAt time.Time
	RevokedAt  *time.Time
	CreatedAt  time.Time
}

func (s *Store) CountAdmins(ctx context.Context) (int, error) {
	var count int
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM admins").Scan(&count); err != nil {
		return 0, fmt.Errorf("count admins: %w", err)
	}
	return count, nil
}

func (s *Store) CreateAdmin(ctx context.Context, admin Admin) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO admins(id, singleton, username, password_hash, created_at, updated_at)
		VALUES (?, 1, ?, ?, ?, ?)
	`, admin.ID, admin.Username, admin.PasswordHash,
		admin.CreatedAt.UnixMilli(), admin.UpdatedAt.UnixMilli())
	if err != nil {
		return fmt.Errorf("create admin: %w", err)
	}
	return nil
}

func (s *Store) GetAdminByUsername(ctx context.Context, username string) (Admin, error) {
	var admin Admin
	var disabled sql.NullInt64
	var created, updated int64
	err := s.db.QueryRowContext(ctx, `
		SELECT id, username, password_hash, disabled_at, created_at, updated_at
		FROM admins WHERE username = ? COLLATE NOCASE
	`, username).Scan(
		&admin.ID, &admin.Username, &admin.PasswordHash, &disabled, &created, &updated,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Admin{}, ErrNotFound
		}
		return Admin{}, fmt.Errorf("get admin: %w", err)
	}
	admin.DisabledAt = nullableTime(disabled)
	admin.CreatedAt = unixTime(created)
	admin.UpdatedAt = unixTime(updated)
	return admin, nil
}

func (s *Store) CreateAdminSession(ctx context.Context, session AdminSession) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO admin_sessions(
			id, admin_id, token_hash, csrf_token, expires_at, last_seen_at, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?)
	`, session.ID, session.AdminID, session.TokenHash, session.CSRFToken,
		session.ExpiresAt.UnixMilli(), session.LastSeenAt.UnixMilli(), session.CreatedAt.UnixMilli())
	if err != nil {
		return fmt.Errorf("create admin session: %w", err)
	}
	return nil
}

func (s *Store) GetAdminSession(ctx context.Context, tokenHash []byte) (AdminSession, error) {
	var session AdminSession
	var expires, lastSeen, created int64
	var revoked sql.NullInt64
	err := s.db.QueryRowContext(ctx, `
		SELECT s.id, s.admin_id, a.username, s.token_hash, s.csrf_token,
		       s.expires_at, s.last_seen_at, s.revoked_at, s.created_at
		FROM admin_sessions s
		JOIN admins a ON a.id = s.admin_id
		WHERE s.token_hash = ? AND a.disabled_at IS NULL
	`, tokenHash).Scan(
		&session.ID, &session.AdminID, &session.Username, &session.TokenHash,
		&session.CSRFToken, &expires, &lastSeen, &revoked, &created,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return AdminSession{}, ErrNotFound
		}
		return AdminSession{}, fmt.Errorf("get admin session: %w", err)
	}
	session.ExpiresAt = unixTime(expires)
	session.LastSeenAt = unixTime(lastSeen)
	session.RevokedAt = nullableTime(revoked)
	session.CreatedAt = unixTime(created)
	return session, nil
}

func (s *Store) TouchAdminSession(ctx context.Context, id string, now time.Time) error {
	_, err := s.db.ExecContext(ctx,
		"UPDATE admin_sessions SET last_seen_at = ? WHERE id = ? AND revoked_at IS NULL",
		now.UnixMilli(), id,
	)
	if err != nil {
		return fmt.Errorf("touch admin session: %w", err)
	}
	return nil
}

func (s *Store) RevokeAdminSession(ctx context.Context, id string, now time.Time) error {
	_, err := s.db.ExecContext(ctx,
		"UPDATE admin_sessions SET revoked_at = ? WHERE id = ? AND revoked_at IS NULL",
		now.UnixMilli(), id,
	)
	if err != nil {
		return fmt.Errorf("revoke admin session: %w", err)
	}
	return nil
}
