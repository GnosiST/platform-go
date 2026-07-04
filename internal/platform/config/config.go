package config

import (
	"os"
	"strings"
)

type Config struct {
	HTTPAddr     string
	Capabilities []string
}

var defaultCapabilities = []string{
	"tenant",
	"identity",
	"session",
	"rbac",
	"menu",
	"api-resource",
	"audit",
	"dictionary",
	"parameter",
	"admin-shell",
	"system-admin",
}

func Load() Config {
	return Config{
		HTTPAddr:     env("PLATFORM_HTTP_ADDR", "127.0.0.1:9200"),
		Capabilities: csvEnv("PLATFORM_CAPABILITIES", defaultCapabilities),
	}
}

func env(key string, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}

func csvEnv(key string, fallback []string) []string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return append([]string(nil), fallback...)
	}
	parts := strings.Split(value, ",")
	items := make([]string, 0, len(parts))
	for _, part := range parts {
		item := strings.TrimSpace(part)
		if item != "" {
			items = append(items, item)
		}
	}
	return items
}
