package logging

import "log/slog"

// Logger interface defines the logging contract for the domain layer
type Logger interface {
	Debug(msg string, args ...any)
	Info(msg string, args ...any)
	Warn(msg string, args ...any)
	Error(msg string, args ...any)
	With(args ...any) Logger
	WithComponent(component string) Logger
}

// Standards is the standard logging interface used throughout the domain
type Standards interface {
	Logger
	// Additional methods can be added here as needed
}

// SlogAdapter adapts slog.Logger to our Logger interface
type SlogAdapter struct {
	logger *slog.Logger
}

// NewSlogAdapter creates a new slog adapter
func NewSlogAdapter(logger *slog.Logger) Standards {
	return &SlogAdapter{logger: logger}
}

func (s *SlogAdapter) Debug(msg string, args ...any) {
	s.logger.Debug(msg, args...)
}

func (s *SlogAdapter) Info(msg string, args ...any) {
	s.logger.Info(msg, args...)
}

func (s *SlogAdapter) Warn(msg string, args ...any) {
	s.logger.Warn(msg, args...)
}

func (s *SlogAdapter) Error(msg string, args ...any) {
	s.logger.Error(msg, args...)
}

func (s *SlogAdapter) With(args ...any) Logger {
	return &SlogAdapter{logger: s.logger.With(args...)}
}

func (s *SlogAdapter) WithComponent(component string) Logger {
	return &SlogAdapter{logger: s.logger.With("component", component)}
}