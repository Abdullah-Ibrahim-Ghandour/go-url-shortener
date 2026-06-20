package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"go-url-shortener/internal/shortener"
)

func TestEncodeReturnsShortURL(t *testing.T) {
	handler := NewHandler(fakeShortener{
		encode: func(_ context.Context, rawURL string) (string, error) {
			if rawURL != "https://example.com/articles/1" {
				t.Fatalf("raw URL = %q; want %q", rawURL, "https://example.com/articles/1")
			}
			return "http://localhost:8080/Ab3dE9xY", nil
		},
	})

	response := serve(handler, http.MethodPost, "/encode", `{"url":"https://example.com/articles/1"}`)

	assertStatus(t, response, http.StatusOK)
	assertContentType(t, response)
	assertJSON(t, response, map[string]string{"short_url": "http://localhost:8080/Ab3dE9xY"})
}

func TestDecodeReturnsOriginalURL(t *testing.T) {
	handler := NewHandler(fakeShortener{
		decode: func(_ context.Context, rawShortURL string) (string, error) {
			if rawShortURL != "http://localhost:8080/Ab3dE9xY" {
				t.Fatalf("short URL = %q; want %q", rawShortURL, "http://localhost:8080/Ab3dE9xY")
			}
			return "https://example.com/articles/1", nil
		},
	})

	response := serve(handler, http.MethodPost, "/decode", `{"short_url":"http://localhost:8080/Ab3dE9xY"}`)

	assertStatus(t, response, http.StatusOK)
	assertContentType(t, response)
	assertJSON(t, response, map[string]string{"url": "https://example.com/articles/1"})
}

func TestRejectsWrongMethod(t *testing.T) {
	handler := NewHandler(fakeShortener{})

	response := serve(handler, http.MethodGet, "/encode", "")

	assertStatus(t, response, http.StatusMethodNotAllowed)
	assertContentType(t, response)
	assertJSON(t, response, map[string]string{"error": "method_not_allowed"})
	if response.Header().Get("Allow") != http.MethodPost {
		t.Fatalf("allow header = %q; want %q", response.Header().Get("Allow"), http.MethodPost)
	}
}

func TestRejectsInvalidRequests(t *testing.T) {
	handler := NewHandler(fakeShortener{})

	tests := []struct {
		name string
		path string
		body string
	}{
		{name: "encode malformed JSON", path: "/encode", body: `{"url":`},
		{name: "encode unknown field", path: "/encode", body: `{"url":"https://example.com","extra":true}`},
		{name: "encode trailing JSON token", path: "/encode", body: `{"url":"https://example.com"} {"url":"https://example.org"}`},
		{name: "encode missing URL", path: "/encode", body: `{}`},
		{name: "decode malformed JSON", path: "/decode", body: `{"short_url":`},
		{name: "decode unknown field", path: "/decode", body: `{"short_url":"http://localhost:8080/Ab3dE9xY","extra":true}`},
		{name: "decode trailing JSON token", path: "/decode", body: `{"short_url":"http://localhost:8080/Ab3dE9xY"} {}`},
		{name: "decode missing short URL", path: "/decode", body: `{}`},
		{name: "request body too large", path: "/encode", body: strings.Repeat(" ", maxRequestBodyBytes+1)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			response := serve(handler, http.MethodPost, tt.path, tt.body)

			assertStatus(t, response, http.StatusBadRequest)
			assertContentType(t, response)
			assertJSON(t, response, map[string]string{"error": "invalid_request"})
		})
	}
}

func TestMapsServiceErrors(t *testing.T) {
	tests := []struct {
		name       string
		path       string
		body       string
		service    fakeShortener
		wantStatus int
		wantError  string
	}{
		{
			name: "invalid original URL",
			path: "/encode",
			body: `{"url":"ftp://example.com/file"}`,
			service: fakeShortener{
				encode: func(context.Context, string) (string, error) {
					return "", shortener.ErrInvalidURL
				},
			},
			wantStatus: http.StatusBadRequest,
			wantError:  "invalid_url",
		},
		{
			name: "invalid short URL",
			path: "/decode",
			body: `{"short_url":"http://example.com/Ab3dE9xY"}`,
			service: fakeShortener{
				decode: func(context.Context, string) (string, error) {
					return "", shortener.ErrInvalidShortURL
				},
			},
			wantStatus: http.StatusBadRequest,
			wantError:  "invalid_short_url",
		},
		{
			name: "unknown code",
			path: "/decode",
			body: `{"short_url":"http://localhost:8080/Ab3dE9xY"}`,
			service: fakeShortener{
				decode: func(context.Context, string) (string, error) {
					return "", shortener.ErrNotFound
				},
			},
			wantStatus: http.StatusNotFound,
			wantError:  "not_found",
		},
		{
			name: "internal encode error",
			path: "/encode",
			body: `{"url":"https://example.com"}`,
			service: fakeShortener{
				encode: func(context.Context, string) (string, error) {
					return "", errors.New("database offline")
				},
			},
			wantStatus: http.StatusInternalServerError,
			wantError:  "internal_error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := NewHandler(tt.service)
			response := serve(handler, http.MethodPost, tt.path, tt.body)

			assertStatus(t, response, tt.wantStatus)
			assertContentType(t, response)
			assertJSON(t, response, map[string]string{"error": tt.wantError})
		})
	}
}

type fakeShortener struct {
	encode func(ctx context.Context, rawURL string) (string, error)
	decode func(ctx context.Context, rawShortURL string) (string, error)
}

func (s fakeShortener) Encode(ctx context.Context, rawURL string) (string, error) {
	if s.encode == nil {
		return "", errors.New("unexpected encode call")
	}

	return s.encode(ctx, rawURL)
}

func (s fakeShortener) Decode(ctx context.Context, rawShortURL string) (string, error) {
	if s.decode == nil {
		return "", errors.New("unexpected decode call")
	}

	return s.decode(ctx, rawShortURL)
}

func serve(handler http.Handler, method string, path string, body string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)
	return response
}

func assertStatus(t *testing.T, response *httptest.ResponseRecorder, want int) {
	t.Helper()

	if response.Code != want {
		t.Fatalf("status = %d; want %d; body = %s", response.Code, want, response.Body.String())
	}
}

func assertContentType(t *testing.T, response *httptest.ResponseRecorder) {
	t.Helper()

	if got := response.Header().Get("Content-Type"); got != "application/json; charset=utf-8" {
		t.Fatalf("content type = %q; want %q", got, "application/json; charset=utf-8")
	}
}

func assertJSON(t *testing.T, response *httptest.ResponseRecorder, want map[string]string) {
	t.Helper()

	var got map[string]string
	if err := json.NewDecoder(response.Body).Decode(&got); err != nil {
		t.Fatalf("decode response JSON: %v", err)
	}

	if len(got) != len(want) {
		t.Fatalf("response JSON = %#v; want %#v", got, want)
	}
	for key, wantValue := range want {
		if got[key] != wantValue {
			t.Fatalf("response JSON[%q] = %q; want %q", key, got[key], wantValue)
		}
	}
}
