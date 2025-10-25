package utils

import (
	"log/slog"
	"os"
)

type LoggerConfig struct {
	jsonHandler *slog.JSONHandler
}

var LoggerConf = &LoggerConfig{}

func (c *LoggerConfig) InitLogger(debug bool) {
	var level = slog.LevelInfo
	if debug {
		level = slog.LevelDebug
	}

	c.jsonHandler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: level,
		ReplaceAttr: func(groups []string, attr slog.Attr) slog.Attr {
			if attr.Key == slog.TimeKey {
				formattedTime := attr.Value.Time().Format("2006-01-02 15:04:05")
				return slog.String(slog.TimeKey, formattedTime)
			}
			return attr
		},
	})
}

func getLogger(m map[string]string) *slog.Logger {
	logger := slog.New(LoggerConf.jsonHandler)
	for k, v := range m {
		logger = logger.With(k, v)
	}
	return logger
}

func GetComponentLogger(component string) *slog.Logger {
	return getLogger(map[string]string{"component": component})
}

func GetPlatformLogger(platform string) *slog.Logger {
	m := map[string]string{
		"platform":  platform,
		"component": "scraper",
	}
	return getLogger(m)
}
