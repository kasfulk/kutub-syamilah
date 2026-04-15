package repository

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/sync/errgroup"

	"github.com/kasjfulk/kutub-syamilah/internal/model"
)

// PostgresRepo implements KitabRepository using pgx/v5 with a connection pool.
type PostgresRepo struct {
	pool     *pgxpool.Pool
	logLevel string
}

// NewPostgres creates a new PostgresRepo with the given connection pool.
func NewPostgres(pool *pgxpool.Pool) *PostgresRepo {
	level := os.Getenv("KUTUB_LOG_LEVEL")
	if level == "" {
		level = "1"
	}
	return &PostgresRepo{pool: pool, logLevel: level}
}

func (r *PostgresRepo) debugQuery(query string, args ...any) {
	if r.logLevel == "1" {
		log.Printf("[DEBUG QUERY] %s | ARGS: %v\n", query, args)
	}
}

// ListKitab returns a paginated list of kitab with optional Arabic filters.
// Uses errgroup to fetch COUNT and data rows concurrently — halves latency
// for paginated queries (golang-performance: parallel independent I/O).
func (r *PostgresRepo) ListKitab(ctx context.Context, f KitabFilter) ([]model.DaftarKitab, int, error) {
	g, ctx := errgroup.WithContext(ctx)

	var (
		items []model.DaftarKitab
		total int
	)

	judul := nullableString(f.Judul)
	kategori := nullableString(f.Kategori)
	offset := (f.Page - 1) * f.Limit

	// Fetch data rows concurrently with count.
	g.Go(func() error {
		r.debugQuery(listKitabSQL, judul, kategori, f.Limit, offset)
		rows, err := r.pool.Query(ctx, listKitabSQL, judul, kategori, f.Limit, offset)
		if err != nil {
			return fmt.Errorf("query kitab: %w", err)
		}
		defer rows.Close()
		// Preallocate slice with limit capacity to avoid repeated heap allocations
		// on the hot read path (golang-performance: preallocated slices).
		items = make([]model.DaftarKitab, 0, f.Limit)
		for rows.Next() {
			var k model.DaftarKitab
			if err := rows.Scan(&k.ID, &k.Judul, &k.Kategori, &k.PathOrig); err != nil {
				return fmt.Errorf("scan kitab: %w", err)
			}
			items = append(items, k)
		}
		return rows.Err()
	})

	// Fetch total count concurrently.
	g.Go(func() error {
		r.debugQuery(countKitabSQL, judul, kategori)
		return r.pool.QueryRow(ctx, countKitabSQL, judul, kategori).Scan(&total)
	})

	if err := g.Wait(); err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

// GetKitabByID returns a single kitab by ID or ErrNotFound.
func (r *PostgresRepo) GetKitabByID(ctx context.Context, id int) (*model.DaftarKitab, error) {
	var k model.DaftarKitab
	r.debugQuery(getKitabByIDSQL, id)
	err := r.pool.QueryRow(ctx, getKitabByIDSQL, id).
		Scan(&k.ID, &k.Judul, &k.Kategori, &k.PathOrig)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get kitab %d: %w", id, err)
	}
	return &k, nil
}

