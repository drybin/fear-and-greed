package logger

import (
	"log/slog"
)

type ILogger interface {
	Debug(msg string, fields ...any)
	Info(msg string, fields ...any)
	Warn(msg string, fields ...any)
	Error(msg string, fields ...any)
}

func NewLogger() ILogger {
	return &logger{slog.Default()}
}

type logger struct {
	*slog.Logger
}

func (l *logger) Debug(msg string, attrs ...any) {
	slog.Debug(msg, attrs...)
}

func (l *logger) Info(msg string, attrs ...any) {
	slog.Info(msg, attrs...)
}

func (l *logger) Warn(msg string, attrs ...any) {
	slog.Warn(msg, attrs...)
}

func (l *logger) Error(msg string, attrs ...any) {
	slog.Error(msg, attrs...)
}
