package api

import (
	"net/http"
	"path/filepath"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/qutaq/GophProfile/internal/handlers"
)

type Handlers struct {
	Avatars *handlers.AvatarHandler
	Health  *handlers.HealthHandler
	Web     *handlers.WebHandler
	WebDir  string
}

func NewRouter(h Handlers) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Get("/health", h.Health.Health)

	r.Route("/api/v1", func(r chi.Router) {
		r.Post("/avatars", h.Avatars.Upload)
		r.Get("/avatars/{avatar_id}", h.Avatars.GetByID)
		r.Get("/avatars/{avatar_id}/metadata", h.Avatars.Metadata)
		r.Delete("/avatars/{avatar_id}", h.Avatars.Delete)

		r.Get("/users/{user_id}/avatar", h.Avatars.GetByUserID)
		r.Get("/users/{user_id}/avatars", h.Avatars.ListByUserID)
		r.Delete("/users/{user_id}/avatar", h.Avatars.DeleteUserAvatar)
	})

	if h.Web != nil {
		webDir := h.WebDir
		if webDir == "" {
			webDir = "web"
		}
		staticDir := http.Dir(filepath.Join(webDir, "static"))
		r.Mount("/web/static", http.StripPrefix("/web/static", http.FileServer(staticDir)))

		r.Get("/", func(w http.ResponseWriter, req *http.Request) {
			http.Redirect(w, req, "/web/upload", http.StatusFound)
		})
		r.Get("/web", func(w http.ResponseWriter, req *http.Request) {
			http.Redirect(w, req, "/web/upload", http.StatusFound)
		})
		r.Get("/web/upload", h.Web.UploadPage)
		r.Post("/web/upload", h.Web.Upload)
		r.Get("/web/gallery/{user_id}", h.Web.Gallery)
	}

	return r
}
