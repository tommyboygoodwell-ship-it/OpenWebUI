package middleware

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/rand"
	"encoding/hex"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/open-webui/open-webui/backend-go/internal/config"
)

type contextKey string

const (
	configKey  contextKey = "owui-config"
	sessionKey contextKey = "owui-session"
)

// ConfigFromContext extracts the shared configuration from a request context.
func ConfigFromContext(r *http.Request) *config.Config {
	if cfg, ok := r.Context().Value(configKey).(*config.Config); ok {
		return cfg
	}
	return nil
}

// Session carries the generated session identifier.
type Session struct {
	ID string
}

// SessionFromContext returns the active session metadata if any.
func SessionFromContext(r *http.Request) *Session {
	if sess, ok := r.Context().Value(sessionKey).(*Session); ok {
		return sess
	}
	return nil
}

// Chain applies the provided middlewares to an http.Handler.
func Chain(h http.Handler, middlewares ...func(http.Handler) http.Handler) http.Handler {
	for i := len(middlewares) - 1; i >= 0; i-- {
		h = middlewares[i](h)
	}
	return h
}

// ConfigInjector injects the configuration into every request context.
func ConfigInjector(cfg *config.Config) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), configKey, cfg)))
		})
	}
}

// ProcessTime appends the X-Process-Time header.
func ProcessTime() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			next.ServeHTTP(w, r)
			duration := time.Since(start)
			w.Header().Set("X-Process-Time", duration.String())
		})
	}
}

// CORS emits permissive headers similar to the FastAPI configuration.
func CORS(cfg *config.Config) func(http.Handler) http.Handler {
	allowAll := len(cfg.CORSAllowOrigins) == 1 && cfg.CORSAllowOrigins[0] == "*"
	allowed := make(map[string]struct{}, len(cfg.CORSAllowOrigins))
	if !allowAll {
		for _, origin := range cfg.CORSAllowOrigins {
			allowed[origin] = struct{}{}
		}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			switch {
			case allowAll || origin == "":
				w.Header().Set("Access-Control-Allow-Origin", "*")
			case origin != "":
				if _, ok := allowed[origin]; ok {
					w.Header().Set("Access-Control-Allow-Origin", origin)
				}
			}
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, X-Requested-With")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Expose-Headers", "Content-Disposition")

			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// Compression applies gzip compression when the client supports it.
func Compression() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
				next.ServeHTTP(w, r)
				return
			}

			gz := gzip.NewWriter(w)
			defer gz.Close()

			w.Header().Set("Content-Encoding", "gzip")
			next.ServeHTTP(&gzipResponseWriter{ResponseWriter: w, Writer: gz}, r)
		})
	}
}

type gzipResponseWriter struct {
	http.ResponseWriter
	io.Writer
}

func (g *gzipResponseWriter) Write(b []byte) (int, error) {
	return g.Writer.Write(b)
}

// SessionManager manages lightweight in-memory session ids.
type SessionManager struct {
	mu   sync.RWMutex
	data map[string]struct{}
}

func NewSessionManager() *SessionManager {
	return &SessionManager{data: make(map[string]struct{})}
}

// SessionMiddleware ensures a session cookie exists and stores the metadata on the request context.
func SessionMiddleware(cfg *config.Config, manager *SessionManager) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cookie, err := r.Cookie(cfg.SessionCookieName)
			var sessionID string
			if err == nil {
				sessionID = cookie.Value
			}

			if sessionID == "" {
				sessionID = randomID()
				http.SetCookie(w, &http.Cookie{
					Name:     cfg.SessionCookieName,
					Value:    sessionID,
					Path:     "/",
					HttpOnly: true,
					Secure:   cfg.SessionSecure,
					SameSite: sameSite(cfg.SessionSameSite),
					MaxAge:   cfg.SessionMaxAgeSeconds,
				})
			}

			manager.mu.Lock()
			manager.data[sessionID] = struct{}{}
			manager.mu.Unlock()

			ctx := context.WithValue(r.Context(), sessionKey, &Session{ID: sessionID})
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// AuditLogger logs request summaries according to the configured level.
func AuditLogger(cfg *config.Config) func(http.Handler) http.Handler {
	level := strings.ToUpper(cfg.AuditLevel)
	if level == "NONE" {
		return func(next http.Handler) http.Handler { return next }
	}

	excluded := make([]string, 0, len(cfg.AuditExcludedPaths))
	for _, p := range cfg.AuditExcludedPaths {
		trimmed := strings.Trim(strings.TrimSpace(p), "/")
		if trimmed != "" {
			excluded = append(excluded, trimmed)
		}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			path := strings.Trim(strings.TrimPrefix(r.URL.Path, "/"), "")
			for _, ex := range excluded {
				if strings.HasPrefix(path, ex) {
					next.ServeHTTP(w, r)
					return
				}
			}

			var bodyPreview string
			if r.Body != nil {
				payload, err := io.ReadAll(r.Body)
				if err == nil {
					if len(payload) > cfg.MaxBodyLogSize {
						bodyPreview = string(payload[:cfg.MaxBodyLogSize])
					} else {
						bodyPreview = string(payload)
					}
					r.Body = io.NopCloser(bytes.NewBuffer(payload))
				}
			}

			start := time.Now()
			rr := &responseRecorder{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(rr, r)
			duration := time.Since(start)

			log.Printf("AUDIT level=%s method=%s path=%s status=%d duration=%s body=%q", level, r.Method, r.URL.Path, rr.status, duration, bodyPreview)
		})
	}
}

type responseRecorder struct {
	http.ResponseWriter
	status int
}

func (r *responseRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

func randomID() string {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return strconv.FormatInt(time.Now().UnixNano(), 16)
	}
	return hex.EncodeToString(buf)
}

func sameSite(mode string) http.SameSite {
	switch strings.ToLower(mode) {
	case "none":
		return http.SameSiteNoneMode
	case "strict":
		return http.SameSiteStrictMode
	default:
		return http.SameSiteLaxMode
	}
}
