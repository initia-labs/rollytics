package log

import (
	"log/slog"
	"os"

	"github.com/rs/zerolog"
	slogzerolog "github.com/samber/slog-zerolog"

	"github.com/initia-labs/rollytics/config"
)

func NewLogger(cfg *config.Config) *slog.Logger {
	var zerologLogger zerolog.Logger
	if cfg.GetLogFormat() == "json" {
		zerologLogger = zerolog.New(os.Stderr)
	} else {
		zerologLogger = zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr})
	}
	return slog.New(slogzerolog.Option{Level: cfg.GetLogLevel(), Logger: &zerologLogger}.NewZerologHandler())
}
