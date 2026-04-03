package observability

import (
	"log/slog"
	"os"

	"github.com/rs/zerolog"
	slogzerolog "github.com/samber/slog-zerolog/v2"
)

const (
	FormatJSON    = "json"
	FormatConsole = "console"
)

// NewLogger creates an *slog.Logger backed by zerolog.
//
// The level parameter is parsed via zerolog.ParseLevel (e.g. "debug", "info",
// "warn", "error"). The format parameter controls output style: FormatConsole
// produces human-readable output, while FormatJSON (or any other value)
// produces structured JSON.
func NewLogger(level string, format string) *slog.Logger {
	zlLevel, err := zerolog.ParseLevel(level)
	if err != nil {
		zlLevel = zerolog.DebugLevel
	}

	var zl zerolog.Logger
	if format == FormatConsole {
		zl = zerolog.New(zerolog.ConsoleWriter{Out: os.Stdout}).
			Level(zlLevel).
			With().
			Timestamp().
			Logger()
	} else {
		zl = zerolog.New(os.Stdout).
			Level(zlLevel).
			With().
			Timestamp().
			Logger()
	}

	handler := slogzerolog.Option{
		Logger: &zl,
	}.NewZerologHandler()

	return slog.New(handler)
}
