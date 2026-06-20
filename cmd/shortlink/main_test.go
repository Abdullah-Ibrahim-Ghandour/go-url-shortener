package main

import (
	"path/filepath"
	"testing"
)

func TestLoadConfigUsesLocalDefaults(t *testing.T) {
	config := loadConfig(func(string) string { return "" })

	if config.addr != defaultAddr {
		t.Fatalf("addr = %q; want %q", config.addr, defaultAddr)
	}
	if config.baseURL != defaultBaseURL {
		t.Fatalf("base URL = %q; want %q", config.baseURL, defaultBaseURL)
	}
	if config.dbPath != defaultDBPath {
		t.Fatalf("db path = %q; want %q", config.dbPath, defaultDBPath)
	}
}

func TestLoadConfigUsesExplicitShortlinkEnv(t *testing.T) {
	values := map[string]string{
		"SHORTLINK_ADDR":     ":9090",
		"SHORTLINK_BASE_URL": "https://short.example",
		"SHORTLINK_DB_PATH":  "/var/data/links.db",
	}

	config := loadConfig(func(key string) string { return values[key] })

	if config.addr != ":9090" {
		t.Fatalf("addr = %q; want %q", config.addr, ":9090")
	}
	if config.baseURL != "https://short.example" {
		t.Fatalf("base URL = %q; want %q", config.baseURL, "https://short.example")
	}
	if config.dbPath != "/var/data/links.db" {
		t.Fatalf("db path = %q; want %q", config.dbPath, "/var/data/links.db")
	}
}

func TestLoadConfigUsesPlatformFallbacks(t *testing.T) {
	values := map[string]string{
		"PORT":                      "12345",
		"RAILWAY_PUBLIC_DOMAIN":     "go-url-shortener.up.railway.app",
		"RAILWAY_VOLUME_MOUNT_PATH": "/data",
	}

	config := loadConfig(func(key string) string { return values[key] })

	if config.addr != ":12345" {
		t.Fatalf("addr = %q; want %q", config.addr, ":12345")
	}
	if config.baseURL != "https://go-url-shortener.up.railway.app" {
		t.Fatalf("base URL = %q; want %q", config.baseURL, "https://go-url-shortener.up.railway.app")
	}
	wantDBPath := filepath.Join("/data", "shortlink.db")
	if config.dbPath != wantDBPath {
		t.Fatalf("db path = %q; want %q", config.dbPath, wantDBPath)
	}
}

func TestLoadConfigPrefersExplicitEnvOverPlatformFallbacks(t *testing.T) {
	values := map[string]string{
		"SHORTLINK_ADDR":            ":9090",
		"SHORTLINK_BASE_URL":        "https://short.example",
		"SHORTLINK_DB_PATH":         "/var/data/links.db",
		"PORT":                      "12345",
		"RAILWAY_PUBLIC_DOMAIN":     "go-url-shortener.up.railway.app",
		"RAILWAY_VOLUME_MOUNT_PATH": "/data",
	}

	config := loadConfig(func(key string) string { return values[key] })

	if config.addr != ":9090" {
		t.Fatalf("addr = %q; want %q", config.addr, ":9090")
	}
	if config.baseURL != "https://short.example" {
		t.Fatalf("base URL = %q; want %q", config.baseURL, "https://short.example")
	}
	if config.dbPath != "/var/data/links.db" {
		t.Fatalf("db path = %q; want %q", config.dbPath, "/var/data/links.db")
	}
}
