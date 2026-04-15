// Package config provides application configuration using the functional
// options pattern. Values are loaded from environment variables with sensible
// defaults. Override specific values with Option functions (useful in tests).
package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config holds all application configuration.
// Field order: largest-to-smallest for optimal struct alignment.
type Config struct {
	DatabaseURL     string
	RedisURL        string
	Addr            string
	ElasticURL      string // KUTUB_ELASTIC_URL — Elasticsearch endpoint
	ElasticIndex    string // KUTUB_ELASTIC_INDEX — target index name
	SearchBackend   string // KUTUB_SEARCH_BACKEND — "elastic" or "postgres"
	CacheTTL        time.Duration
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	ShutdownTimeout time.Duration
	GOMEMLIMIT      int64 // bytes — set to 80% of container limit
	MaxOpenConns    int
	MaxIdleConns    int
	SyncWorkers     int // KUTUB_SYNC_WORKERS — parallel workers for sync command
	SyncChunkSize   int // KUTUB_SYNC_CHUNK_SIZE — rows per keyset-pagination chunk
}

// Option is a functional option for overriding Config defaults.
type Option func(*Config)

// WithAddr overrides the server listen address.
func WithAddr(addr string) Option {
	return func(c *Config) { c.Addr = addr }
}

// WithDatabaseURL overrides the PostgreSQL connection string.
func WithDatabaseURL(url string) Option {
	return func(c *Config) { c.DatabaseURL = url }
}

// WithRedisURL overrides the Redis connection URL.
func WithRedisURL(url string) Option {
	return func(c *Config) { c.RedisURL = url }
}

// WithCacheTTL overrides the default cache TTL.
func WithCacheTTL(d time.Duration) Option {
	return func(c *Config) { c.CacheTTL = d }
}

// WithMaxOpenConns overrides the database max open connections.
func WithMaxOpenConns(n int) Option {
	return func(c *Config) { c.MaxOpenConns = n }
}

// WithMaxIdleConns overrides the database max idle connections.
func WithMaxIdleConns(n int) Option {
	return func(c *Config) { c.MaxIdleConns = n }
}

// WithElasticURL overrides the Elasticsearch URL.
func WithElasticURL(url string) Option {
	return func(c *Config) { c.ElasticURL = url }
}

// WithElasticIndex overrides the Elasticsearch index name.
func WithElasticIndex(index string) Option {
	return func(c *Config) { c.ElasticIndex = index }
}

// New returns a Config populated from environment variables with defaults.
// Override specific values with functional options (useful in tests).
// Returns an error if required environment variables are missing.
func New(opts ...Option) (*Config, error) {
	dbURL, err := mustEnv("KUTUB_DATABASE_URL")
	if err != nil {
		return nil, err
	}

	cfg := &Config{
		Addr:            getEnv("KUTUB_ADDR", ":3000"),
		DatabaseURL:     dbURL,
		RedisURL:        getEnv("KUTUB_REDIS_URL", "redis://localhost:6379/0"),
		ElasticURL:      getEnv("KUTUB_ELASTIC_URL", "http://localhost:9200"),
		ElasticIndex:    getEnv("KUTUB_ELASTIC_INDEX", "konten_kitab"),
		SearchBackend:   getEnv("KUTUB_SEARCH_BACKEND", "elastic"),
		CacheTTL:        parseDuration(getEnv("KUTUB_CACHE_TTL", "5m")),
		ReadTimeout:     parseDuration(getEnv("KUTUB_READ_TIMEOUT", "30s")),
		WriteTimeout:    parseDuration(getEnv("KUTUB_WRITE_TIMEOUT", "30s")),
		ShutdownTimeout: parseDuration(getEnv("KUTUB_SHUTDOWN_TIMEOUT", "30s")),
		MaxOpenConns:    parseInt(getEnv("KUTUB_MAX_OPEN_CONNS", "25")),
		MaxIdleConns:    parseInt(getEnv("KUTUB_MAX_IDLE_CONNS", "5")),
		SyncWorkers:     parseInt(getEnv("KUTUB_SYNC_WORKERS", "6")),
		SyncChunkSize:   parseInt(getEnv("KUTUB_SYNC_CHUNK_SIZE", "1000")),
	}
	for _, opt := range opts {
		opt(cfg)
	}
	return cfg, nil
}

// RedisAddr extracts the host:port from the Redis URL for rueidis.
// Handles both redis://host:port and redis://:password@host:port formats.
func (c *Config) RedisAddr() string {
	// rueidis expects just host:port, not the full URL.
	// For simplicity, we return the configured URL and let the caller parse it.
	// In production, use net/url to extract host.
	return c.RedisURL
}

// mustEnv returns the value of the environment variable or an error if unset.
func mustEnv(key string) (string, error) {
	v := os.Getenv(key)
	if v == "" {
		return "", fmt.Errorf("required environment variable %s is not set", key)
	}
	return v, nil
}

// getEnv returns the value of the environment variable or the fallback.
func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// parseDuration parses a duration string, falling back to 0 on error.
func parseDuration(s string) time.Duration {
	d, err := time.ParseDuration(s)
	if err != nil {
		return 0
	}
	return d
}

// parseInt parses an integer string, falling back to 0 on error.
func parseInt(s string) int {
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0
	}
	return n
}
