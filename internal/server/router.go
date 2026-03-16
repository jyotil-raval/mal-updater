// internal/server/router.go
package server

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jyotil-raval/mal-updater/internal/server/handlers"
	appMiddleware "github.com/jyotil-raval/mal-updater/internal/server/middleware"
)

func NewRouter(h *handlers.Handlers) http.Handler {
	r := chi.NewRouter()

	// Global middleware
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.RequestID)

	// Public routes
	r.Post("/auth/token", h.IssueToken)

	// Protected routes
	r.Group(func(r chi.Router) {
		r.Use(appMiddleware.JWT)

		r.Post("/sync", h.Sync)
		r.Get("/list", h.List)
		r.Get("/anime/search", h.SearchAnime)
		r.Get("/anime/{id}", h.GetAnime)
		r.Patch("/anime/{id}", h.UpdateAnime)
	})

	return r
}
