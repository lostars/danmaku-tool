package cmd

import (
	"bytes"
	"context"
	"danmaku-tool/internal/api/dandan"
	"danmaku-tool/internal/config"
	"danmaku-tool/internal/danmaku"
	"danmaku-tool/internal/utils"
	"danmaku-tool/internal/web"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"runtime/debug"
	"strconv"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-co-op/gocron/v2"
	"github.com/spf13/cobra"
)

// timeout in seconds
const (
	defaultServerPort    = 8089
	defaultServerTimeout = 60
	defaultTimeout       = 120
	defaultReadTimeout   = 10
	defaultWriteTimeout  = 10
	defaultCancelTimeout = 5
)

type schedulerType struct {
	scheduler gocron.Scheduler
}

func (s *schedulerType) Priority() int {
	return 10
}

func (s *schedulerType) Finalize() error {
	if s.scheduler != nil {
		if err := s.scheduler.Shutdown(); err != nil {
			return err
		}
	}
	return nil
}

func (s *schedulerType) ServerInit() error {
	return nil
}

func (s *schedulerType) AsyncInit() error {
	scheduler, err := gocron.NewScheduler()
	if err != nil {
		return err
	}
	for _, j := range danmaku.Jobs() {
		if e := j.CreateJob(scheduler); e != nil {
			utils.ErrorLog(webServerC, e.Error())
		}
	}
	s.scheduler = scheduler
	s.scheduler.Start()
	return nil
}

func serverCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "server",
		Short: "run as a web server",
	}
	var port int
	cmd.Flags().IntVarP(&port, "port", "p", 0, "server port")

	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		InitServer()
		r := chi.NewRouter()

		r.Use(LoggerMiddleware)
		if port <= 0 {
			port = config.GetConfig().Server.Port
		}
		if port <= 0 {
			port = defaultServerPort
		}
		timeout := config.GetConfig().Server.Timeout
		if timeout <= 0 {
			timeout = defaultServerTimeout
		}
		r.Use(middleware.Timeout(time.Duration(1e9 * timeout)))

		r.Get("/", func(w http.ResponseWriter, r *http.Request) {
			web.ResponseJSON(w, http.StatusOK, map[string]string{"version": config.Version})
		})

		// dandan api
		dandan.RegisterRoute(r)

		srv := &http.Server{
			Addr:         ":" + strconv.FormatInt(int64(port), 10),
			Handler:      RecoverMiddleware(r),
			IdleTimeout:  defaultTimeout * time.Second,
			ReadTimeout:  defaultReadTimeout * time.Second,
			WriteTimeout: defaultWriteTimeout * time.Second,
		}

		quit := make(chan os.Signal, 1)
		signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
		go func() {
			utils.InfoLog(webServerC, "web server started", "port", port)
			if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
				utils.ErrorLog(webServerC, "server failed to start", "error", err)
				quit <- syscall.SIGTERM
			}
		}()
		<-quit

		ctx, cancel := context.WithTimeout(context.Background(), defaultCancelTimeout*time.Second)
		defer cancel()

		if err := srv.Shutdown(ctx); err != nil {
			utils.ErrorLog(webServerC, "server forced to shutdown", "error", err)
		}

		Release()

		return nil
	}

	return cmd
}

func RecoverMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				debug.PrintStack()
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

const webServerC = "web_server"

func LoggerMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var bodyBytes []byte
		if r.Body != nil {
			bodyBytes, _ = io.ReadAll(r.Body)
		}
		// rewrite body
		r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

		start := time.Now()
		recorder := &web.StatusRecorder{ResponseWriter: w}
		next.ServeHTTP(recorder, r)

		utils.InfoLog(webServerC, "request completed",
			slog.String("method", r.Method),
			slog.String("path", r.URL.Path),
			slog.Int("status", recorder.Status),
			slog.Int64("cost_ms", time.Since(start).Milliseconds()),
			slog.String("ip", web.GetRealIP(r)),
			"query", r.URL.Query(),
			"body", string(bodyBytes),
		)
	})
}

func init() {
	rootCmd.AddCommand(serverCmd())
	schedulerInterface := &schedulerType{}
	danmaku.RegisterFinalizer(schedulerInterface)
	danmaku.Register(schedulerInterface)
}
