// Package config handles the parsing and validation of application configuration
// from command-line arguments and environment variables.
package config

import (
	"fmt"
	"os"
	"time"

	"github.com/jessevdk/go-flags"
	"github.com/woozymasta/zenit/internal/logger"
	"github.com/woozymasta/zenit/internal/vars"
)

// AnyApplications mark for maintenance any (all) application
const AnyApplications = "AnyApp"

// Config represents the complete application flags configuration.
type Config struct {
	// betteralign:ignore

	Server    Server        `group:"Server Options" env-namespace:"ZENIT"`
	Storage   Storage       `group:"Storage Options" namespace:"db" env-namespace:"ZENIT_DB"`
	GeoIP     GeoIP         `group:"GeoIP Options" namespace:"geoip" env-namespace:"ZENIT_GEOIP"`
	RateLimit RateLimit     `group:"Rate Limit Options" namespace:"rate-limit" env-namespace:"ZENIT_RATE_LIMIT"`
	A2S       A2S           `group:"A2S Options" namespace:"a2s" env-namespace:"ZENIT_A2S"`
	Logger    logger.Config `group:"Logger Options" namespace:"log" env-namespace:"ZENIT_LOG"`

	Version bool `short:"v" long:"version" description:"Print version and build info"`
}

// Server holds web server configuration.
type Server struct {
	// betteralign:ignore

	Address     string   `short:"l" long:"address" env:"LISTEN_ADDRESS" description:"Server listen address" default:":8080"`
	AuthToken   string   `short:"t" long:"auth-token" env:"AUTH_TOKEN" description:"Admin authentication token"`
	AllowedApps []string `short:"a" long:"allowed-app" env:"ALLOWED_APPS" description:"List of allowed application names" default:"MetricZ" env-delim:","`
	MaxBodySize int64    `long:"max-body-size" env:"MAX_BODY_SIZE" description:"Max body size for incoming requests" default:"512"`
	TrustProxy  bool     `long:"trust-proxy" env:"TRUST_PROXY" description:"Trust X-Forwarded-For headers"`
	IgnoreUA    bool     `long:"ignore-user-agent" env:"IGNORE_USER_AGENT" description:"Disable User-Agent validation entirely"`
	ExpectedUA  string   `long:"expect-user-agent" env:"EXPECT_USER_AGENT" description:"Expected User-Agent string" default:""`
	ContentType string   `long:"expect-content-type" env:"EXPECT_CONTENT_TYPE" description:"Expected Content-Type header" default:"application/json"`
}

// Storage holds database configuration.
type Storage struct {
	// betteralign:ignore

	Path          string `short:"d" long:"path" env:"PATH" description:"Path to SQLite database" default:"zenit.db"`
	PruneEmpty    string `long:"prune-empty" description:"Delete nodes with no A2S data. Optional arg: App name." optional:"true" optional-value:"AnyApp"`
	CheckInactive string `long:"check-inactive" description:"Re-check nodes with no A2S data. Update if UP, delete if DOWN. Optional arg: App name." optional:"true" optional-value:"AnyApp"`
	CheckAll      string `long:"check-all" description:"Re-check ALL nodes. Update if UP, delete if DOWN. Optional arg: App name." optional:"true" optional-value:"AnyApp"`
	GenerateCount int    `long:"gen-fake-data" hidden:"true"`
}

// GeoIP holds MaxMind GeoIP configuration.
type GeoIP struct {
	// betteralign:ignore

	Path     string        `short:"g" long:"path" env:"PATH" description:"Path to MMDB file" default:"zenit.mmdb"`
	URL      string        `long:"url" env:"URL" description:"URL to download MMDB" default:"https://git.io/GeoLite2-Country.mmdb"`
	Interval time.Duration `long:"interval" env:"INTERVAL" description:"Update interval check" default:"24h"`
}

// A2S holds Source Query protocol configuration.
type A2S struct {
	// betteralign:ignore

	Timeout    time.Duration `long:"timeout" env:"TIMEOUT" description:"Query timeout" default:"3s"`
	BufferSize uint16        `long:"buffer-size" env:"BUFFER_SIZE" description:"Response body buffer size" default:"1400"`
}

// RateLimit holds API rate limiting configuration.
type RateLimit struct {
	// betteralign:ignore

	HardLimitCount int           `long:"hard-count" env:"HARD_COUNT" description:"Hard IP limit: requests count" default:"8"`
	HardLimitWin   time.Duration `long:"hard-window" env:"HARD_WINDOW" description:"Hard IP limit: window duration" default:"1m"`
	SoftLimitDur   time.Duration `long:"soft" env:"SOFT" description:"Soft Logic limit: ignore update if seen within duration" default:"5m"`
}

// Parse reads the configuration from flags and environment variables.
// It terminates the application if the configuration is invalid or if the help flag is invoked.
func Parse() *Config {
	var cfg Config
	parser := flags.NewParser(&cfg, flags.Default)
	parser.NamespaceDelimiter = "-"

	_, err := parser.Parse()
	if err != nil {
		if flagsErr, ok := err.(*flags.Error); ok {
			if flagsErr.Type == flags.ErrHelp {
				os.Exit(0)
			}
		}
		os.Exit(1)
	}

	if cfg.Version {
		vars.Print()
		os.Exit(0)
	}

	if cfg.Server.AuthToken == "" {
		fmt.Fprintln(os.Stderr,
			"Required flag `-t, --auth-token' or environment variable `ZENIT_AUTH_TOKEN` was not specified!")
		os.Exit(1)
	}

	return &cfg
}
