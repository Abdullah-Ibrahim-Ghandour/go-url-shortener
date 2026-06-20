package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"path/filepath"
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
	config := loadConfig(os.Getenv)

	store, err := storage.OpenSQLite(context.Background(), config.dbPath)
	if err != nil {
		log.Fatalf("open sqlite store: %v", err)
	}

	service, err := shortener.NewService(store, config.baseURL)
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
		Addr:              config.addr,
		Handler:           httpapi.NewHandler(service),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	log.Printf("starting shortlink server addr=%s base_url=%s db_path=%s", config.addr, config.baseURL, config.dbPath)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatalf("listen and serve: %v", err)
	}
}

type config struct {
	addr    string
	baseURL string
	dbPath  string
}

func loadConfig(getenv func(string) string) config {
	addr := getenv("SHORTLINK_ADDR")
	if addr == "" {
		if port := getenv("PORT"); port != "" {
			addr = ":" + port
		} else {
			addr = defaultAddr
		}
	}

	baseURL := getenv("SHORTLINK_BASE_URL")
	if baseURL == "" {
		if publicDomain := getenv("RAILWAY_PUBLIC_DOMAIN"); publicDomain != "" {
			baseURL = "https://" + publicDomain
		} else {
			baseURL = defaultBaseURL
		}
	}

	dbPath := getenv("SHORTLINK_DB_PATH")
	if dbPath == "" {
		if volumeMountPath := getenv("RAILWAY_VOLUME_MOUNT_PATH"); volumeMountPath != "" {
			dbPath = filepath.Join(volumeMountPath, "shortlink.db")
		} else {
			dbPath = defaultDBPath
		}
	}

	return config{
		addr:    addr,
		baseURL: baseURL,
		dbPath:  dbPath,
	}
}
