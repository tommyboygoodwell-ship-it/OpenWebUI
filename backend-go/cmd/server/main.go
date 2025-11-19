package main

import (
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/open-webui/open-webui/backend-go/internal/config"
	"github.com/open-webui/open-webui/backend-go/internal/handlers/audio"
	"github.com/open-webui/open-webui/backend-go/internal/handlers/auths"
	"github.com/open-webui/open-webui/backend-go/internal/handlers/channels"
	"github.com/open-webui/open-webui/backend-go/internal/handlers/chats"
	"github.com/open-webui/open-webui/backend-go/internal/handlers/common"
	"github.com/open-webui/open-webui/backend-go/internal/handlers/configs"
	"github.com/open-webui/open-webui/backend-go/internal/handlers/credit"
	"github.com/open-webui/open-webui/backend-go/internal/handlers/evaluations"
	"github.com/open-webui/open-webui/backend-go/internal/handlers/files"
	"github.com/open-webui/open-webui/backend-go/internal/handlers/folders"
	"github.com/open-webui/open-webui/backend-go/internal/handlers/functions"
	"github.com/open-webui/open-webui/backend-go/internal/handlers/groups"
	"github.com/open-webui/open-webui/backend-go/internal/handlers/images"
	"github.com/open-webui/open-webui/backend-go/internal/handlers/knowledge"
	"github.com/open-webui/open-webui/backend-go/internal/handlers/memories"
	"github.com/open-webui/open-webui/backend-go/internal/handlers/models"
	"github.com/open-webui/open-webui/backend-go/internal/handlers/notes"
	"github.com/open-webui/open-webui/backend-go/internal/handlers/ollama"
	"github.com/open-webui/open-webui/backend-go/internal/handlers/openai"
	"github.com/open-webui/open-webui/backend-go/internal/handlers/pipelines"
	"github.com/open-webui/open-webui/backend-go/internal/handlers/prompts"
	"github.com/open-webui/open-webui/backend-go/internal/handlers/retrieval"
	"github.com/open-webui/open-webui/backend-go/internal/handlers/scim"
	"github.com/open-webui/open-webui/backend-go/internal/handlers/tasks"
	"github.com/open-webui/open-webui/backend-go/internal/handlers/tools"
	"github.com/open-webui/open-webui/backend-go/internal/handlers/users"
	"github.com/open-webui/open-webui/backend-go/internal/handlers/utils"
	"github.com/open-webui/open-webui/backend-go/internal/middleware"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	mux := http.NewServeMux()
	registerRoutes(mux, cfg)
	registerStatic(mux, cfg)

	sessionManager := middleware.NewSessionManager()
	handler := middleware.Chain(
		mux,
		middleware.AuditLogger(cfg),
		middleware.ProcessTime(),
		middleware.Compression(),
		middleware.CORS(cfg),
		middleware.SessionMiddleware(cfg, sessionManager),
		middleware.ConfigInjector(cfg),
	)

	server := &http.Server{
		Addr:    cfg.Address(),
		Handler: handler,
	}

	log.Printf("Open WebUI Go compatibility server listening on %s", cfg.Address())
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("server stopped: %v", err)
	}
}

func registerRoutes(mux *http.ServeMux, cfg *config.Config) {
	mux.Handle("/api/config", common.ConfigHandler(cfg.AppName, cfg.Version, cfg.DefaultLocale))
	mux.Handle("/api/models", common.ModelsHandler())
	mux.Handle("/api/v1/models", common.ModelsHandler())

	swagger := common.SwaggerShim(cfg.SwaggerShimMessage)
	mux.Handle("/docs", swagger)
	mux.Handle("/docs/", swagger)
	mux.Handle("/openapi.json", swagger)

	ollama.RegisterRoutes(mux, "/ollama")
	openai.RegisterRoutes(mux, "/openai")

	audio.RegisterRoutes(mux, "/api/v1/audio")
	images.RegisterRoutes(mux, "/api/v1/images")
	retrieval.RegisterRoutes(mux, "/api/v1/retrieval")
	pipelines.RegisterRoutes(mux, "/api/v1/pipelines")
	tasks.RegisterRoutes(mux, "/api/v1/tasks")
	configs.RegisterRoutes(mux, "/api/v1/configs")
	auths.RegisterRoutes(mux, "/api/v1/auths")
	users.RegisterRoutes(mux, "/api/v1/users")
	credit.RegisterRoutes(mux, "/api/v1/credit")
	channels.RegisterRoutes(mux, "/api/v1/channels")
	chats.RegisterRoutes(mux, "/api/v1/chats")
	notes.RegisterRoutes(mux, "/api/v1/notes")
	models.RegisterRoutes(mux, "/api/v1/models")
	knowledge.RegisterRoutes(mux, "/api/v1/knowledge")
	prompts.RegisterRoutes(mux, "/api/v1/prompts")
	tools.RegisterRoutes(mux, "/api/v1/tools")
	memories.RegisterRoutes(mux, "/api/v1/memories")
	folders.RegisterRoutes(mux, "/api/v1/folders")
	groups.RegisterRoutes(mux, "/api/v1/groups")
	files.RegisterRoutes(mux, "/api/v1/files")
	functions.RegisterRoutes(mux, "/api/v1/functions")
	evaluations.RegisterRoutes(mux, "/api/v1/evaluations")
	utils.RegisterRoutes(mux, "/api/v1/utils")
	scim.RegisterRoutes(mux, "/api/v1/scim/v2")
}

func registerStatic(mux *http.ServeMux, cfg *config.Config) {
	if info, err := os.Stat(cfg.StaticDir); err == nil && info.IsDir() {
		fileServer := http.FileServer(http.Dir(cfg.StaticDir))
		mux.Handle("/static/", http.StripPrefix("/static/", fileServer))
	}

	if _, err := os.Stat(filepath.Join(cfg.SPABuildDir, "index.html")); err != nil {
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			http.NotFound(w, r)
		})
		return
	}

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") || strings.HasPrefix(r.URL.Path, "/ollama") || strings.HasPrefix(r.URL.Path, "/openai") {
			http.NotFound(w, r)
			return
		}

		if !serveSPAFile(w, r, cfg, strings.TrimPrefix(r.URL.Path, "/")) {
			serveSPAFile(w, r, cfg, "index.html")
		}
	})
}

func serveSPAFile(w http.ResponseWriter, r *http.Request, cfg *config.Config, relative string) bool {
	if relative == "" {
		relative = "index.html"
	}

	target := filepath.Clean(filepath.Join(cfg.SPABuildDir, relative))
	root := cfg.SPABuildDir
	if root != "/" && !strings.HasSuffix(root, string(os.PathSeparator)) {
		root = root + string(os.PathSeparator)
	}

	if target != cfg.SPABuildDir && !strings.HasPrefix(target, root) {
		http.Error(w, "invalid path", http.StatusForbidden)
		return true
	}

	info, err := os.Stat(target)
	if err != nil || info.IsDir() {
		return false
	}

	http.ServeFile(w, r, target)
	return true
}
