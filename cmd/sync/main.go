package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/elastic/go-elasticsearch/v8/esutil"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/kasjfulk/kutub-syamilah/internal/config"
	"github.com/kasjfulk/kutub-syamilah/internal/model"
	"github.com/kasjfulk/kutub-syamilah/internal/repository"
)

// Index settings for syncing
const mappingAndSettings = `
{
  "settings": {
    "analysis": {
      "char_filter": {
        "remove_harakat": {
          "type": "pattern_replace",
          "pattern": "[\\u064B-\\u065F]",
          "replacement": ""
        }
      },
      "filter": {
        "arabic_stop": {
          "type": "stop",
          "stopwords": "_arabic_"
        },
        "arabic_stemmer": {
          "type": "stemmer",
          "language": "arabic"
        },
        "arabic_synonym": {
          "type": "synonym",
          "lenient": true,
          "synonyms": [
            "زكاة, زكاه",
            "ربا, فوائد"
          ]
        }
      },
      "analyzer": {
        "arabic_custom": {
          "tokenizer": "standard",
          "char_filter": [
            "remove_harakat"
          ],
          "filter": [
            "lowercase",
            "decimal_digit",
            "arabic_normalization",
            "arabic_stop",
            "arabic_stemmer",
            "arabic_synonym"
          ]
        }
      }
    }
  },
  "mappings": {
    "properties": {
      "kitab_id": { "type": "integer" },
      "section_id": { "type": "integer" },
      "nomor_bagian": { "type": "integer" },
      "kategori": { "type": "keyword" },
      "judul": {
        "type": "text",
        "analyzer": "arabic_custom"
      },
      "penulis": {
        "type": "text",
        "analyzer": "arabic_custom"
      },
      "publisher": {
        "type": "text",
        "analyzer": "arabic_custom"
      },
      "isi_teks": {
        "type": "text",
        "analyzer": "arabic_custom"
      }
    }
  }
}
`

func main() {
	if err := run(); err != nil {
		slog.Error("fatal", "error", err)
		os.Exit(1)
	}
}

