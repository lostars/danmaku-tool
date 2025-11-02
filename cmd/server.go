package cmd

import (
	"context"
	"danmaku-tool/internal/api"
	"danmaku-tool/internal/api/dandan"
	"danmaku-tool/internal/config"
	"danmaku-tool/internal/utils"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/spf13/cobra"
)

func serverCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "server",
		Short: "run as a web server",
	}
	var port int
	cmd.Flags().IntVarP(&port, "port", "p", 0, "server port")

	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		Init()
		InitServer()
		dandan.InitDandanCache()
		httpLogger = utils.GetComponentLogger("dandan-api")
		r := chi.NewRouter()

		r.Use(LoggerMiddleware)
		if port <= 0 {
			port = config.GetConfig().Server.Port
		}
		timeout := config.GetConfig().Server.Timeout
		if timeout <= 0 {
			timeout = 60
		}
		r.Use(middleware.Timeout(time.Duration(1e9 * timeout)))

		r.Get("/", func(w http.ResponseWriter, r *http.Request) {
			api.ResponseJSON(w, http.StatusOK, map[string]string{"version": ""})
		})

		// dandan api
		dandan.RegisterRoute(r)

		srv := &http.Server{
			Addr:         ":" + strconv.FormatInt(int64(port), 10),
			Handler:      r,
			IdleTimeout:  120 * time.Second,
			ReadTimeout:  10 * time.Second,
			WriteTimeout: 10 * time.Second,
		}

		quit := make(chan os.Signal, 1)
		signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
		go func() {
			utils.GetComponentLogger("web-server").Info("web server started", "port", port)
			if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
				httpLogger.Error("server failed to start", "error", err)
				quit <- syscall.SIGTERM
			}
		}()
		<-quit

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := srv.Shutdown(ctx); err != nil {
			httpLogger.Error("server forced to shutdown", "error", err)
		}

		Release()

		return nil
	}

	return cmd
}

var httpLogger *slog.Logger

func LoggerMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		reqLogger := httpLogger.With(
			slog.String("http_method", r.Method),
			slog.String("path", r.URL.Path),
		)
		requestId := r.Header.Get("X-Request-ID")
		if requestId != "" {
			reqLogger = httpLogger.With("request_id", requestId)
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
