package store

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

const locationThrottle = 45 * time.Second

func (s *Store) SetSharing(ctx context.Context, userID string, enabled bool, now time.Time) error {
	tag, err := s.pool.Exec(ctx, `
		UPDATE circles SET sharing_enabled = $2, updated_at = $3 WHERE user_id = $1
	`, userID, enabled, now)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) AddLocation(ctx context.Context, personID string, lat, lng, accuracy float64, now time.Time) (bool, Location, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return false, Location{}, err
	}
	defer tx.Rollback(ctx)

	var sharing bool
	if err := tx.QueryRow(ctx, `SELECT sharing_enabled FROM circles WHERE user_id = $1 FOR UPDATE`, personID).Scan(&sharing); err != nil {
		return false, Location{}, err
	}
	if !sharing {
		return false, Location{}, ErrSharing
	}

	var last time.Time
	err = tx.QueryRow(ctx, `
		SELECT recorded_at FROM location_points WHERE person_id = $1 ORDER BY recorded_at DESC LIMIT 1
	`, personID).Scan(&last)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return false, Location{}, err
	}
	if err == nil && now.Sub(last) < locationThrottle {
		return false, Location{}, nil
	}

	loc := Location{Latitude: lat, Longitude: lng, AccuracyM: accuracy, RecordedAt: now}
	if _, err := tx.Exec(ctx, `
		INSERT INTO location_points (id, person_id, latitude, longitude, accuracy_m, recorded_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, uuid.NewString(), personID, lat, lng, accuracy, now); err != nil {
		return false, Location{}, err
	}
	if _, err := tx.Exec(ctx, `
		DELETE FROM location_points WHERE person_id = $1 AND recorded_at < $2
	`, personID, now.Add(-24*time.Hour)); err != nil {
		return false, Location{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return false, Location{}, err
	}
	return true, loc, nil
}

func (s *Store) LatestLocation(ctx context.Context, personID string) (Location, error) {
	var loc Location
	err := s.pool.QueryRow(ctx, `
		SELECT latitude, longitude, accuracy_m, recorded_at
		FROM location_points WHERE person_id = $1
		ORDER BY recorded_at DESC LIMIT 1
	`, personID).Scan(&loc.Latitude, &loc.Longitude, &loc.AccuracyM, &loc.RecordedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Location{}, ErrNotFound
	}
	return loc, err
}

func (s *Store) Trail(ctx context.Context, personID string, since time.Time) ([]Location, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT latitude, longitude, accuracy_m, recorded_at
		FROM location_points
		WHERE person_id = $1 AND recorded_at >= $2
		ORDER BY recorded_at ASC
	`, personID, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Location
	for rows.Next() {
		var loc Location
		if err := rows.Scan(&loc.Latitude, &loc.Longitude, &loc.AccuracyM, &loc.RecordedAt); err != nil {
			return nil, err
		}
		out = append(out, loc)
	}
	return out, rows.Err()
}

func (s *Store) CreatePlace(ctx context.Context, personID, label string, lat, lng float64, now time.Time) (Place, error) {
	var n int
	if err := s.pool.QueryRow(ctx, `SELECT count(*) FROM places WHERE person_id = $1`, personID).Scan(&n); err != nil {
		return Place{}, err
	}
	if n >= 20 {
		return Place{}, ErrCap
	}
	p := Place{ID: uuid.NewString(), Label: label, Latitude: lat, Longitude: lng}
	_, err := s.pool.Exec(ctx, `
		INSERT INTO places (id, person_id, label, latitude, longitude, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, p.ID, personID, label, lat, lng, now)
	return p, err
}

func (s *Store) Places(ctx context.Context, personID string) ([]Place, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, label, latitude, longitude FROM places WHERE person_id = $1 ORDER BY created_at
	`, personID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Place
	for rows.Next() {
		var p Place
		if err := rows.Scan(&p.ID, &p.Label, &p.Latitude, &p.Longitude); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (s *Store) DeletePlace(ctx context.Context, personID, placeID string) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM places WHERE id = $1 AND person_id = $2`, placeID, personID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
