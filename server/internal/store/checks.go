package store

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"safecircle/internal/checkflow"
)

const checkCols = `
	c.id, c.person_id, person.display_name, c.started_by, starter.display_name,
	c.reason, c.status, c.started_at, c.respond_by, c.resolved_at,
	COALESCE(confirmer.display_name, '')
`

const checkFrom = `
	FROM safety_checks c
	JOIN users person ON person.id = c.person_id
	JOIN users starter ON starter.id = c.started_by
	LEFT JOIN users confirmer ON confirmer.id = c.confirmed_by
`

func scanCheck(row pgx.Row) (Check, error) {
	var c Check
	err := row.Scan(
		&c.ID, &c.PersonID, &c.PersonName, &c.StartedBy, &c.StartedByName,
		&c.Reason, &c.Status, &c.StartedAt, &c.RespondBy, &c.ResolvedAt, &c.ConfirmedByName,
	)
	return c, err
}

func (s *Store) ExpireChecks(ctx context.Context, now time.Time) ([]Check, error) {
	rows, err := s.pool.Query(ctx, `
		UPDATE safety_checks SET status = 'unanswered', resolved_at = $1
		WHERE status = 'pending' AND respond_by <= $1
		RETURNING id
	`, now)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	out := make([]Check, 0, len(ids))
	for _, id := range ids {
		chk, err := s.GetCheck(ctx, id)
		if err != nil {
			return nil, err
		}
		out = append(out, chk)
	}
	return out, nil
}

func (s *Store) GetCheck(ctx context.Context, id string) (Check, error) {
	chk, err := scanCheck(s.pool.QueryRow(ctx, `SELECT `+checkCols+checkFrom+` WHERE c.id = $1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return Check{}, ErrNotFound
	}
	return chk, err
}

func (s *Store) LatestCheck(ctx context.Context, personID string) (Check, error) {
	chk, err := scanCheck(s.pool.QueryRow(ctx, `
		SELECT `+checkCols+checkFrom+`
		WHERE c.person_id = $1
		ORDER BY c.started_at DESC
		LIMIT 1
	`, personID))
	if errors.Is(err, pgx.ErrNoRows) {
		return Check{}, ErrNotFound
	}
	return chk, err
}

func (s *Store) PendingCheck(ctx context.Context, personID string) (Check, error) {
	chk, err := scanCheck(s.pool.QueryRow(ctx, `
		SELECT `+checkCols+checkFrom+`
		WHERE c.person_id = $1 AND c.status = 'pending'
		ORDER BY c.started_at DESC
		LIMIT 1
	`, personID))
	if errors.Is(err, pgx.ErrNoRows) {
		return Check{}, ErrNotFound
	}
	return chk, err
}

func (s *Store) CreateCheck(ctx context.Context, personID, supervisorID, reason string, now time.Time) (Check, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Check{}, err
	}
	defer tx.Rollback(ctx)

	var linked bool
	if err := tx.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM supervisor_links
			WHERE person_id = $1 AND supervisor_id = $2 AND status = 'active'
		)
	`, personID, supervisorID).Scan(&linked); err != nil {
		return Check{}, err
	}
	if !linked {
		return Check{}, ErrNotFound
	}

	if _, err := tx.Exec(ctx, `
		UPDATE safety_checks SET status = 'unanswered', resolved_at = $2
		WHERE person_id = $1 AND status = 'pending' AND respond_by <= $2
	`, personID, now); err != nil {
		return Check{}, err
	}

	var pending bool
	if err := tx.QueryRow(ctx, `
		SELECT EXISTS(SELECT 1 FROM safety_checks WHERE person_id = $1 AND status = 'pending')
	`, personID).Scan(&pending); err != nil {
		return Check{}, err
	}
	if pending {
		return Check{}, ErrPending
	}

	id := uuid.NewString()
	if _, err := tx.Exec(ctx, `
		INSERT INTO safety_checks (
			id, person_id, started_by, reason, status, started_at, respond_by
		) VALUES ($1, $2, $3, $4, 'pending', $5, $6)
	`, id, personID, supervisorID, reason, now, now.Add(checkflow.Window)); err != nil {
		return Check{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Check{}, err
	}
	return s.GetCheck(ctx, id)
}

func (s *Store) ConfirmCheck(ctx context.Context, checkID, personID string, now time.Time) (Check, error) {
	tag, err := s.pool.Exec(ctx, `
		UPDATE safety_checks
		SET status = 'confirmed_safe', resolved_at = $3, confirmed_by = $2
		WHERE id = $1 AND person_id = $2 AND status = 'pending' AND respond_by > $3
	`, checkID, personID, now)
	if err != nil {
		return Check{}, err
	}
	if tag.RowsAffected() == 1 {
		return s.GetCheck(ctx, checkID)
	}
	tag, err = s.pool.Exec(ctx, `
		UPDATE safety_checks SET status = 'unanswered', resolved_at = $3
		WHERE id = $1 AND person_id = $2 AND status = 'pending' AND respond_by <= $3
	`, checkID, personID, now)
	if err != nil {
		return Check{}, err
	}
	if tag.RowsAffected() == 1 {
		return Check{}, ErrWindow
	}
	return Check{}, ErrNotFound
}
