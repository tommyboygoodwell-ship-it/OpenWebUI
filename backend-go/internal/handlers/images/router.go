package images

import (
	"net/http"

	"github.com/open-webui/open-webui/backend-go/internal/handlers/common"
)

func RegisterRoutes(mux *http.ServeMux, prefix string) {
	common.RegisterPrefix(mux, prefix, common.NotImplemented("images"))
}
