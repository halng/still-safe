package store

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"io/fs"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed migrations/*.sql
var migrations embed.FS

var (
	ErrNotFound = errors.New("not found")
	ErrConflict = errors.New("conflict")
	ErrCap      = errors.New("supervisor limit reached")
	ErrLimit    = errors.New("limit below active supervisors")
	ErrPending  = errors.New("check already open")
	ErrWindow   = errors.New("window closed")
	ErrSharing  = errors.New("sharing off")
	ErrSelf     = errors.New("self")
)

type Store struct {
	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

func Connect(ctx context.Context, url string) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		return nil, err
	}
	return pgxpool.NewWithConfig(ctx, cfg)
}

func (s *Store) Migrate(ctx context.Context) error {
	if _, err := s.pool.Exec(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (name TEXT PRIMARY KEY)`); err != nil {
		return err
	}
	entries, err := fs.ReadDir(migrations, "migrations")
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		var done bool
		if err := s.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE name = $1)`, entry.Name()).Scan(&done); err != nil {
			return err
		}
		if done {
			continue
		}
		body, err := fs.ReadFile(migrations, "migrations/"+entry.Name())
		if err != nil {
			return err
		}
		conn, err := s.pool.Acquire(ctx)
		if err != nil {
			return err
		}
		result := conn.Conn().PgConn().Exec(ctx, string(body))
		_, execErr := result.ReadAll()
		if execErr != nil {
			conn.Release()
			return fmt.Errorf("%s: %w", entry.Name(), execErr)
		}
		_, err = conn.Exec(ctx, `INSERT INTO schema_migrations (name) VALUES ($1)`, entry.Name())
		conn.Release()
		if err != nil {
			return err
		}
	}
	return nil
}

func mapUnique(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return ErrConflict
	}
	return err
}