// GetKontenByKitabID returns paginated content sections for a given kitab.
// Uses errgroup to fetch COUNT and data rows concurrently.
func (r *PostgresRepo) GetKontenByKitabID(ctx context.Context, kitabID, page, limit int) ([]model.KontenKitab, int, error) {
	g, ctx := errgroup.WithContext(ctx)

	var (
		items []model.KontenKitab
		total int
	)

	offset := (page - 1) * limit

	g.Go(func() error {
		r.debugQuery(listKontenSQL, kitabID, limit, offset)
		rows, err := r.pool.Query(ctx, listKontenSQL, kitabID, limit, offset)
		if err != nil {
			return fmt.Errorf("query konten: %w", err)
		}
		defer rows.Close()
		items = make([]model.KontenKitab, 0, limit)
		for rows.Next() {
			var k model.KontenKitab
			if err := rows.Scan(&k.ID, &k.KitabID, &k.NomorBagian, &k.IsiTeks); err != nil {
				return fmt.Errorf("scan konten: %w", err)
			}
			items = append(items, k)
		}
		return rows.Err()
	})

	g.Go(func() error {
		r.debugQuery(countKontenSQL, kitabID)
		return r.pool.QueryRow(ctx, countKontenSQL, kitabID).Scan(&total)
	})

	if err := g.Wait(); err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

// SearchKonten performs Arabic full-text search using PostgreSQL GIN index.
// Uses errgroup to fetch COUNT and data rows concurrently.
// Uses dynamic query building to avoid Postgres sequential scans caused by parameterizing `IS NULL OR ...` clauses.
func (r *PostgresRepo) SearchKonten(ctx context.Context, f SearchFilter) ([]model.SearchResult, int, error) {
	g, ctx := errgroup.WithContext(ctx)

	var (
		items []model.SearchResult
		total int
	)

	offset := (f.Page - 1) * f.Limit

	// Build dynamic SQL
	searchSQL := searchKontenBaseSQL
	countSQL := countSearchBaseSQL
	var args []any
	args = append(args, f.Query)

	if f.Kategori != "" {
		// PostgreSQL uses 1-based indexing for parameters
		searchSQL += ` AND kk.kitab_id IN (SELECT id FROM daftar_kitab WHERE kategori = $2)`
		countSQL += ` AND kk.kitab_id IN (SELECT id FROM daftar_kitab WHERE kategori = $2)`
		args = append(args, f.Kategori)
		
		// For ordering and pagination
		searchSQL += ` ORDER BY rank DESC LIMIT $3 OFFSET $4`
		args = append(args, f.Limit, offset)
	} else {
		// For ordering and pagination
		searchSQL += ` ORDER BY rank DESC LIMIT $2 OFFSET $3`
		args = append(args, f.Limit, offset)
	}

	g.Go(func() error {
		r.debugQuery(searchSQL, args...)
		rows, err := r.pool.Query(ctx, searchSQL, args...)
		if err != nil {
			return fmt.Errorf("query search: %w", err)
		}
		defer rows.Close()
		items = make([]model.SearchResult, 0, f.Limit)
		for rows.Next() {
			var s model.SearchResult
			if err := rows.Scan(
				&s.KitabID, &s.Judul, &s.Kategori,
				&s.SectionID, &s.NomorBagian, &s.IsiTeks,
				&s.Rank,
			); err != nil {
				return fmt.Errorf("scan search result: %w", err)
			}
			items = append(items, s)
		}
		return rows.Err()
	})

	g.Go(func() error {
		var countArgs []any
		countArgs = append(countArgs, f.Query)
		if f.Kategori != "" {
			countArgs = append(countArgs, f.Kategori)
		}

		r.debugQuery(countSQL, countArgs...)
		return r.pool.QueryRow(ctx, countSQL, countArgs...).Scan(&total)
	})

	if err := g.Wait(); err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

// ListKategori returns a distinct list of all kategori values.
func (r *PostgresRepo) ListKategori(ctx context.Context) ([]string, error) {
	r.debugQuery(listKategoriSQL)
	rows, err := r.pool.Query(ctx, listKategoriSQL)
	if err != nil {
		return nil, fmt.Errorf("query kategori: %w", err)
	}
	defer rows.Close()

	// Start with small capacity — kategori count is bounded and typically small.
	result := make([]string, 0, 32)
	for rows.Next() {
		var k string
		if err := rows.Scan(&k); err != nil {
			return nil, fmt.Errorf("scan kategori: %w", err)
		}
		result = append(result, k)
	}
	return result, rows.Err()
}

// nullableString converts an empty string to nil for SQL NULL parameter handling.
// This enables the "($1::text IS NULL OR ...)" pattern for optional filters.
func nullableString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// StreamKontenChunked streams all konten for a single kitab using keyset pagination.
// fn is called for each row. If fn returns an error, streaming is aborted.
func (r *PostgresRepo) StreamKontenChunked(ctx context.Context, kitabID, chunkSize int, fn func(model.SearchResult) error) error {
	var lastID int = 0

	for {
		r.debugQuery(streamKontenChunkedSQL, kitabID, lastID, chunkSize)
		rows, err := r.pool.Query(ctx, streamKontenChunkedSQL, kitabID, lastID, chunkSize)
		if err != nil {
			return fmt.Errorf("query stream chunk: %w", err)
		}

		count := 0
		for rows.Next() {
			var s model.SearchResult
			s.KitabID = kitabID
			if err := rows.Scan(
				&s.SectionID, // Maps to kk.id
				&s.KitabID,
				&s.NomorBagian,
				&s.IsiTeks,
				&s.Judul,
				&s.Kategori,
			); err != nil {
				rows.Close()
				return fmt.Errorf("scan stream row: %w", err)
			}

			lastID = s.SectionID
			count++

			if err := fn(s); err != nil {
				rows.Close()
				return fmt.Errorf("process stream row: %w", err)
			}
		}

		rows.Close()
		if err := rows.Err(); err != nil {
			return fmt.Errorf("stream row iteration: %w", err)
		}

		// If we fetched fewer rows than chunkSize, we've reached the end
		if count < chunkSize {
			break
		}
	}

	return nil
}

// ListKitabIDs returns all distinct kitab_id values from konten_kitab.
func (r *PostgresRepo) ListKitabIDs(ctx context.Context) ([]int, error) {
	r.debugQuery(listKitabIDsSQL)
	rows, err := r.pool.Query(ctx, listKitabIDsSQL)
	if err != nil {
		return nil, fmt.Errorf("query list kitab ids: %w", err)
	}
	defer rows.Close()

	var ids []int
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan kitab id: %w", err)
		}
		ids = append(ids, id)
	}
	
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list kitab ids iteration: %w", err)
	}

	return ids, nil
}
