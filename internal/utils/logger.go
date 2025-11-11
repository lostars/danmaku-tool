package utils

import (
	"log/slog"
	"os"
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

func InitLogger(debug bool) {
	if logger != nil {
		return
	}
	var level = slog.LevelInfo
	if debug {
		level = slog.LevelDebug
	}

	jsonHandler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: level,
		ReplaceAttr: func(groups []string, attr slog.Attr) slog.Attr {
			if attr.Key == slog.TimeKey {
				formattedTime := attr.Value.Time().Format("2006-01-02 15:04:05")
				return slog.String(slog.TimeKey, formattedTime)
			}
			return attr
		},
	})
	logger = slog.New(jsonHandler)
}
