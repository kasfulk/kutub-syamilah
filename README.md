# Kutub Syamilah API

A production-grade, high-performance REST API for the Kutub Syamilah digital library. Optimized for Arabic full-text search, high concurrency, and low latency.

## 🚀 Architecture

The system is built with a modern, high-performance stack:

- **Language**: Go 1.25.1
- **Primary Database**: PostgreSQL (Source of truth for library metadata and content)
- **Search Engine**: Elasticsearch v8 (Powering advanced Arabic full-text search with normalization and stemming)
- **Cache**: Redis via `rueidis` (Cache-aside pattern to reduce DB load)
- **Router**: `chi` v5 (Lightweight and fast)

## ✨ Features

- **Advanced Arabic Search**: Full-text search with field boosting, fuzziness, and highlighting.
- **High Concurrency**: Uses `singleflight` to prevent cache stampedes and `sync.Pool` for JSON buffer reuse.
- **Parallel I/O**: Repository layer uses `errgroup` for parallel COUNT and data fetching.
- **Graceful Shutdown**: Sanely handles SIGINT/SIGTERM.
- **Built-in Interactive Docs**: Swagger UI documentation included.
- **Efficient Migration**: Bulk indexing script for syncing PostgreSQL to Elasticsearch.

## 🛠 Setup

### 1. Prerequisites

- Go 1.25.1+
- PostgreSQL
- Redis
- Elasticsearch v8

### 2. Environment Variables

Configure the application via environment variables or a `.env` file (if using Air):

| Variable | Description | Default |
|----------|-------------|---------|
| `KUTUB_DATABASE_URL` | PostgreSQL DSN (**Required**) | - |
| `KUTUB_REDIS_URL` | Redis URL | `redis://localhost:6379/0` |
| `KUTUB_ELASTIC_URL` | Elasticsearch Endpoint | `http://localhost:9200` |
| `KUTUB_ELASTIC_INDEX` | Elasticsearch Index Name | `konten_kitab` |
| `KUTUB_SEARCH_BACKEND` | `elastic` or `postgres` | `elastic` |
| `KUTUB_ADDR` | Server listen address | `:3000` |
| `KUTUB_LOG_LEVEL` | Logging level (`1` for debug) | `1` |

### 3. Installation

```bash
git clone <repository-url>
cd kutub-syamilah
make deps
```

## 🏃 Usage

### Start the API Server
```bash
make run
```
For development with live-reload:
```bash
make dev
```

### Sync Data to Elasticsearch
Ensure PostgreSQL is populated with data, then run the sync command to build the search index:
```bash
make sync
```

## 🔍 API Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/v1/kitab` | List all available kitab with pagination/filters |
| `GET` | `/v1/kitab/{id}` | Get metadata for a specific kitab |
| `GET` | `/v1/kitab/{id}/konten` | Get paginated content for a kitab |
| `GET` | `/v1/search` | Full-text search across all library content |
| `GET` | `/v1/kategori` | List all unique categories |
| `GET` | `/docs` | Interactive Swagger UI documentation |
| `GET` | `/health` | Service health status |

## 🧪 Development

See the [Makefile](Makefile) for available development commands:

- `make build`: Build the API binary
- `make test`: Run tests with race detection
- `make lint`: Run code linters (`golangci-lint`)
- `make fmt`: Format code and tidy imports
- `make bench`: Run repository performance benchmarks
