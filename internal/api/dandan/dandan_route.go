package dandan

import (
	"danmu-tool/internal/config"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
)

func RegisterRoute(route *chi.Mux) {
	timeout := config.GetConfig().DandanTimeout
	if timeout <= 0 {
		timeout = 60
	}
	dandanRoute := route.Group(func(d chi.Router) {
		dandanOptions := cors.New(cors.Options{
			AllowedOrigins: []string{"*"},
			AllowedMethods: []string{http.MethodGet, http.MethodPost},
			AllowedHeaders: []string{"*"},
		})
		d.Use(dandanOptions.Handler)
		d.Use(middleware.Timeout(time.Duration(1e9 * timeout)))
		d.Use(CacheMiddleware)
	})
	dandanRoute.Route("/api/v1/{token}/api/v2", func(r chi.Router) {
		r.Get("/comment/{id}", CommentHandler)
		r.Post("/match", MatchHandler)
	})
	dandanRoute.Route("/api/v1/{token}", func(r chi.Router) {
		r.Get("/comment/{id}", CommentHandler)
		r.Post("/match", MatchHandler)
	})
}
