// Package logger initializes and configures the global zerolog instance.
package logger

import (
	"io"
	"os"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// Config holds configuration options for the application logger.
type Config struct {
	Level  string `long:"level" env:"LEVEL" description:"Log level (trace, debug, info, warn, error)" default:"info" json:"level"`
	Format string `long:"format" env:"FORMAT" description:"Log format (text or json)" default:"console" json:"format"`
	Output string `long:"output" env:"OUTPUT" description:"Log output (stdout, stderr or file path)" default:"stderr" json:"output"`
}

// Setup initializes the global logger based on the provided configuration options.
// It sets the log level, output destination (stdout, stderr, or file), and format (JSON or Console).
func Setup(cfg Config) {
	var err error

	// Level
	level, err := zerolog.ParseLevel(cfg.Level)
	if err != nil {
		level = zerolog.InfoLevel
	}
	zerolog.SetGlobalLevel(level)

	// Output Writer
	var writer io.Writer
	switch cfg.Output {
	case "stdout":
		writer = os.Stdout
	case "stderr":
		writer = os.Stderr
	default:
		file, err := os.OpenFile(cfg.Output, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
		if err != nil {
			// Fallback to stderr if file fails
			tempLogger := zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr}).With().Timestamp().Logger()
			tempLogger.Error().Err(err).Str("path", cfg.Output).Msg("Failed to open log file, falling back to stderr")
			writer = os.Stderr
		} else {
			writer = file
		}
	}

	// Format
	if cfg.Format == "json" {
		log.Logger = zerolog.New(writer).With().Timestamp().Logger()
	} else {
		consoleWriter := zerolog.ConsoleWriter{
			Out:        writer,
			TimeFormat: time.RFC3339,
		}

		// Detect colors: check if writer is file/tty AND NO_COLOR is not set
		if f, ok := writer.(*os.File); ok {
			if os.Getenv("NO_COLOR") != "" || !isTerminal(f) {
				consoleWriter.NoColor = true
			}
		}

		log.Logger = log.Output(consoleWriter)
	}
}

// isTerminal checks if the provided file descriptor refers to a character device (terminal).
func isTerminal(f *os.File) bool {
	stat, err := f.Stat()
	if err != nil {
		return false
	}

	return (stat.Mode() & os.ModeCharDevice) != 0
}
