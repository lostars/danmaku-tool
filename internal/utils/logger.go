package utils

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
)

var logger *slog.Logger

func InfoLog(component string, msg string, args ...any) {
	logger.Info(msg, append([]any{"component", component}, args...)...)
}

func DebugLog(component string, msg string, args ...any) {
	logger.Debug(msg, append([]any{"component", component}, args...)...)
}

func ErrorLog(component string, msg string, args ...any) {
	logger.Error(msg, append([]any{"component", component}, args...)...)
}

func WarnLog(component string, msg string, args ...any) {
	logger.Warn(msg, append([]any{"component", component}, args...)...)
}

// InitLogger 允许重复初始化以适应不同场景日志输出
func InitLogger(debug bool, json bool) {
	var level = slog.LevelInfo
	if debug {
		level = slog.LevelDebug
	}
	options := &slog.HandlerOptions{
		Level: level,
		ReplaceAttr: func(groups []string, attr slog.Attr) slog.Attr {
			if attr.Key == slog.TimeKey {
				formattedTime := attr.Value.Time().Format("2006-01-02 15:04:05")
				return slog.String(slog.TimeKey, formattedTime)
			}
			return attr
		},
	}

	if json {
		logger = slog.New(slog.NewJSONHandler(os.Stdout, options))
	} else {
		logger = slog.New(&customTextHandler{
			w:    os.Stdout,
			opts: options,
		})
	}
}

type customTextHandler struct {
	*slog.TextHandler
	w    io.Writer
	opts *slog.HandlerOptions
}

func (h *customTextHandler) Handle(_ context.Context, r slog.Record) error {
	var sb strings.Builder
	prefix := fmt.Sprintf("%s %-5s %s", r.Time.Format("2006-01-02 15:04:05"), r.Level.String(), r.Message)
	sb.WriteString(prefix)
	r.Attrs(func(a slog.Attr) bool {
		if a.Key != slog.TimeKey && a.Key != slog.LevelKey {
			_, _ = fmt.Fprintf(&sb, " %s=%v", a.Key, a.Value)
		}
		return true
	})
	sb.WriteByte('\n')
	_, err := h.w.Write([]byte(sb.String()))
	return err
}

func (h *customTextHandler) Enabled(_ context.Context, level slog.Level) bool {
	minLevel := slog.LevelInfo
	if h.opts != nil && h.opts.Level != nil {
		minLevel = h.opts.Level.Level()
	}
	return level >= minLevel
}

func (h *customTextHandler) WithAttrs(_ []slog.Attr) slog.Handler {
	return h
}

func (h *customTextHandler) WithGroup(_ string) slog.Handler {
	return h
}
