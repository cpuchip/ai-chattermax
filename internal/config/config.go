// Package config loads ai-chattermax's runtime configuration from the
// environment.
package config

import (
	"fmt"
	"os"
	"strings"
)

// Config is the resolved runtime configuration.
type Config struct {
	Port         string
	DatabaseURL  string
	AuthMode     string // "dev" | "ibeco"
	CookieDomain string // e.g. ".ibeco.me" in prod; "" for localhost
	CookieSecure bool
	IbecoBaseURL string // where the ibeco handshake calls GET /api/me
}

// DevMode reports whether the dev authenticator (name-login, no ibeco.me) is in use.
func (c Config) DevMode() bool { return c.AuthMode == "dev" }

// Load resolves configuration from the environment, applying local-friendly
// defaults so `go run` / local docker compose work with zero config.
func Load() Config {
	c := Config{
		Port:         envOr("PORT", "8080"),
		DatabaseURL:  os.Getenv("DATABASE_URL"),
		AuthMode:     strings.ToLower(envOr("AUTH_MODE", "dev")),
		CookieDomain: os.Getenv("COOKIE_DOMAIN"),
		IbecoBaseURL: strings.TrimRight(envOr("IBECO_BASE_URL", "https://ibeco.me"), "/"),
	}
	if c.DatabaseURL == "" {
		c.DatabaseURL = buildDSN()
	}
	// Secure cookies whenever a cookie domain is set (i.e. behind HTTPS in prod).
	c.CookieSecure = c.CookieDomain != ""
	return c
}

// buildDSN assembles a libpq DSN from PG* parts (local-friendly defaults).
func buildDSN() string {
	host := envOr("PGHOST", "localhost")
	port := envOr("PGPORT", "5432")
	user := envOr("PGUSER", "chattermax")
	pass := envOr("PGPASSWORD", "chattermax")
	dbname := envOr("PGDATABASE", "chattermax")
	ssl := envOr("PGSSLMODE", "disable")
	return fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s", user, pass, host, port, dbname, ssl)
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
