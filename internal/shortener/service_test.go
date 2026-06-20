package shortener

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestServiceEncodeReturnsShortURL(t *testing.T) {
	store := newMemoryStore()
	service := newTestService(t, store, "Ab3dE9xY")

	shortURL, err := service.Encode(context.Background(), "https://example.com/articles/1")
	if err != nil {
		t.Fatalf("encode URL: %v", err)
	}

	if shortURL != "http://localhost:8080/Ab3dE9xY" {
		t.Fatalf("short URL = %q; want %q", shortURL, "http://localhost:8080/Ab3dE9xY")
	}
}

func TestServiceEncodeIsIdempotent(t *testing.T) {
	store := newMemoryStore()
	service := newTestService(t, store, "Ab3dE9xY", "Zx9Wv8Ut")

	first, err := service.Encode(context.Background(), "https://example.com/articles/1")
	if err != nil {
		t.Fatalf("first encode: %v", err)
	}

	second, err := service.Encode(context.Background(), "https://example.com/articles/1")
	if err != nil {
		t.Fatalf("second encode: %v", err)
	}

	if second != first {
		t.Fatalf("second short URL = %q; want %q", second, first)
	}
	if store.insertCalls != 1 {
		t.Fatalf("insert calls = %d; want 1", store.insertCalls)
	}
}

func TestServiceReturnsExistingURLWhenConcurrentInsertCreatesOriginalURL(t *testing.T) {
	store := &concurrentOriginalURLStore{
		existing: Link{
			Code:        "Ab3dE9xY",
			OriginalURL: "https://example.com/articles/1",
			CreatedAt:   time.Now().UTC(),
		},
	}
	service := newTestService(t, store, "Zx9Wv8Ut")

	shortURL, err := service.Encode(context.Background(), "https://example.com/articles/1")
	if err != nil {
		t.Fatalf("encode URL after concurrent insert: %v", err)
	}

	if shortURL != "http://localhost:8080/Ab3dE9xY" {
		t.Fatalf("short URL = %q; want %q", shortURL, "http://localhost:8080/Ab3dE9xY")
	}
}

func TestServiceDecodeReturnsOriginalURL(t *testing.T) {
	store := newMemoryStore()
	service := newTestService(t, store, "Ab3dE9xY")

	shortURL, err := service.Encode(context.Background(), "https://example.com/articles/1")
	if err != nil {
		t.Fatalf("encode URL: %v", err)
	}

	originalURL, err := service.Decode(context.Background(), shortURL)
	if err != nil {
		t.Fatalf("decode short URL: %v", err)
	}

	if originalURL != "https://example.com/articles/1" {
		t.Fatalf("original URL = %q; want %q", originalURL, "https://example.com/articles/1")
	}
}

func TestServiceRetriesCodeCollision(t *testing.T) {
	store := newMemoryStore()
	store.mustInsert(t, Link{
		Code:        "Ab3dE9xY",
		OriginalURL: "https://example.org/existing",
		CreatedAt:   time.Now().UTC(),
	})
	service := newTestService(t, store, "Ab3dE9xY", "Zx9Wv8Ut")

	shortURL, err := service.Encode(context.Background(), "https://example.com/articles/1")
	if err != nil {
		t.Fatalf("encode URL after collision: %v", err)
	}

	if shortURL != "http://localhost:8080/Zx9Wv8Ut" {
		t.Fatalf("short URL = %q; want %q", shortURL, "http://localhost:8080/Zx9Wv8Ut")
	}
	if store.insertCalls != 2 {
		t.Fatalf("insert calls = %d; want 2", store.insertCalls)
	}
}

func TestServiceReturnsErrorWhenCodeRetriesAreExhausted(t *testing.T) {
	store := newMemoryStore()
	store.mustInsert(t, Link{
		Code:        "Ab3dE9xY",
		OriginalURL: "https://example.org/existing",
		CreatedAt:   time.Now().UTC(),
	})
	service := newTestService(t, store, "Ab3dE9xY", "Ab3dE9xY", "Ab3dE9xY", "Ab3dE9xY", "Ab3dE9xY")

	_, err := service.Encode(context.Background(), "https://example.com/articles/1")
	if !errors.Is(err, ErrCodeExhausted) {
		t.Fatalf("encode error = %v; want %v", err, ErrCodeExhausted)
	}
	if store.insertCalls != maxInsertTries {
		t.Fatalf("insert calls = %d; want %d", store.insertCalls, maxInsertTries)
	}
}

