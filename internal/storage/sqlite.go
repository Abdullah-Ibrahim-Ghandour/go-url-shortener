package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/Abdullah-Ibrahim-Ghandour/go-url-shortener/internal/shortener"

	_ "modernc.org/sqlite"
)

const schema = `
CREATE TABLE IF NOT EXISTS links (
	code TEXT PRIMARY KEY,
	original_url TEXT NOT NULL UNIQUE,
	created_at TEXT NOT NULL
);`

type SQLiteStore struct {
	db *sql.DB
}

func OpenSQLite(ctx context.Context, path string) (*SQLiteStore, error) {
	if path == "" {
		return nil, fmt.Errorf("sqlite path is required")
	}

	if path != ":memory:" {
		dir := filepath.Dir(path)
		if dir != "." {
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return nil, fmt.Errorf("create sqlite directory: %w", err)
			}
		}
	}

	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	db.SetMaxOpenConns(1)

	store := &SQLiteStore{db: db}
	if err := store.initialize(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}

	return store, nil
}

func (s *SQLiteStore) initialize(ctx context.Context) error {
	if _, err := s.db.ExecContext(ctx, "PRAGMA busy_timeout=5000"); err != nil {
		return fmt.Errorf("set sqlite busy timeout: %w", err)
	}

	if _, err := s.db.ExecContext(ctx, schema); err != nil {
		return fmt.Errorf("create links schema: %w", err)
	}

	return nil
}

func (s *SQLiteStore) FindByOriginalURL(ctx context.Context, originalURL string) (shortener.Link, error) {
	return s.find(ctx, "original_url = ?", originalURL)
}

func (s *SQLiteStore) FindByCode(ctx context.Context, code string) (shortener.Link, error) {
	return s.find(ctx, "code = ?", code)
}

func (s *SQLiteStore) Insert(ctx context.Context, link shortener.Link) (bool, error) {
	result, err := s.db.ExecContext(
		ctx,
		`INSERT OR IGNORE INTO links (code, original_url, created_at) VALUES (?, ?, ?)`,
		link.Code,
		link.OriginalURL,
		link.CreatedAt.UTC().Format(time.RFC3339Nano),
	)
	if err != nil {
		return false, fmt.Errorf("insert link: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("read inserted row count: %w", err)
	}

	return rows == 1, nil
}

func (s *SQLiteStore) Close() error {
	return s.db.Close()
}

func (s *SQLiteStore) find(ctx context.Context, predicate string, value string) (shortener.Link, error) {
	query := fmt.Sprintf(`SELECT code, original_url, created_at FROM links WHERE %s`, predicate)

	var link shortener.Link
	var createdAt string
	err := s.db.QueryRowContext(ctx, query, value).Scan(&link.Code, &link.OriginalURL, &createdAt)
	if errors.Is(err, sql.ErrNoRows) {
		return shortener.Link{}, shortener.ErrNotFound
	}
	if err != nil {
		return shortener.Link{}, fmt.Errorf("find link: %w", err)
	}

	link.CreatedAt, err = time.Parse(time.RFC3339Nano, createdAt)
	if err != nil {
		return shortener.Link{}, fmt.Errorf("parse link created_at: %w", err)
	}

	return link, nil
}
