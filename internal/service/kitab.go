// Package service implements the business logic layer for the Kutub Syamilah API.
// It sits between handlers and the repository, providing cache-aside reads with
// singleflight to prevent cache stampedes on expensive Arabic FTS queries.
package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"golang.org/x/sync/singleflight"

	"github.com/kasjfulk/kutub-syamilah/internal/cache"
	"github.com/kasjfulk/kutub-syamilah/internal/model"
	"github.com/kasjfulk/kutub-syamilah/internal/repository"
)

// TTL strategy per endpoint — tuned per data volatility.
// Conservative starting points; adjust based on observed latency.
const (
	ttlKitabList = 5 * time.Minute  // Filtered lists change only on data updates
	ttlKitabByID = 30 * time.Minute // Individual kitab rarely changes
	ttlKonten    = 30 * time.Minute // Arabic text content is static
	ttlSearch    = 2 * time.Minute  // FTS is expensive; short TTL for freshness
	ttlKategori  = 60 * time.Minute // Kategori list changes very infrequently
)

// KitabService implements the cache-aside pattern with singleflight.
// The singleflight.Group ensures that when many concurrent requests all miss
// the same cache key simultaneously, only ONE goroutine hits the database.
// All others wait and share the single result. This is critical for Arabic
// FTS queries which are CPU-heavy on PostgreSQL.
type KitabService struct {
	repo  repository.KitabRepository
	cache cache.Cache
	sf    singleflight.Group // zero value ready — idiomatic "make zero value useful"
}

// NewKitab creates a new KitabService with the given repository and cache.
func NewKitab(repo repository.KitabRepository, c cache.Cache) *KitabService {
	return &KitabService{repo: repo, cache: c}
}

// ListKitab returns a paginated list of kitab with optional Arabic filters.
// Cache-aside: check cache → on miss, fetch from DB → populate cache.
func (s *KitabService) ListKitab(ctx context.Context, f repository.KitabFilter) ([]model.DaftarKitab, int, error) {
	key := cache.KeyListKitab(f.Judul, f.Kategori, f.Page, f.Limit)

	// 1. Cache hit?
	if b, err := s.cache.Get(ctx, key); err == nil {
		var resp paginatedCacheEntry[model.DaftarKitab]
		if err := json.Unmarshal(b, &resp); err == nil {
			return resp.Items, resp.Total, nil
		}
	}

	// 2. singleflight collapses concurrent misses for the same key.
	type result struct {
		items []model.DaftarKitab
		total int
	}
	val, err, _ := s.sf.Do(key, func() (any, error) {
		items, total, err := s.repo.ListKitab(ctx, f)
		if err != nil {
			return nil, err
		}
		// Best-effort cache write; ignore error to not degrade read path.
		entry := paginatedCacheEntry[model.DaftarKitab]{Items: items, Total: total}
		if b, mErr := json.Marshal(entry); mErr == nil {
			_ = s.cache.Set(ctx, key, b, ttlKitabList)
		}
		return result{items: items, total: total}, nil
	})
	if err != nil {
		return nil, 0, err
	}
	r := val.(result)
	return r.items, r.total, nil
}

// GetKitab returns a single kitab by ID. Cache-aside with singleflight.
func (s *KitabService) GetKitab(ctx context.Context, id int) (*model.DaftarKitab, error) {
	key := cache.KeyGetKitab(id)

	// 1. Cache hit?
	if b, err := s.cache.Get(ctx, key); err == nil {
		var k model.DaftarKitab
		if err := json.Unmarshal(b, &k); err == nil {
			return &k, nil
		}
	}

	// 2. singleflight
	val, err, _ := s.sf.Do(key, func() (any, error) {
		k, err := s.repo.GetKitabByID(ctx, id)
		if err != nil {
			return nil, err
		}
		if b, mErr := json.Marshal(k); mErr == nil {
			_ = s.cache.Set(ctx, key, b, ttlKitabByID)
		}
		return k, nil
	})
	if err != nil {
		return nil, err
	}
	return val.(*model.DaftarKitab), nil
}

