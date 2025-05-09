package logger

import (
	"io"
	"os"
	"time"

	"github.com/rs/zerolog"
)

var logger zerolog.Logger

func init() {
	writer := zerolog.MultiLevelWriter(
		SpecificLevelWriter{
			Writer: zerolog.ConsoleWriter{
				Out:        os.Stdout,
				TimeFormat: time.RFC3339,
			},
			Levels: []zerolog.Level{
				zerolog.DebugLevel, zerolog.InfoLevel, zerolog.WarnLevel,
			},
		},
		SpecificLevelWriter{
			Writer: zerolog.ConsoleWriter{
				Out: os.Stderr,
			},
			Levels: []zerolog.Level{
				zerolog.ErrorLevel, zerolog.FatalLevel, zerolog.PanicLevel,
			},
		},
	)
	logger = zerolog.New(writer).With().Timestamp().Logger()
}

func Info(msg string) {
	logger.Info().Msg(msg)
}

func Infof(format string, args ...interface{}) {
	logger.Info().Msgf(format, args...)
}

func Warn(msg string) {
	logger.Warn().Msg(msg)
}

func Warnf(format string, args ...interface{}) {
	logger.Warn().Msgf(format, args...)
}

func Error(msg string) {
	logger.Error().Msg(msg)
}

func Errorf(format string, args ...interface{}) {
	logger.Error().Msgf(format, args...)
}

func Debug(msg string) {
	logger.Debug().Msg(msg)
}

func Debugf(format string, args ...interface{}) {
	logger.Debug().Msgf(format, args...)
}

// multilevel writer from https://stackoverflow.com/questions/76858037/how-to-use-zerolog-to-filter-info-logs-to-stdout-and-error-logs-to-stderr
type SpecificLevelWriter struct {
	io.Writer
	Levels []zerolog.Level
}

func (w SpecificLevelWriter) WriteLevel(level zerolog.Level, p []byte) (int, error) {
	for _, l := range w.Levels {
		if l == level {
			return w.Write(p)
		}
	}
	return len(p), nil
}
