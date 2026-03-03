package logger

import (
	"io"
	"log/slog"
	"os"
	"strings"

	"smart-daily/internal/config"

	"gopkg.in/lumberjack.v2"
)

func Init(cfg config.LogConfig) {
	level := parseLevel(cfg.Level)

	var writers []io.Writer
	if cfg.Console {
		writers = append(writers, os.Stdout)
	}
	if cfg.File != "" {
		writers = append(writers, &lumberjack.Logger{
			Filename:   cfg.File,
			MaxSize:    cfg.MaxSizeMB,
			MaxBackups: cfg.MaxBackups,
			MaxAge:     cfg.MaxAgeDays,
			LocalTime:  true,
		})
	}
	if len(writers) == 0 {
		writers = append(writers, os.Stdout)
	}

	h := slog.NewJSONHandler(io.MultiWriter(writers...), &slog.HandlerOptions{Level: level})
	slog.SetDefault(slog.New(h))
	Info("logger initialized", "level", cfg.Level, "file", cfg.File)
}

func Info(msg string, args ...any)  { slog.Info(msg, args...) }
func Warn(msg string, args ...any)  { slog.Warn(msg, args...) }
func Error(msg string, args ...any) { slog.Error(msg, args...) }
func Debug(msg string, args ...any) { slog.Debug(msg, args...) }

func parseLevel(s string) slog.Level {
	switch strings.ToLower(s) {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
