package shortener

import (
	"context"
	"errors"
	"time"
)

var ErrNotFound = errors.New("not found")

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
