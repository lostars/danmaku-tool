package utils

import (
	"danmu-tool/internal/config"
	"log/slog"
	"os"
)

func GetLogger(m map[string]string) *slog.Logger {
	var level = slog.LevelInfo
	if config.Debug {
		level = slog.LevelDebug
	}

	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: level,
		ReplaceAttr: func(groups []string, attr slog.Attr) slog.Attr {
			if attr.Key == slog.TimeKey {
				formattedTime := attr.Value.Time().Format("2006-01-02 15:04:05")
				return slog.String(slog.TimeKey, formattedTime)
			}
			return attr
		},
	})
	logger := slog.New(handler)
	for k, v := range m {
		logger = logger.With(k, v)
	}
	return logger
}

func GetComponentLogger(component string) *slog.Logger {
	return GetLogger(map[string]string{"component": component})
}

func GetPlatformLogger(platform string) *slog.Logger {
	m := map[string]string{
		"platform":  platform,
		"component": platform,
	}
	return GetLogger(m)
}
