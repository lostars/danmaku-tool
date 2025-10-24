package server

import (
	"danmu-tool/internal/api/dandan"
	"danmu-tool/internal/config"
	"danmu-tool/internal/utils"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

var logger = utils.GetComponentLogger("web-server")

func StartWebServer() {
	r := chi.NewRouter()

	r.Use(LoggerMiddleware)
	r.Use(middleware.Timeout(60 * time.Second))

	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("hi"))
	})

	// dandan api
	r.Route("/api/v1/{token}/api/v2", func(r chi.Router) {
		r.Get("/comment/{id}", dandan.CommentHandler)
		r.Get("/match", dandan.MatchHandler)
	})

	logger.Info("web server started", "port", config.Port)
	err := http.ListenAndServe(":"+strconv.FormatInt(int64(config.Port), 10), r)
	if err != nil {
		panic(err)
	}
}

func LoggerMiddleware(next http.Handler) http.Handler {
	start := time.Now()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		reqLogger := logger.With(
			slog.String("http_method", r.Method),
			slog.String("path", r.URL.Path),
		)
		requestId := r.Header.Get("X-Request-ID")
		if requestId != "" {
			reqLogger = logger.With("request_id", requestId)
		}

		ww := &statusRecorder{ResponseWriter: w, status: 200}

		next.ServeHTTP(ww, r)

		reqLogger.Info("request completed",
			slog.Int("status_code", ww.status),
			slog.Int64("latency_ms", time.Since(start).Milliseconds()),
		)
	})
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}
