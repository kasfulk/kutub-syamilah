// Package main is the entry point for the Kutub Syamilah MCP server.
// It exposes the API as MCP tools via stdio (Claude Desktop) or HTTP transports.
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"time"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mark3labs/mcp-go/server"
	"github.com/redis/rueidis"

	"github.com/kasjfulk/kutub-syamilah/internal/cache"
	"github.com/kasjfulk/kutub-syamilah/internal/config"
	"github.com/kasjfulk/kutub-syamilah/internal/mcp"
	"github.com/kasjfulk/kutub-syamilah/internal/repository"
	"github.com/kasjfulk/kutub-syamilah/internal/service"
)

var (
	transport = flag.String("transport", "stdio", "Transport mode: stdio or http")
	httpAddr  = flag.String("http-addr", getEnv("KUTUB_MCP_ADDR", ":3001"), "HTTP server address (only for http transport)")
)

func main() {
	flag.Parse()

	if err := run(); err != nil {
		slog.Error("fatal", "error", err)
		os.Exit(1)
	}
}

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
	poolCfg.MaxConns = int32(cfg.MaxOpenConns)
	poolCfg.MinConns = int32(cfg.MaxIdleConns)
	poolCfg.HealthCheckPeriod = 30 * time.Second

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return err
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		return err
	}
	slog.Info("database connected", "max_conns", cfg.MaxOpenConns)

	// --- Redis ---
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

	// --- Create MCP Server ---
	mcpServer := server.NewMCPServer(
		"kutub-syamilah",
		"1.0.0",
		server.WithToolCapabilities(false),
	)

	// --- Register Tools ---
	mcpHandler := mcp.NewServer(svc)
	mcpServer.AddTool(mcp.ListKitabTool, mcpHandler.HandleListKitab)
	mcpServer.AddTool(mcp.GetKitabTool, mcpHandler.HandleGetKitab)
	mcpServer.AddTool(mcp.GetKontenTool, mcpHandler.HandleGetKonten)
	mcpServer.AddTool(mcp.SearchKitabTool, mcpHandler.HandleSearchKitab)
	mcpServer.AddTool(mcp.ListKategoriTool, mcpHandler.HandleListKategori)

	slog.Info("MCP server initialized", "tools", 5)

	// --- Start Server based on transport ---
	switch *transport {
	case "stdio":
		slog.Info("starting MCP server", "transport", "stdio")
		if err := server.ServeStdio(mcpServer); err != nil {
			return fmt.Errorf("stdio server failed: %w", err)
		}
	case "http":
		slog.Info("starting MCP server", "transport", "http", "addr", *httpAddr)
		httpServer := server.NewStreamableHTTPServer(mcpServer)
		if err := httpServer.Start(*httpAddr); err != nil {
			return fmt.Errorf("http server failed: %w", err)
		}
	default:
		return fmt.Errorf("unknown transport: %s (use 'stdio' or 'http')", *transport)
	}

	return nil
}

// parseRedisURL extracts host:port and password from a Redis URL.
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

// getEnv returns the value of an environment variable or a default if unset.
func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}
