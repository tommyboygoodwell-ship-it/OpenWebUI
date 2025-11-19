package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Config captures the minimum set of knobs required by the compatibility backend.
type Config struct {
	Host                 string
	Port                 int
	AppName              string
	Version              string
	DefaultLocale        string
	CORSAllowOrigins     []string
	EnableCompression    bool
	SessionSecret        string
	SessionCookieName    string
	SessionSecure        bool
	SessionSameSite      string
	SessionMaxAgeSeconds int
	RedisURL             string
	AuditLevel           string
	AuditExcludedPaths   []string
	MaxBodyLogSize       int
	StaticDir            string
	SPABuildDir          string
	SwaggerShimMessage   string
}

// Load reads configuration from the environment and applies sane defaults.
func Load() (*Config, error) {
	cfg := &Config{
		Host:                 getEnv("WEBUI_HOST", "0.0.0.0"),
		Port:                 getEnvAsInt("WEBUI_PORT", 8080),
		AppName:              getEnv("WEBUI_NAME", "Open WebUI"),
		Version:              getEnv("WEBUI_VERSION", "dev"),
		DefaultLocale:        getEnv("DEFAULT_LOCALE", "en"),
		CORSAllowOrigins:     splitAndClean(os.Getenv("CORS_ALLOW_ORIGIN"), "*"),
		EnableCompression:    getEnvAsBool("ENABLE_COMPRESSION_MIDDLEWARE", true),
		SessionSecret:        getEnv("WEBUI_SECRET_KEY", "dev-secret-key"),
		SessionCookieName:    getEnv("WEBUI_SESSION_COOKIE_NAME", "owui-session"),
		SessionSecure:        getEnvAsBool("WEBUI_SESSION_COOKIE_SECURE", false),
		SessionSameSite:      strings.ToLower(getEnv("WEBUI_SESSION_COOKIE_SAME_SITE", "lax")),
		SessionMaxAgeSeconds: getEnvAsInt("WEBUI_SESSION_COOKIE_MAX_AGE", 60*60*24*7),
		RedisURL:             os.Getenv("REDIS_URL"),
		AuditLevel:           strings.ToUpper(getEnv("AUDIT_LOG_LEVEL", "NONE")),
		AuditExcludedPaths:   splitAndClean(os.Getenv("AUDIT_EXCLUDED_PATHS"), "chats,chat,folders"),
		MaxBodyLogSize:       getEnvAsInt("MAX_BODY_LOG_SIZE", 2048),
		StaticDir:            normalizePath(getEnv("STATIC_DIR", filepath.Join("..", "backend", "open_webui", "static"))),
		SPABuildDir:          normalizePath(getEnv("FRONTEND_BUILD_DIR", filepath.Join("..", "static"))),
		SwaggerShimMessage:   getEnv("SWAGGER_DISABLED_MESSAGE", "FastAPI docs are not available in the Go compatibility server."),
	}

	return cfg, nil
}

func (c *Config) Address() string {
	return fmt.Sprintf("%s:%d", c.Host, c.Port)
}

func splitAndClean(raw string, fallback string) []string {
	if strings.TrimSpace(raw) == "" {
		raw = fallback
	}

	sep := ";"
	if strings.Contains(raw, ",") && !strings.Contains(raw, ";") {
		sep = ","
	}

	parts := strings.Split(raw, sep)
	cleaned := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed == "" {
			continue
		}
		cleaned = append(cleaned, trimmed)
	}

	if len(cleaned) == 0 {
		cleaned = []string{fallback}
	}

	return cleaned
}

func normalizePath(path string) string {
	if path == "" {
		return ""
	}

	cleaned := filepath.Clean(path)
	if filepath.IsAbs(cleaned) {
		return cleaned
	}

	abs, err := filepath.Abs(cleaned)
	if err != nil {
		return cleaned
	}
	return abs
}

func getEnv(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func getEnvAsBool(key string, fallback bool) bool {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		parsed, err := strconv.ParseBool(value)
		if err == nil {
			return parsed
		}
	}
	return fallback
}

func getEnvAsInt(key string, fallback int) int {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		parsed, err := strconv.Atoi(value)
		if err == nil {
			return parsed
		}
	}
	return fallback
}
