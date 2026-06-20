package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"time"

	"go-url-shortener/internal/httpapi"
	"go-url-shortener/internal/shortener"
	"go-url-shortener/internal/storage"
)

const (
	defaultAddr    = ":8080"
	defaultBaseURL = "http://localhost:8080"
	defaultDBPath  = "data/shortlink.db"
)

func main() {
	addr := envOrDefault("SHORTLINK_ADDR", defaultAddr)
	baseURL := envOrDefault("SHORTLINK_BASE_URL", defaultBaseURL)
	dbPath := envOrDefault("SHORTLINK_DB_PATH", defaultDBPath)

	store, err := storage.OpenSQLite(context.Background(), dbPath)
	if err != nil {
		log.Fatalf("open sqlite store: %v", err)
	}

	service, err := shortener.NewService(store, baseURL)
	if err != nil {
		if closeErr := store.Close(); closeErr != nil {
			log.Printf("close sqlite store after service creation failure: %v", closeErr)
		}
		log.Fatalf("create shortener service: %v", err)
	}
	defer func() {
		if err := store.Close(); err != nil {
			log.Printf("close sqlite store: %v", err)
		}
	}()

	server := &http.Server{
		Addr:              addr,
		Handler:           httpapi.NewHandler(service),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	log.Printf("starting shortlink server addr=%s base_url=%s db_path=%s", addr, baseURL, dbPath)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatalf("listen and serve: %v", err)
	}
}

func envOrDefault(key string, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}

	return value
}
