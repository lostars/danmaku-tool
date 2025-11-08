package dandan

import (
	"danmaku-tool/internal/api"
	"danmaku-tool/internal/config"
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
	dandanRoute.Route("/api/v1/{token}/api/v2", apiRoute())
	dandanRoute.Route("/api/v1/{token}", apiRoute())
}

func apiRoute() func(r chi.Router) {
	return func(r chi.Router) {
		r.Use(TokenValidatorMiddleware)
		r.Get("/comment/{id}", CommentHandler)
		r.Post("/match", MatchHandler)
		r.Get("/search/anime", SearchAnime)
		r.Get("/bangumi/{id}", AnimeInfo)
	}
}

func TokenValidatorMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := chi.URLParam(r, "token")
		for _, t := range config.GetConfig().Server.Tokens {
			if token == t {
				next.ServeHTTP(w, r)
				return
			}
		}
		api.ResponseJSON(w, http.StatusUnauthorized, map[string]string{"message": "Unauthorized"})
	})
}
