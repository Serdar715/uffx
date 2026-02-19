package output

import (
	"log/slog"
	"os"
)

// SetupLogger initializes the global logger.
// It writes to stderr to avoid interfering with stdout output.
func SetupLogger(debug bool, jsonFormat bool) {
	opts := &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}

	if debug {
		opts.Level = slog.LevelDebug
	}

	var handler slog.Handler
	if jsonFormat {
		handler = slog.NewJSONHandler(os.Stderr, opts)
	} else {
		handler = slog.NewTextHandler(os.Stderr, opts)
	}

	logger := slog.New(handler)
	slog.SetDefault(logger)
}
