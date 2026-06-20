package shortener

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"net/url"
	"strings"
	"time"
)

const (
	CodeLength     = 8
	MaxURLLength   = 2048
	maxInsertTries = 5
	base62Alphabet = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
)

var (
	ErrNotFound        = errors.New("not found")
	ErrInvalidURL      = errors.New("invalid url")
	ErrInvalidShortURL = errors.New("invalid short url")
	ErrCodeExhausted   = errors.New("code space exhausted")
)

type Link struct {
	Code        string
	OriginalURL string
	CreatedAt   time.Time
}

type Store interface {
	FindByOriginalURL(ctx context.Context, originalURL string) (Link, error)
	FindByCode(ctx context.Context, code string) (Link, error)
	Insert(ctx context.Context, link Link) (bool, error)
}

type Service struct {
	store        Store
	baseURL      url.URL
	generateCode func() (string, error)
}

func NewService(store Store, baseURL string) (*Service, error) {
	if store == nil {
		return nil, fmt.Errorf("store is required")
	}

	parsed, err := parseBaseURL(baseURL)
	if err != nil {
		return nil, err
	}

	return &Service{
		store:        store,
		baseURL:      parsed,
		generateCode: randomBase62Code,
	}, nil
}

func (s *Service) Encode(ctx context.Context, rawURL string) (string, error) {
	originalURL, err := normalizeOriginalURL(rawURL)
	if err != nil {
		return "", err
	}

	existing, err := s.store.FindByOriginalURL(ctx, originalURL)
	if err == nil {
		return s.shortURL(existing.Code), nil
	}
	if !errors.Is(err, ErrNotFound) {
		return "", fmt.Errorf("find existing URL: %w", err)
	}

	for range maxInsertTries {
		code, err := s.generateCode()
		if err != nil {
			return "", fmt.Errorf("generate code: %w", err)
		}

		inserted, err := s.store.Insert(ctx, Link{
			Code:        code,
			OriginalURL: originalURL,
			CreatedAt:   time.Now().UTC(),
		})
		if err != nil {
			return "", fmt.Errorf("insert link: %w", err)
		}
		if inserted {
			return s.shortURL(code), nil
		}

		existing, err := s.store.FindByOriginalURL(ctx, originalURL)
		if err == nil {
			return s.shortURL(existing.Code), nil
		}
		if !errors.Is(err, ErrNotFound) {
			return "", fmt.Errorf("find existing URL after insert miss: %w", err)
		}
	}

	return "", ErrCodeExhausted
}

func (s *Service) Decode(ctx context.Context, rawShortURL string) (string, error) {
	code, err := s.codeFromShortURL(rawShortURL)
	if err != nil {
		return "", err
	}

	link, err := s.store.FindByCode(ctx, code)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return "", ErrNotFound
		}
		return "", fmt.Errorf("find by code: %w", err)
	}

	return link.OriginalURL, nil
}

func (s *Service) shortURL(code string) string {
	shortURL := s.baseURL
	shortURL.Path = "/" + code
	shortURL.RawPath = ""
	shortURL.RawQuery = ""
	shortURL.Fragment = ""

	return shortURL.String()
}

func (s *Service) codeFromShortURL(rawShortURL string) (string, error) {
	trimmed := strings.TrimSpace(rawShortURL)
	if trimmed == "" {
		return "", ErrInvalidShortURL
	}

	parsed, err := url.Parse(trimmed)
	if err != nil {
		return "", ErrInvalidShortURL
	}
	if !strings.EqualFold(parsed.Scheme, s.baseURL.Scheme) || !strings.EqualFold(parsed.Host, s.baseURL.Host) {
		return "", ErrInvalidShortURL
	}
	if parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", ErrInvalidShortURL
	}

	code := strings.TrimPrefix(parsed.Path, "/")
	if parsed.Path == "" || !strings.HasPrefix(parsed.Path, "/") || strings.Contains(code, "/") || !isValidCode(code) {
		return "", ErrInvalidShortURL
	}

	return code, nil
}

func normalizeOriginalURL(rawURL string) (string, error) {
	trimmed := strings.TrimSpace(rawURL)
	if trimmed == "" || len(trimmed) > MaxURLLength {
		return "", ErrInvalidURL
	}

	parsed, err := url.Parse(trimmed)
	if err != nil {
		return "", ErrInvalidURL
	}
	if parsed.Host == "" {
		return "", ErrInvalidURL
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", ErrInvalidURL
	}

	return trimmed, nil
}

func parseBaseURL(rawBaseURL string) (url.URL, error) {
	parsed, err := url.Parse(strings.TrimSpace(rawBaseURL))
	if err != nil {
		return url.URL{}, fmt.Errorf("invalid base URL: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return url.URL{}, fmt.Errorf("invalid base URL: scheme must be http or https")
	}
	if parsed.Host == "" {
		return url.URL{}, fmt.Errorf("invalid base URL: host is required")
	}
	if parsed.Path != "" && parsed.Path != "/" {
		return url.URL{}, fmt.Errorf("invalid base URL: path must be empty")
	}
	if parsed.RawQuery != "" || parsed.Fragment != "" {
		return url.URL{}, fmt.Errorf("invalid base URL: query and fragment are not allowed")
	}

	parsed.Path = ""
	return *parsed, nil
}

func randomBase62Code() (string, error) {
	code := make([]byte, CodeLength)

	limit := big.NewInt(int64(len(base62Alphabet)))
	for i := range code {
		index, err := rand.Int(rand.Reader, limit)
		if err != nil {
			return "", err
		}
		code[i] = base62Alphabet[index.Int64()]
	}

	return string(code), nil
}

func isValidCode(code string) bool {
	if len(code) != CodeLength {
		return false
	}

	for i := range code {
		if !strings.ContainsRune(base62Alphabet, rune(code[i])) {
			return false
		}
	}

	return true
}
