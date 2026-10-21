package store

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

func (s *Store) CreateUser(ctx context.Context, id, name, nameKey, passphraseHash string, now time.Time) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `
		INSERT INTO users (id, display_name, display_name_key, passphrase_hash, created_at)
		VALUES ($1, $2, $3, $4, $5)
	`, id, name, nameKey, passphraseHash, now); err != nil {
		return mapUnique(err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO circles (user_id, supervisor_limit, sharing_enabled, updated_at)
		VALUES ($1, 1, FALSE, $2)
	`, id, now); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Store) UserByNameKey(ctx context.Context, nameKey string) (User, error) {
	var u User
	err := s.pool.QueryRow(ctx, `
		SELECT id, display_name, passphrase_hash FROM users WHERE display_name_key = $1
	`, nameKey).Scan(&u.ID, &u.DisplayName, &u.PassphraseHash)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrNotFound
	}
	return u, err
}

func (s *Store) CreateSession(ctx context.Context, tokenHash, userID string, now, expires time.Time) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO sessions (token_hash, user_id, expires_at, created_at) VALUES ($1, $2, $3, $4)
	`, tokenHash, userID, expires, now)
	return err
}

func (s *Store) DeleteSession(ctx context.Context, tokenHash string) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM sessions WHERE token_hash = $1`, tokenHash)
	return err
}

func (s *Store) UserByToken(ctx context.Context, tokenHash string, now time.Time) (User, error) {
	var u User
	err := s.pool.QueryRow(ctx, `
		SELECT u.id, u.display_name, (i.user_id IS NOT NULL)
		FROM sessions s
		JOIN users u ON u.id = s.user_id
		LEFT JOIN identity_records i ON i.user_id = u.id
		WHERE s.token_hash = $1 AND s.expires_at > $2
	`, tokenHash, now).Scan(&u.ID, &u.DisplayName, &u.IDOnFile)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrNotFound
	}
	return u, err
}

func (s *Store) SaveIdentity(ctx context.Context, userID, legalName, documentType, numberHash, last4, photoPath string, dob, now time.Time) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO identity_records (
			user_id, legal_name, date_of_birth, document_type, document_number_hash,
			document_last4, photo_path, status, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, 'id_on_file', $8)
	`, userID, legalName, dob, documentType, numberHash, last4, photoPath, now)
	return mapUnique(err)
}

func (s *Store) Identity(ctx context.Context, userID string) (Identity, error) {
	var idn Identity
	err := s.pool.QueryRow(ctx, `
		SELECT legal_name, date_of_birth, document_type, document_last4, status
		FROM identity_records WHERE user_id = $1
	`, userID).Scan(&idn.LegalName, &idn.DateOfBirth, &idn.DocumentType, &idn.DocumentLast4, &idn.Status)
	if errors.Is(err, pgx.ErrNoRows) {
		return Identity{}, ErrNotFound
	}
	return idn, err
}
