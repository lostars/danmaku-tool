package cmd

import (
	"danmu-tool/internal/api"
	"danmu-tool/internal/api/dandan"
	"danmu-tool/internal/utils"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/spf13/cobra"
)

func serverCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "server",
		Short: "start as a web server",
	}
	var port int
	cmd.Flags().IntVarP(&port, "port", "p", 8089, "server port")

	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		Init()
		r := chi.NewRouter()

		r.Use(LoggerMiddleware)
		r.Use(middleware.Timeout(60 * time.Second))

		r.Get("/", func(w http.ResponseWriter, r *http.Request) {
			api.ResponseJSON(w, http.StatusOK, map[string]string{"version": ""})
		})

		// dandan api
		dandan.RegisterRoute(r)

		utils.GetComponentLogger("web-server").Info("web server started", "port", port)
		err := http.ListenAndServe(":"+strconv.FormatInt(int64(port), 10), r)
		if err != nil {
			return err
		}

		return nil
	}

	return cmd
}

func LoggerMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		logger := utils.GetComponentLogger("dandan-api")
		reqLogger := logger.With(
			slog.String("http_method", r.Method),
			slog.String("path", r.URL.Path),
		)
		requestId := r.Header.Get("X-Request-ID")
		if requestId != "" {
			reqLogger = logger.With("request_id", requestId)
		}

		ww := &statusRecorder{ResponseWriter: w, status: http.StatusOK}

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

func init() {
	rootCmd.AddCommand(serverCmd())
}
