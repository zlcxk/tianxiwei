package store

import (
	"context"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"xiweispace/backend/internal/models"
)

type Credential struct {
	User         models.User
	PasswordHash string
}

func (s *Store) CountUsers(ctx context.Context) (int, error) {
	var n int
	err := s.Pool.QueryRow(ctx, `SELECT count(*) FROM users`).Scan(&n)
	return n, err
}

// SetupAdmin atomically creates the first administrator. The advisory lock
// closes the race between two simultaneous first-run requests.
func (s *Store) SetupAdmin(ctx context.Context, email, passwordHash, nickname string) (models.User, error) {
	var u models.User
	err := s.withTx(ctx, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(982451653)`); err != nil {
			return err
		}
		var count int
		if err := tx.QueryRow(ctx, `SELECT count(*) FROM users`).Scan(&count); err != nil {
			return err
		}
		if count > 0 {
			return ErrConflict
		}
		return tx.QueryRow(ctx, `
			INSERT INTO users (email, password_hash, nickname, role)
			VALUES ($1, $2, $3, 'admin')
			RETURNING id, email, nickname, avatar, role, status, created_at`,
			strings.ToLower(strings.TrimSpace(email)), passwordHash, nickname,
		).Scan(&u.ID, &u.Email, &u.Nickname, &u.Avatar, &u.Role, &u.Status, &u.CreatedAt)
	})
	return u, err
}

func (s *Store) CreateUser(ctx context.Context, email, passwordHash, nickname, role string) (models.User, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	var u models.User
	err := s.Pool.QueryRow(ctx, `
		INSERT INTO users (email, password_hash, nickname, role)
		VALUES ($1, $2, $3, $4)
		RETURNING id, email, nickname, avatar, role, status, created_at`,
		email, passwordHash, nickname, role,
	).Scan(&u.ID, &u.Email, &u.Nickname, &u.Avatar, &u.Role, &u.Status, &u.CreatedAt)
	if err != nil && strings.Contains(err.Error(), "users_email_lower_uidx") {
		return u, ErrConflict
	}
	return u, err
}

func (s *Store) GetCredentialByEmail(ctx context.Context, email string) (Credential, error) {
	var c Credential
	err := s.Pool.QueryRow(ctx, `
		SELECT id, email, nickname, avatar, role, status, created_at, password_hash
		FROM users WHERE lower(email) = lower($1)`, strings.TrimSpace(email),
	).Scan(&c.User.ID, &c.User.Email, &c.User.Nickname, &c.User.Avatar, &c.User.Role, &c.User.Status, &c.User.CreatedAt, &c.PasswordHash)
	return c, mapNoRows(err)
}

func (s *Store) GetUser(ctx context.Context, id string) (models.User, error) {
	var u models.User
	err := s.Pool.QueryRow(ctx, `
		SELECT id, email, nickname, avatar, role, status, created_at
		FROM users WHERE id = $1::uuid`, id,
	).Scan(&u.ID, &u.Email, &u.Nickname, &u.Avatar, &u.Role, &u.Status, &u.CreatedAt)
	return u, mapNoRows(err)
}

func (s *Store) CreateSession(ctx context.Context, userID, tokenHash, csrfToken, userAgent, ip string, expiresAt time.Time) error {
	_, err := s.Pool.Exec(ctx, `
		INSERT INTO sessions (token_hash, user_id, csrf_token, user_agent, ip, expires_at)
		VALUES ($1, $2::uuid, $3, $4, $5, $6)`,
		tokenHash, userID, csrfToken, userAgent, ip, expiresAt)
	return err
}

func (s *Store) GetSession(ctx context.Context, tokenHash string) (models.AuthSession, error) {
	var a models.AuthSession
	err := s.Pool.QueryRow(ctx, `
		SELECT u.id, u.email, u.nickname, u.avatar, u.role, u.status, u.created_at,
		       s.csrf_token, s.expires_at
		FROM sessions s JOIN users u ON u.id = s.user_id
		WHERE s.token_hash = $1 AND s.expires_at > now() AND u.status = 'active'`,
		tokenHash,
	).Scan(&a.User.ID, &a.User.Email, &a.User.Nickname, &a.User.Avatar, &a.User.Role, &a.User.Status, &a.User.CreatedAt, &a.CSRFToken, &a.ExpiresAt)
	return a, mapNoRows(err)
}

func (s *Store) DeleteSession(ctx context.Context, tokenHash string) error {
	_, err := s.Pool.Exec(ctx, `DELETE FROM sessions WHERE token_hash = $1`, tokenHash)
	return err
}

func (s *Store) CleanupSessions(ctx context.Context) error {
	_, err := s.Pool.Exec(ctx, `DELETE FROM sessions WHERE expires_at <= now()`)
	return err
}
