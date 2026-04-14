// Package cache defines the caching contract and key builders for the
// Kutub Syamilah API. The interface is defined in the consumer package
// (idiomatic Go) so the service layer depends on the abstraction, not
// the concrete Redis implementation.
package cache

import (
	"context"
	"errors"
	"fmt"
	"hash/fnv"
	"time"
)

// ErrCacheMiss is returned by Get when the key does not exist.
// Preallocated sentinel error — zero allocation on the hot path.
var ErrCacheMiss = errors.New("cache miss")

// Cache is the contract consumed by the service layer.
// Implementations: RedisCache (production), NoopCache (tests without Redis).
type Cache interface {
	// Get retrieves a cached value by key. Returns ErrCacheMiss if not found.
	Get(ctx context.Context, key string) ([]byte, error)

	// Set stores a value with the given TTL. Pass 0 to use the default TTL.
	Set(ctx context.Context, key string, val []byte, ttl time.Duration) error

	// Del removes one or more keys from the cache.
	Del(ctx context.Context, keys ...string) error
}

// --- Key builders ---
// All keys share the "ks:" prefix to namespace the application in Redis.
// Go functions enforce consistent formatting and prevent typos.

// KeyListKitab builds the cache key for paginated kitab listings.
func KeyListKitab(judul, kategori string, page, limit int) string {
	return fmt.Sprintf("ks:kitab:list:%s:%s:%d:%d", judul, kategori, page, limit)
}

// KeyGetKitab builds the cache key for a single kitab by ID.
func KeyGetKitab(id int) string {
	return fmt.Sprintf("ks:kitab:%d", id)
}

// KeyKonten builds the cache key for paginated konten sections.
func KeyKonten(kitabID, page, limit int) string {
	return fmt.Sprintf("ks:konten:%d:%d:%d", kitabID, page, limit)
}

// KeySearch builds the cache key for Arabic FTS results.
// The Arabic query string is hashed with FNV-64a to keep key length bounded
// and avoid multi-byte rune issues in Redis key names.
func KeySearch(query, kategori string, page, limit int) string {
	h := fnv.New64a()
	_, _ = h.Write([]byte(query))
	return fmt.Sprintf("ks:search:%x:%s:%d:%d", h.Sum64(), kategori, page, limit)
}

// KeyKategori builds the cache key for the distinct kategori list.
func KeyKategori() string {
	return "ks:kategori"
}

// --- NoopCache ---

// NoopCache is a no-op implementation for tests that don't need Redis.
// Every Get returns ErrCacheMiss; Set and Del are silent no-ops.
type NoopCache struct{}

// Get always returns ErrCacheMiss.
func (NoopCache) Get(_ context.Context, _ string) ([]byte, error) {
	return nil, ErrCacheMiss
}

// Set is a no-op.
func (NoopCache) Set(_ context.Context, _ string, _ []byte, _ time.Duration) error {
	return nil
}

// Del is a no-op.
func (NoopCache) Del(_ context.Context, _ ...string) error {
	return nil
}