func run() error {
	ctx := context.Background()

	cfg, err := config.New()
	if err != nil {
		return fmt.Errorf("config error: %w", err)
	}

	// 1. Initialize Postgres
	slog.Info("Connecting to postgres...")
	poolCfg, err := pgxpool.ParseConfig(cfg.DatabaseURL)
	if err != nil {
		return err
	}
	poolCfg.MaxConns = int32(cfg.SyncWorkers + 2) // Ensure enough connections for workers
	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return err
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		return fmt.Errorf("postgres ping: %w", err)
	}
	pgRepo := repository.NewPostgres(pool)

	// 2. Initialize Elasticsearch
	slog.Info("Connecting to elasticsearch...", "url", cfg.ElasticURL)
	es, err := elasticsearch.NewClient(elasticsearch.Config{
		Addresses: []string{cfg.ElasticURL},
	})
	if err != nil {
		return fmt.Errorf("elastic client error: %w", err)
	}

	res, err := es.Info()
	if err != nil {
		return fmt.Errorf("elastic info error: %w", err)
	}
	res.Body.Close()

	// 3. Setup Index
	slog.Info("Setting up index...", "index", cfg.ElasticIndex)
	res, err = es.Indices.Exists([]string{cfg.ElasticIndex})
	if err != nil {
		return err
	}
	if res.StatusCode == 200 {
		slog.Info("Deleting existing index...")
		res, _ := es.Indices.Delete([]string{cfg.ElasticIndex})
		res.Body.Close()
	} else {
		res.Body.Close()
	}

	res, err = es.Indices.Create(
		cfg.ElasticIndex,
		es.Indices.Create.WithBody(strings.NewReader(mappingAndSettings)),
	)
	if err != nil {
		return fmt.Errorf("create index error: %w", err)
	}
	if res.IsError() {
		return fmt.Errorf("create index failed: %s", res.String())
	}
	res.Body.Close()

	// Optimize index for bulk loading (disable refresh and replicas)
	slog.Info("Optimizing index for bulk load...")
	res, err = es.Indices.PutSettings(strings.NewReader(`{"refresh_interval": "-1", "number_of_replicas": 0}`), es.Indices.PutSettings.WithIndex(cfg.ElasticIndex))
	if err != nil {
		return err
	}
	res.Body.Close()

	// 4. Fetch Kitab IDs
	slog.Info("Fetching distinct kitab IDs...")
	ids, err := pgRepo.ListKitabIDs(ctx)
	if err != nil {
		return fmt.Errorf("list kitab ids: %w", err)
	}
	slog.Info("Found kitab to index", "count", len(ids))

	// 5. Setup Bulk Indexer
	bi, err := esutil.NewBulkIndexer(esutil.BulkIndexerConfig{
		Index:         cfg.ElasticIndex,
		Client:        es,
		NumWorkers:    cfg.SyncWorkers * 2, // Bulk workers
		FlushBytes:    5e6,                 // 5MB
		FlushInterval: 5 * time.Second,
	})
	if err != nil {
		return fmt.Errorf("bulk indexer error: %w", err)
	}

	var countSuccessful uint64
	var countFailed uint64

	// 6. Start Sync Workers
	slog.Info("Starting streaming workers...", "workers", cfg.SyncWorkers)

	jobs := make(chan int, len(ids))
	for _, id := range ids {
		jobs <- id
	}
	close(jobs)

	// Semaphore to limit parallel postgres streaming
	sem := make(chan struct{}, cfg.SyncWorkers)
	errChan := make(chan error, len(ids))

	for id := range jobs {
		sem <- struct{}{} // Acquire token
		go func(kitabID int) {
			defer func() { <-sem }() // Release token

			// Stream from postgres
			err := pgRepo.StreamKontenChunked(ctx, kitabID, cfg.SyncChunkSize, func(s model.SearchResult) error {
				doc := map[string]any{
					"kitab_id":     s.KitabID,
					"section_id":   s.SectionID,
					"nomor_bagian": s.NomorBagian,
					"kategori":     s.Kategori,
					"judul":        s.Judul,
					"penulis":      s.Penulis,
					"publisher":    s.Publisher,
					"isi_teks":     s.IsiTeks,
				}

				data, err := json.Marshal(doc)
				if err != nil {
					return fmt.Errorf("marshal doc: %w", err)
				}

				return bi.Add(
					ctx,
					esutil.BulkIndexerItem{
						Action:     "index",
						DocumentID: fmt.Sprintf("%d_%d", s.KitabID, s.SectionID),
						Body:       bytes.NewReader(data),
						OnSuccess: func(ctx context.Context, item esutil.BulkIndexerItem, res esutil.BulkIndexerResponseItem) {
							atomic.AddUint64(&countSuccessful, 1)
						},
						OnFailure: func(ctx context.Context, item esutil.BulkIndexerItem, res esutil.BulkIndexerResponseItem, err error) {
							atomic.AddUint64(&countFailed, 1)
							if err != nil {
								slog.Error("Bulk format error", "err", err)
							} else {
								slog.Error("Bulk error", "type", res.Error.Type, "reason", res.Error.Reason)
							}
						},
					},
				)
			})
			if err != nil {
				slog.Error("Failed to stream kitab", "kitab_id", kitabID, "error", err)
				errChan <- err
			}
		}(id)
	}

	// Wait for all postgres streaming workers
	for i := 0; i < cfg.SyncWorkers; i++ {
		sem <- struct{}{}
	}

	// Wait for bulk indexer to finish
	slog.Info("All PostgreSQL reads complete. Waiting for Elasticsearch bulk indexer to finish...")
	if err := bi.Close(ctx); err != nil {
		return fmt.Errorf("bulk indexer close error: %w", err)
	}

	// Restore original settings
	slog.Info("Restoring index settings (refresh_interval: 1s, replicas: 1)...")
	res, err = es.Indices.PutSettings(strings.NewReader(`{"refresh_interval": "1s", "number_of_replicas": 1}`), es.Indices.PutSettings.WithIndex(cfg.ElasticIndex))
	if err != nil {
		return err
	}
	res.Body.Close()

	if failed := atomic.LoadUint64(&countFailed); failed > 0 {
		slog.Warn("Synchronization finished with some failures", "successful", atomic.LoadUint64(&countSuccessful), "failed", failed)
	} else {
		slog.Info("Synchronization completed successfully!", "indexed", atomic.LoadUint64(&countSuccessful))
	}

	return nil
}
