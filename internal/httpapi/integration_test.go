package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"go-url-shortener/internal/shortener"
	"go-url-shortener/internal/storage"
)

func TestEncodeDecodeEndpointIntegrationPersistsAcrossRestart(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "links.db")
	originalURL := "https://codesubmit.io/library/react"
	baseURL := "http://localhost:8080"

	server, closeStore := startIntegrationServer(t, dbPath, baseURL)
	shortURL := encodeIntegrationURL(t, server.URL, originalURL)
	decodeIntegrationURL(t, server.URL, shortURL, originalURL)
	server.Close()
	closeStore()

	restarted, closeRestartedStore := startIntegrationServer(t, dbPath, baseURL)
	defer restarted.Close()
	defer closeRestartedStore()

	decodeIntegrationURL(t, restarted.URL, shortURL, originalURL)
}

func startIntegrationServer(t *testing.T, dbPath string, baseURL string) (*httptest.Server, func()) {
	t.Helper()

	store, err := storage.OpenSQLite(context.Background(), dbPath)
	if err != nil {
		t.Fatalf("open sqlite store: %v", err)
	}

	service, err := shortener.NewService(store, baseURL)
	if err != nil {
		if closeErr := store.Close(); closeErr != nil {
			t.Fatalf("create service: %v; close store: %v", err, closeErr)
		}
		t.Fatalf("create service: %v", err)
	}

	server := httptest.NewServer(NewHandler(service))

	return server, func() {
		if err := store.Close(); err != nil {
			t.Fatalf("close sqlite store: %v", err)
		}
	}
}

func encodeIntegrationURL(t *testing.T, baseURL string, originalURL string) string {
	t.Helper()

	body := map[string]string{"url": originalURL}
	var response struct {
		ShortURL string `json:"short_url"`
	}
	postIntegrationJSON(t, baseURL+"/encode", body, http.StatusOK, &response)

	if response.ShortURL == "" {
		t.Fatal("short URL is empty")
	}

	return response.ShortURL
}

func decodeIntegrationURL(t *testing.T, baseURL string, shortURL string, wantOriginalURL string) {
	t.Helper()

	body := map[string]string{"short_url": shortURL}
	var response struct {
		URL string `json:"url"`
	}
	postIntegrationJSON(t, baseURL+"/decode", body, http.StatusOK, &response)

	if response.URL != wantOriginalURL {
		t.Fatalf("decoded URL = %q; want %q", response.URL, wantOriginalURL)
	}
}

func postIntegrationJSON(t *testing.T, url string, body any, wantStatus int, dst any) {
	t.Helper()

	requestBody, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal request body: %v", err)
	}

	response, err := http.Post(url, "application/json", bytes.NewReader(requestBody))
	if err != nil {
		t.Fatalf("post %s: %v", url, err)
	}
	defer response.Body.Close()

	if response.StatusCode != wantStatus {
		t.Fatalf("status = %d; want %d", response.StatusCode, wantStatus)
	}

	if err := json.NewDecoder(response.Body).Decode(dst); err != nil {
		t.Fatalf("decode response body: %v", err)
	}
}
