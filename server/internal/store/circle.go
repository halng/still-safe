package store

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func (s *Store) Circle(ctx context.Context, userID string) (Circle, error) {
	var c Circle
	err := s.pool.QueryRow(ctx, `
		SELECT supervisor_limit, sharing_enabled,
			(SELECT count(*) FROM supervisor_links WHERE person_id = $1 AND status = 'active')
		FROM circles WHERE user_id = $1
	`, userID).Scan(&c.SupervisorLimit, &c.SharingEnabled, &c.ActiveSupervisors)
	if errors.Is(err, pgx.ErrNoRows) {
		return Circle{}, ErrNotFound
	}
	return c, err
}

func (s *Store) SetLimit(ctx context.Context, userID string, limit int, now time.Time) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var active int
	if err := tx.QueryRow(ctx, `
		SELECT count(*) FROM supervisor_links WHERE person_id = $1 AND status = 'active'
	`, userID).Scan(&active); err != nil {
		return err
	}
	if limit < active {
		return ErrLimit
	}
	tag, err := tx.Exec(ctx, `
		UPDATE circles SET supervisor_limit = $2, updated_at = $3 WHERE user_id = $1
	`, userID, limit, now)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return tx.Commit(ctx)
}

func (s *Store) CreateInvite(ctx context.Context, personID, code string, now time.Time) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var limit, active int
	if err := tx.QueryRow(ctx, `
		SELECT supervisor_limit,
			(SELECT count(*) FROM supervisor_links WHERE person_id = $1 AND status = 'active')
		FROM circles WHERE user_id = $1 FOR UPDATE
	`, personID).Scan(&limit, &active); err != nil {
		return err
	}
	if active >= limit {
		return ErrCap
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO invites (code, person_id, created_at) VALUES ($1, $2, $3)
	`, code, personID, now); err != nil {
		return mapUnique(err)
	}
	return tx.Commit(ctx)
}

func (s *Store) AcceptInvite(ctx context.Context, supervisorID, code string, now time.Time) (string, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer tx.Rollback(ctx)

	var personID string
	var usedAt *time.Time
	err = tx.QueryRow(ctx, `SELECT person_id, used_at FROM invites WHERE code = $1 FOR UPDATE`, code).Scan(&personID, &usedAt)
	if errors.Is(err, pgx.ErrNoRows) || usedAt != nil {
		return "", ErrNotFound
	}
	if err != nil {
		return "", err
	}
	if personID == supervisorID {
		return "", ErrSelf
	}

	var limit int
	if err := tx.QueryRow(ctx, `SELECT supervisor_limit FROM circles WHERE user_id = $1 FOR UPDATE`, personID).Scan(&limit); err != nil {
		return "", err
	}
	var active int
	if err := tx.QueryRow(ctx, `
		SELECT count(*) FROM supervisor_links WHERE person_id = $1 AND status = 'active'
	`, personID).Scan(&active); err != nil {
		return "", err
	}

	var status string
	err = tx.QueryRow(ctx, `
		SELECT status FROM supervisor_links WHERE person_id = $1 AND supervisor_id = $2 FOR UPDATE
	`, personID, supervisorID).Scan(&status)
	if errors.Is(err, pgx.ErrNoRows) {
		if active >= limit {
			return "", ErrCap
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO supervisor_links (id, person_id, supervisor_id, status, created_at)
			VALUES ($1, $2, $3, 'active', $4)
		`, uuid.NewString(), personID, supervisorID, now); err != nil {
			return "", err
		}
	} else if err != nil {
		return "", err
	} else if status == "active" {
		return "", ErrConflict
	} else {
		if active >= limit {
			return "", ErrCap
		}
		if _, err := tx.Exec(ctx, `
			UPDATE supervisor_links SET status = 'active', revoked_at = NULL
			WHERE person_id = $1 AND supervisor_id = $2
		`, personID, supervisorID); err != nil {
			return "", err
		}
	}

	tag, err := tx.Exec(ctx, `
		UPDATE invites SET used_at = $2, used_by = $3 WHERE code = $1 AND used_at IS NULL
	`, code, now, supervisorID)
	if err != nil {
		return "", err
	}
	if tag.RowsAffected() != 1 {
		return "", ErrNotFound
	}
	if err := tx.Commit(ctx); err != nil {
		return "", err
	}
	return personID, nil
}

func (s *Store) Supervisors(ctx context.Context, personID string) ([]Supervisor, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT l.id, u.id, u.display_name
		FROM supervisor_links l
		JOIN users u ON u.id = l.supervisor_id
		WHERE l.person_id = $1 AND l.status = 'active'
		ORDER BY l.created_at
	`, personID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Supervisor
	for rows.Next() {
		var sup Supervisor
		if err := rows.Scan(&sup.LinkID, &sup.UserID, &sup.DisplayName); err != nil {
			return nil, err
		}
		out = append(out, sup)
	}
	return out, rows.Err()
}

func (s *Store) Revoke(ctx context.Context, personID, linkID string, now time.Time) error {
	tag, err := s.pool.Exec(ctx, `
		UPDATE supervisor_links SET status = 'revoked', revoked_at = $3
		WHERE id = $1 AND person_id = $2 AND status = 'active'
	`, linkID, personID, now)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) IsActiveSupervisor(ctx context.Context, personID, supervisorID string) (bool, error) {
	var ok bool
	err := s.pool.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM supervisor_links
			WHERE person_id = $1 AND supervisor_id = $2 AND status = 'active'
		)
	`, personID, supervisorID).Scan(&ok)
	return ok, err
}

func (s *Store) Watching(ctx context.Context, supervisorID string) ([]Watched, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT u.id, u.display_name, c.sharing_enabled
		FROM supervisor_links l
		JOIN users u ON u.id = l.person_id
		JOIN circles c ON c.user_id = l.person_id
		WHERE l.supervisor_id = $1 AND l.status = 'active'
		ORDER BY u.display_name
	`, supervisorID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Watched
	for rows.Next() {
		var w Watched
		if err := rows.Scan(&w.PersonID, &w.DisplayName, &w.SharingEnabled); err != nil {
			return nil, err
		}
		out = append(out, w)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for i := range out {
		loc, err := s.LatestLocation(ctx, out[i].PersonID)
		if err == nil {
			out[i].LastLocation = &loc
		} else if !errors.Is(err, ErrNotFound) {
			return nil, err
		}
		chk, err := s.LatestCheck(ctx, out[i].PersonID)
		if err == nil {
			out[i].Check = &chk
		} else if !errors.Is(err, ErrNotFound) {
			return nil, err
		}
	}
	return out, nil
}