func TestServiceRejectsInvalidOriginalURLs(t *testing.T) {
	service := newTestService(t, newMemoryStore(), "Ab3dE9xY")

	tests := []struct {
		name string
		url  string
	}{
		{name: "empty", url: ""},
		{name: "unsupported scheme", url: "ftp://example.com/file"},
		{name: "missing host", url: "https:///path"},
		{name: "over max length", url: "https://example.com/" + strings.Repeat("a", MaxURLLength)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := service.Encode(context.Background(), tt.url)
			if !errors.Is(err, ErrInvalidURL) {
				t.Fatalf("encode error = %v; want %v", err, ErrInvalidURL)
			}
		})
	}
}

func TestServiceRejectsInvalidShortURLs(t *testing.T) {
	service := newTestService(t, newMemoryStore(), "Ab3dE9xY")

	tests := []struct {
		name     string
		shortURL string
	}{
		{name: "wrong scheme", shortURL: "https://localhost:8080/Ab3dE9xY"},
		{name: "wrong host", shortURL: "http://example.com/Ab3dE9xY"},
		{name: "multi segment path", shortURL: "http://localhost:8080/Ab3dE9xY/extra"},
		{name: "query", shortURL: "http://localhost:8080/Ab3dE9xY?x=1"},
		{name: "fragment", shortURL: "http://localhost:8080/Ab3dE9xY#section"},
		{name: "malformed code", shortURL: "http://localhost:8080/not-ok!!"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := service.Decode(context.Background(), tt.shortURL)
			if !errors.Is(err, ErrInvalidShortURL) {
				t.Fatalf("decode error = %v; want %v", err, ErrInvalidShortURL)
			}
		})
	}
}

func TestServiceReturnsNotFoundForUnknownCode(t *testing.T) {
	service := newTestService(t, newMemoryStore(), "Ab3dE9xY")

	_, err := service.Decode(context.Background(), "http://localhost:8080/Ab3dE9xY")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("decode error = %v; want %v", err, ErrNotFound)
	}
}

func newTestService(t *testing.T, store Store, codes ...string) *Service {
	t.Helper()

	service, err := NewService(store, "http://localhost:8080")
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	codeIndex := 0
	service.generateCode = func() (string, error) {
		if codeIndex >= len(codes) {
			t.Fatalf("unexpected code generation call %d", codeIndex+1)
		}

		code := codes[codeIndex]
		codeIndex++
		return code, nil
	}

	return service
}

type memoryStore struct {
	linksByCode map[string]Link
	codesByURL  map[string]string
	insertCalls int
}

func newMemoryStore() *memoryStore {
	return &memoryStore{
		linksByCode: make(map[string]Link),
		codesByURL:  make(map[string]string),
	}
}

func (s *memoryStore) FindByOriginalURL(_ context.Context, originalURL string) (Link, error) {
	code, ok := s.codesByURL[originalURL]
	if !ok {
		return Link{}, ErrNotFound
	}

	return s.linksByCode[code], nil
}

func (s *memoryStore) FindByCode(_ context.Context, code string) (Link, error) {
	link, ok := s.linksByCode[code]
	if !ok {
		return Link{}, ErrNotFound
	}

	return link, nil
}

func (s *memoryStore) Insert(_ context.Context, link Link) (bool, error) {
	s.insertCalls++

	if _, exists := s.linksByCode[link.Code]; exists {
		return false, nil
	}
	if _, exists := s.codesByURL[link.OriginalURL]; exists {
		return false, nil
	}

	s.linksByCode[link.Code] = link
	s.codesByURL[link.OriginalURL] = link.Code
	return true, nil
}

func (s *memoryStore) mustInsert(t *testing.T, link Link) {
	t.Helper()

	inserted, err := s.Insert(context.Background(), link)
	if err != nil {
		t.Fatalf("insert fixture: %v", err)
	}
	if !inserted {
		t.Fatal("fixture was not inserted")
	}
	s.insertCalls = 0
}

type concurrentOriginalURLStore struct {
	existing  Link
	findCalls int
}

func (s *concurrentOriginalURLStore) FindByOriginalURL(_ context.Context, originalURL string) (Link, error) {
	s.findCalls++
	if s.findCalls == 1 {
		return Link{}, ErrNotFound
	}
	if originalURL != s.existing.OriginalURL {
		return Link{}, ErrNotFound
	}

	return s.existing, nil
}

func (s *concurrentOriginalURLStore) FindByCode(_ context.Context, code string) (Link, error) {
	if code != s.existing.Code {
		return Link{}, ErrNotFound
	}

	return s.existing, nil
}

func (s *concurrentOriginalURLStore) Insert(context.Context, Link) (bool, error) {
	return false, nil
}
