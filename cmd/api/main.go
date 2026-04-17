// Package main is the entry point for the Kutub Syamilah API server.
// It wires all dependencies, registers routes, and implements graceful shutdown
// with proper context propagation.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/pprof"
	"net/url"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors"
	"github.com/redis/rueidis"

	"github.com/jackc/pgx/v5/pgxpool"

	spec "github.com/kasjfulk/kutub-syamilah/api/v1"
	"github.com/kasjfulk/kutub-syamilah/internal/cache"
	"github.com/kasjfulk/kutub-syamilah/internal/config"
	"github.com/kasjfulk/kutub-syamilah/internal/handler"
	"github.com/kasjfulk/kutub-syamilah/internal/middleware"
	"github.com/kasjfulk/kutub-syamilah/internal/repository"
	"github.com/kasjfulk/kutub-syamilah/internal/service"
)

func main() {
	if err := run(); err != nil {
		slog.Error("fatal", "error", err)
		os.Exit(1)
	}
}

// run encapsulates the application lifecycle so main() can handle the exit code.
// This pattern avoids os.Exit in deferred functions and enables clean testing.
func run() error {
	// --- Configuration ---
	cfg, err := config.New()
	if err != nil {
		return err
	}

	ctx := context.Background()

	// --- PostgreSQL ---
	poolCfg, err := pgxpool.ParseConfig(cfg.DatabaseURL)
	if err != nil {
		return err
	}
	// Tune connection pool to match workload.
	// Arabic FTS queries are CPU-heavy on PostgreSQL; limiting concurrency prevents DB overload.
	poolCfg.MaxConns = int32(cfg.MaxOpenConns)
	poolCfg.MinConns = int32(cfg.MaxIdleConns)
	// HealthCheckPeriod keeps idle connections alive and detects stale ones.
	poolCfg.HealthCheckPeriod = 30 * time.Second

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return err
	}
	defer pool.Close()

	// Verify database connectivity at startup.
	if err := pool.Ping(ctx); err != nil {
		return err
	}
	slog.Info("database connected", "max_conns", cfg.MaxOpenConns)

	// --- Redis (rueidis) ---
	redisAddr, redisPassword, err := parseRedisURL(cfg.RedisURL)
	if err != nil {
		return err
	}

	rdb, err := rueidis.NewClient(rueidis.ClientOption{
		InitAddress: []string{redisAddr},
		Password:    redisPassword,
	})
	if err != nil {
		return err
	}
	defer rdb.Close()
	slog.Info("redis connected", "addr", redisAddr)

	// --- Wire dependencies ---
	var repo repository.KitabRepository
	pgRepo := repository.NewPostgres(pool)

	if cfg.SearchBackend == "elastic" {
		esClient, err := elasticsearch.NewTypedClient(elasticsearch.Config{
			Addresses: []string{cfg.ElasticURL},
		})
		if err != nil {
			return fmt.Errorf("failed to create elastic client: %w", err)
		}
		repo = repository.NewElastic(pgRepo, esClient, cfg.ElasticIndex)
		slog.Info("search backend initialized", "backend", "elastic", "url", cfg.ElasticURL)
	} else {
		repo = pgRepo
		slog.Info("search backend initialized", "backend", "postgres")
	}

	redisCache := cache.NewRedis(rdb, cfg.CacheTTL)
	svc := service.NewKitab(repo, redisCache)
	h := handler.New(svc)

	// --- Router ---
	r := chi.NewRouter()

	// CORS middleware — configured from environment.
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   cfg.AllowedOrigins,
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300, // Maximum value not ignored by any of major browsers
	}))

	r.Use(middleware.Logger, middleware.Recover)

	r.Route("/v1", func(r chi.Router) {
		r.Get("/kitab", h.ListKitab)
		r.Get("/kitab/{id}", h.GetKitab)
		r.Get("/kitab/{id}/konten", h.GetKonten)
		r.Get("/kitab/{id}/konten/{hal}", h.GetKonten)
		r.Get("/search", h.Search)
		r.Get("/kategori", h.ListKategori)
	})

	// Health check endpoint.
	r.Get("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})

	// API documentation endpoints.
	r.Get("/docs/openapi.yaml", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/x-yaml")
		w.Header().Set("Cache-Control", "public, max-age=3600")
		_, _ = w.Write(spec.OpenAPI)
	})
	r.Get("/docs", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(swaggerUIHTML))
	})

	// pprof endpoints for production diagnostics.
	// In production, protect these behind auth middleware.
	r.HandleFunc("/debug/pprof/", pprof.Index)
	r.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	r.HandleFunc("/debug/pprof/profile", pprof.Profile)
	r.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	r.HandleFunc("/debug/pprof/trace", pprof.Trace)

	// --- Server ---
	srv := &http.Server{
		Addr:         cfg.Addr,
		Handler:      r,
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
		IdleTimeout:  120 * time.Second,
	}

	// --- Graceful shutdown ---
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	slog.Info("server started", "addr", cfg.Addr)

	<-quit
	slog.Info("shutting down...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		return err
	}

	slog.Info("server stopped gracefully")
	return nil
}

// parseRedisURL extracts host:port and password from a Redis URL.
// Supports formats: redis://host:port, redis://:password@host:port/db
func parseRedisURL(rawURL string) (addr, password string, err error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", "", err
	}
	addr = u.Host
	if addr == "" {
		addr = "localhost:6379"
	}
	if u.User != nil {
		password, _ = u.User.Password()
	}
	return addr, password, nil
}
