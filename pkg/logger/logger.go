package logger

import (
	"os"
	"time"

	"github.com/rs/zerolog"
)

var logger zerolog.Logger

func init() {
	output := zerolog.ConsoleWriter{
		Out:        os.Stdout,
		TimeFormat: time.RFC3339,
	}
	logger = zerolog.New(output).With().Timestamp().Logger()
	zerolog.SetGlobalLevel(zerolog.InfoLevel)
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

func Debug(msg string) {
	logger.Debug().Msg(msg)
}
