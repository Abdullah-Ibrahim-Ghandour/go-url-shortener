package storage

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"go-url-shortener/internal/shortener"
)

func TestSQLiteStoreInsertsAndFindsLink(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t, filepath.Join(t.TempDir(), "links.db"))
	defer closeStore(t, store)

	link := shortener.Link{
		Code:        "Ab3dE9xY",
		OriginalURL: "https://example.com/articles/1",
		CreatedAt:   time.Date(2026, 6, 20, 10, 0, 0, 123, time.UTC),
	}

	inserted, err := store.Insert(ctx, link)
	if err != nil {
		t.Fatalf("insert link: %v", err)
	}
	if !inserted {
		t.Fatal("expected first insert to insert a row")
	}

	byCode, err := store.FindByCode(ctx, link.Code)
	if err != nil {
		t.Fatalf("find by code: %v", err)
	}
	assertLink(t, byCode, link)

	byOriginalURL, err := store.FindByOriginalURL(ctx, link.OriginalURL)
	if err != nil {
		t.Fatalf("find by original URL: %v", err)
	}
	assertLink(t, byOriginalURL, link)
}

func TestSQLiteStoreReportsDuplicateOriginalURLAsInsertMiss(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t, filepath.Join(t.TempDir(), "links.db"))
	defer closeStore(t, store)

	first := shortener.Link{
		Code:        "Ab3dE9xY",
		OriginalURL: "https://example.com/articles/1",
		CreatedAt:   time.Now().UTC(),
	}
	second := shortener.Link{
		Code:        "Zx9Wv8Ut",
		OriginalURL: first.OriginalURL,
		CreatedAt:   first.CreatedAt.Add(time.Second),
	}

	if inserted, err := store.Insert(ctx, first); err != nil || !inserted {
		t.Fatalf("insert first link = %v, %v; want true, nil", inserted, err)
	}

	inserted, err := store.Insert(ctx, second)
	if err != nil {
		t.Fatalf("insert duplicate original URL: %v", err)
	}
	if inserted {
		t.Fatal("expected duplicate original URL to be ignored")
	}

	got, err := store.FindByOriginalURL(ctx, first.OriginalURL)
	if err != nil {
		t.Fatalf("find duplicate original URL: %v", err)
	}
	if got.Code != first.Code {
		t.Fatalf("duplicate original URL changed stored code to %q; want %q", got.Code, first.Code)
	}
}

func TestSQLiteStoreReportsDuplicateCodeAsInsertMiss(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t, filepath.Join(t.TempDir(), "links.db"))
	defer closeStore(t, store)

	first := shortener.Link{
		Code:        "Ab3dE9xY",
		OriginalURL: "https://example.com/articles/1",
		CreatedAt:   time.Now().UTC(),
	}
	second := shortener.Link{
		Code:        first.Code,
		OriginalURL: "https://example.org/articles/2",
		CreatedAt:   first.CreatedAt.Add(time.Second),
	}

	if inserted, err := store.Insert(ctx, first); err != nil || !inserted {
		t.Fatalf("insert first link = %v, %v; want true, nil", inserted, err)
	}

	inserted, err := store.Insert(ctx, second)
	if err != nil {
		t.Fatalf("insert duplicate code: %v", err)
	}
	if inserted {
		t.Fatal("expected duplicate code to be ignored")
	}

	got, err := store.FindByCode(ctx, first.Code)
	if err != nil {
		t.Fatalf("find duplicate code: %v", err)
	}
	if got.OriginalURL != first.OriginalURL {
		t.Fatalf("duplicate code changed stored URL to %q; want %q", got.OriginalURL, first.OriginalURL)
	}
}

func TestSQLiteStoreReturnsNotFound(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t, filepath.Join(t.TempDir(), "links.db"))
	defer closeStore(t, store)

	_, err := store.FindByCode(ctx, "Ab3dE9xY")
	if !errors.Is(err, shortener.ErrNotFound) {
		t.Fatalf("find missing code error = %v; want %v", err, shortener.ErrNotFound)
	}

	_, err = store.FindByOriginalURL(ctx, "https://example.com")
	if !errors.Is(err, shortener.ErrNotFound) {
		t.Fatalf("find missing URL error = %v; want %v", err, shortener.ErrNotFound)
	}
}

func TestSQLiteStorePersistsAcrossReopen(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "links.db")
	link := shortener.Link{
		Code:        "Ab3dE9xY",
		OriginalURL: "https://example.com/articles/1",
		CreatedAt:   time.Now().UTC(),
	}

	store := openTestStore(t, dbPath)
	if inserted, err := store.Insert(ctx, link); err != nil || !inserted {
		t.Fatalf("insert link = %v, %v; want true, nil", inserted, err)
	}
	closeStore(t, store)

	reopened := openTestStore(t, dbPath)
	defer closeStore(t, reopened)

	got, err := reopened.FindByCode(ctx, link.Code)
	if err != nil {
		t.Fatalf("find persisted link: %v", err)
	}
	assertLink(t, got, link)
}

func openTestStore(t *testing.T, path string) *SQLiteStore {
	t.Helper()

	store, err := OpenSQLite(context.Background(), path)
	if err != nil {
		t.Fatalf("open sqlite store: %v", err)
	}

	return store
}

func closeStore(t *testing.T, store *SQLiteStore) {
	t.Helper()

	if err := store.Close(); err != nil {
		t.Fatalf("close sqlite store: %v", err)
	}
}

func assertLink(t *testing.T, got shortener.Link, want shortener.Link) {
	t.Helper()

	if got.Code != want.Code {
		t.Fatalf("code = %q; want %q", got.Code, want.Code)
	}
	if got.OriginalURL != want.OriginalURL {
		t.Fatalf("original URL = %q; want %q", got.OriginalURL, want.OriginalURL)
	}
	if !got.CreatedAt.Equal(want.CreatedAt) {
		t.Fatalf("created at = %s; want %s", got.CreatedAt, want.CreatedAt)
	}
}
