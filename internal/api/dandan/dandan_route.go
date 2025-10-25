package dandan

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors"
)

func RegisterRoute(route *chi.Mux) {
	dandanRoute := route.Group(func(d chi.Router) {
		dandanOptions := cors.New(cors.Options{
			AllowedOrigins: []string{"*"},
			AllowedMethods: []string{http.MethodGet, http.MethodPost},
			AllowedHeaders: []string{"*"},
		})
		d.Use(dandanOptions.Handler)
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
