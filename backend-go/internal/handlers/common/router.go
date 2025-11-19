package common

import (
	"net/http"
	"strings"
)

// RegisterPrefix wires a handler for both the exact prefix and any nested path.
func RegisterPrefix(mux *http.ServeMux, prefix string, handler http.Handler) {
	normalized := strings.TrimSuffix(prefix, "/")
	mux.Handle(normalized, handler)
	mux.Handle(normalized+"/", handler)
}
