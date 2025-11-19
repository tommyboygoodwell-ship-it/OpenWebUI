package common

import (
    "encoding/json"
    "net/http"
)

// Envelope is the default JSON envelope returned by compatibility handlers.
type Envelope struct {
    Status  bool        `json:"status"`
    Message string      `json:"message"`
    Data    interface{} `json:"data,omitempty"`
}

// NotImplemented responds with a FastAPI-like HTTPException payload.
func NotImplemented(resource string) http.Handler {
    detail := resource + " endpoint is not implemented in the Go backend yet"
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        writeJSON(w, http.StatusNotImplemented, map[string]any{
            "detail": detail,
            "status": http.StatusNotImplemented,
        })
    })
}

// ConfigHandler returns a lightweight /api/config payload.
func ConfigHandler(appName, version, locale string) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        writeJSON(w, http.StatusOK, map[string]any{
            "status":         true,
            "name":           appName,
            "version":        version,
            "default_locale": locale,
            "features":       map[string]any{},
        })
    })
}

// ModelsHandler mirrors the minimal JSON envelope expected by the UI.
func ModelsHandler() http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        writeJSON(w, http.StatusOK, map[string]any{
            "status": true,
            "data":   []any{},
        })
    })
}

// SwaggerShim returns a JSON notice for docs endpoints.
func SwaggerShim(message string) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        writeJSON(w, http.StatusOK, map[string]any{
            "status": false,
            "detail": message,
        })
    })
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(status)
    _ = json.NewEncoder(w).Encode(payload)
}
