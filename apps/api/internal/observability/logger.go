package observability

import (
	"log/slog"
	"os"
	"strings"
)

const ServiceName = "bodysense-api"

// ConfigureLogger installs the process-wide structured logger. slog.SetDefault
// also routes the standard library log package through this handler, which lets
// legacy log.Printf call sites migrate incrementally without losing JSON output.
func ConfigureLogger() *slog.Logger {
	level := parseLevel(os.Getenv("LOG_LEVEL"))
	appEnv := strings.TrimSpace(os.Getenv("APP_ENV"))
	if appEnv == "" {
		appEnv = "development"
	}

	handlerOptions := &slog.HandlerOptions{
		Level:     level,
		AddSource: shouldAddSource(appEnv),
	}

	var handler slog.Handler
	if strings.EqualFold(strings.TrimSpace(os.Getenv("LOG_FORMAT")), "text") {
		handler = slog.NewTextHandler(os.Stdout, handlerOptions)
	} else {
		handler = slog.NewJSONHandler(os.Stdout, handlerOptions)
	}

	logger := slog.New(handler).With(
		"service", ServiceName,
		"environment", appEnv,
	)
	slog.SetDefault(logger)
	return logger
}

func parseLevel(raw string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

func shouldAddSource(appEnv string) bool {
	if raw := strings.TrimSpace(os.Getenv("LOG_ADD_SOURCE")); raw != "" {
		return strings.EqualFold(raw, "true") || raw == "1"
	}
	return !strings.EqualFold(appEnv, "production")
}
