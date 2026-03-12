package dandan

import (
	"danmaku-tool/internal/config"
	"danmaku-tool/internal/danmaku"
	"danmaku-tool/internal/service"
	"danmaku-tool/internal/web"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
)

func RegisterRoute(route *chi.Mux) {
	timeout := config.GetDandan().Timeout
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
		d.Use(middleware.SetHeader("Content-Type", "application/json"))
		d.Use(middleware.SetHeader("Cache-Control", "no-cache"))
		d.Use(middleware.Compress(5, "application/json"))
		d.Use(TokenValidatorMiddleware)
		d.Use(CacheMiddleware)
		d.Use(SourceModeConfigurer)
	})
	dandanRoute.Route("/api/v1/{token}/api/v2", apiRoute())
	dandanRoute.Route("/api/v1/{token}", apiRoute())
}

func apiRoute() func(r chi.Router) {
	return func(r chi.Router) {
		r.Get("/comment/{id}", CommentHandler)
		r.Post("/match", MatchHandler)
		r.Get("/search/anime", SearchAnime)
		r.Get("/bangumi/{id}", AnimeInfo)
	}
}

func init() {
	danmaku.Register(&dandanMode{})
}

type dandanMode struct{}

func (d dandanMode) Priority() int {
	// 注意需要在 dandan mode 实现初始化后进行，前者优先级是10
	return 100
}

func (d dandanMode) ServerInit() error {
	source = service.GetDandanSourceMode()
	if source == nil {
		return fmt.Errorf("dandan source mode not available")
	}
	return nil
}

var source service.DandanSourceMode

func SourceModeConfigurer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if source == nil {
			web.ResponseJSON(w, http.StatusNotFound, map[string]string{
				"message": "no available source",
			})
			return
		}
		next.ServeHTTP(w, r)
	})
}

func TokenValidatorMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		disableAuth := config.GetConfig().DanDan.DisableAuth
		if disableAuth {
			next.ServeHTTP(w, r)
			return
		}
		token := chi.URLParam(r, "token")
		for _, t := range config.GetConfig().Server.Tokens {
			if token == t {
				next.ServeHTTP(w, r)
				return
			}
		}
		web.ResponseJSON(w, http.StatusUnauthorized, map[string]string{"message": "Unauthorized"})
	})
}
