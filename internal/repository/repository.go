// Package repository defines the data access contract for the Kutub Syamilah API.
// The interface is defined in the consumer package (idiomatic Go: accept interfaces,
// return structs) so the service layer depends on the abstraction, not the concrete
// pgx implementation.
package repository

import (
	"context"
	"errors"

	"github.com/kasjfulk/kutub-syamilah/internal/model"
)

// Sentinel errors — preallocated for zero-allocation returns on hot paths.
var (
	// ErrNotFound is returned when the requested resource does not exist.
	ErrNotFound = errors.New("resource not found")

	// ErrInvalidInput is returned when query parameters fail validation.
	ErrInvalidInput = errors.New("invalid input")
)

// KitabRepository defines what the service layer needs from the database.
type KitabRepository interface {
	// ListKitab returns a paginated list of kitab, optionally filtered by
	// Arabic judul (partial match) and/or Arabic kategori (exact match).
	ListKitab(ctx context.Context, filter KitabFilter) ([]model.DaftarKitab, int, error)

	// GetKitabByID returns a single kitab by ID or ErrNotFound.
	GetKitabByID(ctx context.Context, id int) (*model.DaftarKitab, error)

	// GetKontenByKitabID returns paginated content sections for a given kitab.
	GetKontenByKitabID(ctx context.Context, kitabID int, page, limit int) ([]model.KontenKitab, int, error)

	// SearchKonten performs Arabic full-text search across all konten,
	// optionally filtered by kategori. Uses PostgreSQL GIN index.
	SearchKonten(ctx context.Context, filter SearchFilter) ([]model.SearchResult, int, error)

	// ListKategori returns a distinct list of all kategori values.
	ListKategori(ctx context.Context) ([]string, error)
}

// KitabFilter holds validated query parameters for listing kitab.
// Arabic strings are passed as-is after UTF-8 validation.
type KitabFilter struct {
	Judul    string // Arabic partial match — used with ILIKE
	Kategori string // Arabic exact match — byte-level equality
	Page     int
	Limit    int
}

// SearchFilter holds validated parameters for Arabic FTS.
type SearchFilter struct {
	Query    string // Arabic search terms for websearch_to_tsquery
	Kategori string // Optional Arabic kategori filter
	Page     int
	Limit    int
}