// GetKonten returns paginated content sections for a given kitab.
// Cache-aside with singleflight.
func (s *KitabService) GetKonten(ctx context.Context, kitabID, page, limit int) ([]model.KontenKitab, int, error) {
	key := cache.KeyKonten(kitabID, page, limit)

	if b, err := s.cache.Get(ctx, key); err == nil {
		var resp paginatedCacheEntry[model.KontenKitab]
		if err := json.Unmarshal(b, &resp); err == nil {
			return resp.Items, resp.Total, nil
		}
	}

	type result struct {
		items []model.KontenKitab
		total int
	}
	val, err, _ := s.sf.Do(key, func() (any, error) {
		items, total, err := s.repo.GetKontenByKitabID(ctx, kitabID, page, limit)
		if err != nil {
			return nil, err
		}
		entry := paginatedCacheEntry[model.KontenKitab]{Items: items, Total: total}
		if b, mErr := json.Marshal(entry); mErr == nil {
			_ = s.cache.Set(ctx, key, b, ttlKonten)
		}
		return result{items: items, total: total}, nil
	})
	if err != nil {
		return nil, 0, err
	}
	r := val.(result)
	return r.items, r.total, nil
}

// Search performs Arabic full-text search across all konten.
// Cache-aside with singleflight — critical for expensive FTS queries.
func (s *KitabService) Search(ctx context.Context, f repository.SearchFilter) ([]model.SearchResult, int, error) {
	key := cache.KeySearch(f.Query, f.Kategori, f.Page, f.Limit)

	if b, err := s.cache.Get(ctx, key); err == nil {
		var resp paginatedCacheEntry[model.SearchResult]
		if err := json.Unmarshal(b, &resp); err == nil {
			return resp.Items, resp.Total, nil
		}
	}

	type result struct {
		items []model.SearchResult
		total int
	}
	val, err, _ := s.sf.Do(key, func() (any, error) {
		items, total, err := s.repo.SearchKonten(ctx, f)
		if err != nil {
			return nil, err
		}
		entry := paginatedCacheEntry[model.SearchResult]{Items: items, Total: total}
		if b, mErr := json.Marshal(entry); mErr == nil {
			_ = s.cache.Set(ctx, key, b, ttlSearch)
		}
		return result{items: items, total: total}, nil
	})
	if err != nil {
		return nil, 0, err
	}
	r := val.(result)
	return r.items, r.total, nil
}

// ListKategori returns a distinct list of all kategori values.
// Cache-aside with singleflight.
func (s *KitabService) ListKategori(ctx context.Context) ([]string, error) {
	key := cache.KeyKategori()

	if b, err := s.cache.Get(ctx, key); err == nil {
		var items []string
		if err := json.Unmarshal(b, &items); err == nil {
			return items, nil
		}
	}

	val, err, _ := s.sf.Do(key, func() (any, error) {
		items, err := s.repo.ListKategori(ctx)
		if err != nil {
			return nil, err
		}
		if b, mErr := json.Marshal(items); mErr == nil {
			_ = s.cache.Set(ctx, key, b, ttlKategori)
		}
		return items, nil
	})
	if err != nil {
		return nil, err
	}
	return val.([]string), nil
}

// paginatedCacheEntry is a helper type for caching paginated results.
// Items and Total are stored together so a single cache hit satisfies the response.
type paginatedCacheEntry[T any] struct {
	Items []T `json:"items"`
	Total int `json:"total"`
}

// Service defines the contract for the handler layer.
// Defined here (consumer side) so handlers depend on the abstraction.
type Service interface {
	ListKitab(ctx context.Context, filter repository.KitabFilter) ([]model.DaftarKitab, int, error)
	GetKitab(ctx context.Context, id int) (*model.DaftarKitab, error)
	GetKonten(ctx context.Context, kitabID, page, limit int) ([]model.KontenKitab, int, error)
	Search(ctx context.Context, filter repository.SearchFilter) ([]model.SearchResult, int, error)
	ListKategori(ctx context.Context) ([]string, error)
}

// Compile-time interface conformance check.
var _ Service = (*KitabService)(nil)

// GetKitabByID is a convenience wrapper for GetKitab to be used in the konten
// handler for checking kitab existence. Exposed to match the naming used by
// the konten handler.
func (s *KitabService) GetKitabByID(ctx context.Context, id int) (*model.DaftarKitab, error) {
	return s.GetKitab(ctx, id)
}

// formatError wraps an error with a descriptive context message.
func formatError(op string, err error) error {
	return fmt.Errorf("service.%s: %w", op, err)
}

// ensure formatError is reachable to satisfy linter (used in future methods).
var _ = formatError
