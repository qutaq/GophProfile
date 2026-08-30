package api

import (
	"log/slog"
	"net/http"
	"path/filepath"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/riandyrn/otelchi"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/qutaq/GophProfile/internal/handlers"
	"github.com/qutaq/GophProfile/internal/observability"
)

type Handlers struct {
	Avatars     *handlers.AvatarHandler
	Health      *handlers.HealthHandler
	Web         *handlers.WebHandler
	WebDir      string
	MetricsPath string
	Logger      *slog.Logger
	Metrics     *observability.Metrics
}

func NewRouter(h Handlers) http.Handler {
	r := chi.NewRouter()

	metricsPath := h.MetricsPath
	if metricsPath == "" {
		metricsPath = "/metrics"
	}

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(otelchi.Middleware("gophprofile-server",
		otelchi.WithChiRoutes(r),
		otelchi.WithRequestMethodInSpanName(true),
		otelchi.WithFilter(func(req *http.Request) bool {
			path := req.URL.Path
			return path != "/health" && path != "/livez" && path != "/readyz" && path != metricsPath
		}),
	))
	r.Use(withUserIDSpan)
	r.Use(observability.HTTPMiddleware(h.Logger, metricsPath, h.Metrics))
	r.Use(middleware.Recoverer)

	r.Handle(metricsPath, h.Metrics.Handler())

	r.Get("/livez", h.Health.Live)
	r.Get("/readyz", h.Health.Health)
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

func withUserIDSpan(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if userID := r.Header.Get("X-User-ID"); userID != "" {
			trace.SpanFromContext(r.Context()).SetAttributes(attribute.String("user.id", userID))
		}
		next.ServeHTTP(w, r)
	})
}
