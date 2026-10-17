package config

import (
	"errors"
	"flag"
	"io"
	"net"
	"net/url"
	"strconv"
	"strings"

	"github.com/caarlos0/env/v11"
)

type Config struct {
	JWTSecret  string `env:"JWT_SECRET"`
	Addr       string `env:"RUN_ADDRESS" envDefault:"localhost:8080"`
	DBDSN      string `env:"DATABASE_DSN"`
	AccrualURL string `env:"ACCRUAL_SYSTEM_ADDRESS"`
}

func Load(args []string) (Config, error) {
	var cfg Config
	if err := env.Parse(&cfg); err != nil {
		return Config{}, errors.New("cannot parse configuration environment")
	}
	fs := flag.NewFlagSet("gophermart", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.StringVar(&cfg.Addr, "a", cfg.Addr, "HTTP listen address")
	fs.StringVar(&cfg.DBDSN, "d", cfg.DBDSN, "PostgreSQL connection string")
	fs.StringVar(&cfg.AccrualURL, "r", cfg.AccrualURL, "accrual service URL")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return Config{}, flag.ErrHelp
		}
		return Config{}, errors.New("invalid flags: use -a, -d and -r with string values")
	}
	if fs.NArg() != 0 {
		return Config{}, errors.New("unexpected positional arguments")
	}
	if err := cfg.validate(); err != nil {
		return Config{}, err
	}
	if len(cfg.JWTSecret) < 32 {
		return Config{}, errors.New("JWT_SECRET must contain at least 32 bytes")
	}
	return cfg, nil
}

func (cfg Config) validate() error {
	_, port, err := net.SplitHostPort(cfg.Addr)
	if err != nil {
		return errors.New("RUN_ADDRESS (-a) must be host:port")
	}
	n, err := strconv.Atoi(port)
	if err != nil || n < 1 || n > 65535 {
		return errors.New("RUN_ADDRESS (-a) port must be between 1 and 65535")
	}
	if strings.TrimSpace(cfg.DBDSN) == "" {
		return errors.New("DATABASE_DSN (-d) is required")
	}
	u, err := url.Parse(cfg.AccrualURL)
	if err != nil || u.Hostname() == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return errors.New("ACCRUAL_SYSTEM_ADDRESS (-r) must be an absolute HTTP or HTTPS URL")
	}
	return nil
}
