package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"go-url-shortener/internal/shortener"
)

const maxRequestBodyBytes = 16 * 1024

type Shortener interface {
	Encode(ctx context.Context, rawURL string) (string, error)
	Decode(ctx context.Context, rawShortURL string) (string, error)
}

type Handler struct {
	shortener Shortener
}

func NewHandler(shortener Shortener) http.Handler {
	handler := &Handler{shortener: shortener}

	mux := http.NewServeMux()
	mux.HandleFunc("/encode", handler.encode)
	mux.HandleFunc("/decode", handler.decode)

	return mux
}

func (h *Handler) encode(w http.ResponseWriter, r *http.Request) {
	if !requirePost(w, r) {
		return
	}

	var request struct {
		URL *string `json:"url"`
	}
	if err := decodeJSON(w, r, &request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request")
		return
	}
	if request.URL == nil {
		writeError(w, http.StatusBadRequest, "invalid_request")
		return
	}

	shortURL, err := h.shortener.Encode(r.Context(), *request.URL)
	if err != nil {
		status, code := errorResponse(err)
		writeError(w, status, code)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"short_url": shortURL})
}

func (h *Handler) decode(w http.ResponseWriter, r *http.Request) {
	if !requirePost(w, r) {
		return
	}

	var request struct {
		ShortURL *string `json:"short_url"`
	}
	if err := decodeJSON(w, r, &request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request")
		return
	}
	if request.ShortURL == nil {
		writeError(w, http.StatusBadRequest, "invalid_request")
		return
	}

	originalURL, err := h.shortener.Decode(r.Context(), *request.ShortURL)
	if err != nil {
		status, code := errorResponse(err)
		writeError(w, status, code)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"url": originalURL})
}

func requirePost(w http.ResponseWriter, r *http.Request) bool {
	if r.Method == http.MethodPost {
		return true
	}

	w.Header().Set("Allow", http.MethodPost)
	writeError(w, http.StatusMethodNotAllowed, "method_not_allowed")
	return false
}

func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("request body must contain one JSON value")
	}

	return nil
}

func errorResponse(err error) (int, string) {
	switch {
	case errors.Is(err, shortener.ErrInvalidURL):
		return http.StatusBadRequest, "invalid_url"
	case errors.Is(err, shortener.ErrInvalidShortURL):
		return http.StatusBadRequest, "invalid_short_url"
	case errors.Is(err, shortener.ErrNotFound):
		return http.StatusNotFound, "not_found"
	default:
		return http.StatusInternalServerError, "internal_error"
	}
}

func writeError(w http.ResponseWriter, status int, code string) {
	writeJSON(w, status, map[string]string{"error": code})
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
