package log

import (
	"io"
	"log/slog"
	"os"

	"gopkg.in/natefinch/lumberjack.v2"
)

var (
	logger   *slog.Logger
	logLevel = &slog.LevelVar{}
)

// Init initializes the global logger with rotation and a default level.
func Init(level Level, filename string, isProduction bool) {
	var output io.Writer

	if isProduction {
		output = &lumberjack.Logger{
			Filename:   filename,
			MaxSize:    10, // megabytes
			MaxBackups: 5,
			MaxAge:     10, // days
			Compress:   true,
			LocalTime:  true,
		}
	} else {
		output = os.Stdout
	}

	logLevel.Set(slogLevel(level))

	handler := slog.NewJSONHandler(output, &slog.HandlerOptions{
		Level: logLevel,
	})

	logger = slog.New(handler)
}

// Get returns the configured logger instance. If Init was not called, it lazily
// creates a console logger at INFO level to avoid nil dereferences.
func Get() *slog.Logger {
	if logger == nil {
		// Lazy initialize a sensible default (console text handler at INFO level)
		logLevel.Set(slogLevel(LevelInfo))
		handler := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: logLevel})
		logger = slog.New(handler)
	}
	return logger
}

// SetLevel dynamically changes the log level of the global logger.
func SetLevel(level Level) {
	logLevel.Set(slogLevel(level))
}

// slogLevel converts our custom Level to a slog.Level.
func slogLevel(level Level) slog.Level {
	switch level {
	case LevelDebug:
		return slog.LevelDebug
	case LevelInfo:
		return slog.LevelInfo
	case LevelWarn:
		return slog.LevelWarn
	case LevelError:
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
