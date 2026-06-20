package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Abdullah-Ibrahim-Ghandour/go-url-shortener/internal/shortener"
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

func TestDecodeEndpointReturnsOriginalURL(t *testing.T) {
	handler := NewHandler(fakeShortener{
		resolveShortURL: func(_ context.Context, rawShortURL string) (string, error) {
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

func TestRedirectReturnsFound(t *testing.T) {
	handler := NewHandler(fakeShortener{
		resolveCode: func(_ context.Context, code string) (string, error) {
			if code != "Ab3dE9xY" {
				t.Fatalf("code = %q; want %q", code, "Ab3dE9xY")
			}
			return "https://example.com/articles/1", nil
		},
	})

	response := serve(handler, http.MethodGet, "/Ab3dE9xY", "")

	assertStatus(t, response, http.StatusFound)
	if response.Header().Get("Location") != "https://example.com/articles/1" {
		t.Fatalf("location = %q; want %q", response.Header().Get("Location"), "https://example.com/articles/1")
	}
}

func TestHomePageReturnsHTML(t *testing.T) {
	handler := NewHandler(fakeShortener{})

	response := serve(handler, http.MethodGet, "/", "")

	assertStatus(t, response, http.StatusOK)
	if got := response.Header().Get("Content-Type"); got != "text/html; charset=utf-8" {
		t.Fatalf("content type = %q; want %q", got, "text/html; charset=utf-8")
	}
	body := response.Body.String()
	if !strings.Contains(body, "Go URL Shortener") {
		t.Fatalf("homepage body does not contain app title")
	}
	if !strings.Contains(body, "/encode") || !strings.Contains(body, "/decode") {
		t.Fatalf("homepage body does not contain endpoint usage")
	}
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

func TestRedirectRejectsWrongMethod(t *testing.T) {
	handler := NewHandler(fakeShortener{})

	response := serve(handler, http.MethodPost, "/Ab3dE9xY", "")

	assertStatus(t, response, http.StatusMethodNotAllowed)
	assertContentType(t, response)
	assertJSON(t, response, map[string]string{"error": "method_not_allowed"})
	if response.Header().Get("Allow") != http.MethodGet {
		t.Fatalf("allow header = %q; want %q", response.Header().Get("Allow"), http.MethodGet)
	}
}

func TestRedirectRejectsInvalidPaths(t *testing.T) {
	handler := NewHandler(fakeShortener{})

	tests := []struct {
		name string
		path string
	}{
		{name: "nested path", path: "/Ab3dE9xY/extra"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			response := serve(handler, http.MethodGet, tt.path, "")

			assertStatus(t, response, http.StatusNotFound)
			assertContentType(t, response)
			assertJSON(t, response, map[string]string{"error": "not_found"})
		})
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
		method     string
		path       string
		body       string
		service    fakeShortener
		wantStatus int
		wantError  string
	}{
		{
			name:   "invalid original URL",
			method: http.MethodPost,
			path:   "/encode",
			body:   `{"url":"ftp://example.com/file"}`,
			service: fakeShortener{
				encode: func(context.Context, string) (string, error) {
					return "", shortener.ErrInvalidURL
				},
			},
			wantStatus: http.StatusBadRequest,
			wantError:  "invalid_url",
		},
		{
			name:   "invalid short URL",
			method: http.MethodPost,
			path:   "/decode",
			body:   `{"short_url":"http://example.com/Ab3dE9xY"}`,
			service: fakeShortener{
				resolveShortURL: func(context.Context, string) (string, error) {
					return "", shortener.ErrInvalidShortURL
				},
			},
			wantStatus: http.StatusBadRequest,
			wantError:  "invalid_short_url",
		},
		{
			name:   "unknown code",
			method: http.MethodPost,
			path:   "/decode",
			body:   `{"short_url":"http://localhost:8080/Ab3dE9xY"}`,
			service: fakeShortener{
				resolveShortURL: func(context.Context, string) (string, error) {
					return "", shortener.ErrNotFound
				},
			},
			wantStatus: http.StatusNotFound,
			wantError:  "not_found",
		},
		{
			name:   "internal encode error",
			method: http.MethodPost,
			path:   "/encode",
			body:   `{"url":"https://example.com"}`,
			service: fakeShortener{
				encode: func(context.Context, string) (string, error) {
					return "", errors.New("database offline")
				},
			},
			wantStatus: http.StatusInternalServerError,
			wantError:  "internal_error",
		},
		{
			name:   "redirect invalid code",
			method: http.MethodGet,
			path:   "/not-ok!!",
			body:   "",
			service: fakeShortener{
				resolveCode: func(context.Context, string) (string, error) {
					return "", shortener.ErrInvalidShortURL
				},
			},
			wantStatus: http.StatusBadRequest,
			wantError:  "invalid_short_url",
		},
		{
			name:   "redirect unknown code",
			method: http.MethodGet,
			path:   "/Ab3dE9xY",
			body:   "",
			service: fakeShortener{
				resolveCode: func(context.Context, string) (string, error) {
					return "", shortener.ErrNotFound
				},
			},
			wantStatus: http.StatusNotFound,
			wantError:  "not_found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := NewHandler(tt.service)
			response := serve(handler, tt.method, tt.path, tt.body)

			assertStatus(t, response, tt.wantStatus)
			assertContentType(t, response)
			assertJSON(t, response, map[string]string{"error": tt.wantError})
		})
	}
}

type fakeShortener struct {
	encode          func(ctx context.Context, rawURL string) (string, error)
	resolveShortURL func(ctx context.Context, rawShortURL string) (string, error)
	resolveCode     func(ctx context.Context, code string) (string, error)
}

func (s fakeShortener) Encode(ctx context.Context, rawURL string) (string, error) {
	if s.encode == nil {
		return "", errors.New("unexpected encode call")
	}

	return s.encode(ctx, rawURL)
}

func (s fakeShortener) ResolveShortURL(ctx context.Context, rawShortURL string) (string, error) {
	if s.resolveShortURL == nil {
		return "", errors.New("unexpected resolve short URL call")
	}

	return s.resolveShortURL(ctx, rawShortURL)
}

func (s fakeShortener) ResolveCode(ctx context.Context, code string) (string, error) {
	if s.resolveCode == nil {
		return "", errors.New("unexpected resolve code call")
	}

	return s.resolveCode(ctx, code)
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
