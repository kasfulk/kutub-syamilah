package repository

// Raw SQL constants for full control over PostgreSQL-specific features
// (GIN, websearch_to_tsquery, ts_rank). No ORM overhead on hot paths.
// All queries use parameterized placeholders ($1, $2, ...) to prevent SQL injection.
const (
	// listKitabSQL filters by Arabic judul (ILIKE partial) and/or
	// Arabic kategori (exact byte match, = is sufficient since Arabic has no case).
	// NULL parameter means "no filter" — elegant parameterized optional filtering.
	listKitabSQL = `
		SELECT id, judul, kategori, path_orig, penulis, publisher
		FROM daftar_kitab
		WHERE
			($1::text IS NULL OR judul ILIKE '%' || $1 || '%')
			AND ($2::text IS NULL OR kategori = $2)
		ORDER BY id
		LIMIT $3 OFFSET $4`

	countKitabSQL = `
		SELECT COUNT(*)
		FROM daftar_kitab
		WHERE
			($1::text IS NULL OR judul ILIKE '%' || $1 || '%')
			AND ($2::text IS NULL OR kategori = $2)`

	getKitabByIDSQL = `
		SELECT id, judul, kategori, path_orig, penulis, publisher
		FROM daftar_kitab
		WHERE id = $1`

	listKontenSQL = `
		SELECT id, kitab_id, nomor_bagian, isi_teks
		FROM konten_kitab
		WHERE kitab_id = $1
		ORDER BY nomor_bagian
		LIMIT $2 OFFSET $3`

	countKontenSQL = `
		SELECT COUNT(*) FROM konten_kitab WHERE kitab_id = $1`

	// searchKontenBaseSQL uses websearch_to_tsquery for safe natural-language.
	// We use the materialized search_vector column to eliminate ranking CPU bottlenecks.
	// Kept as fallback — primary search is now handled by ElasticRepo.
	searchKontenBaseSQL = `
		SELECT
			dk.id,
			dk.judul,
			dk.kategori,
			dk.penulis,
			dk.publisher,
			kk.id, kk.nomor_bagian, LEFT(kk.isi_teks, 50) AS isi_teks,
			ts_rank(kk.search_vector, websearch_to_tsquery('arabic'::regconfig, $1)) AS rank
		FROM konten_kitab kk
		JOIN daftar_kitab dk ON dk.id = kk.kitab_id
		WHERE
			kk.search_vector @@ websearch_to_tsquery('arabic'::regconfig, $1)`

	countSearchBaseSQL = `
		SELECT COUNT(*)
		FROM konten_kitab kk
		WHERE
			kk.search_vector @@ websearch_to_tsquery('arabic'::regconfig, $1)`

	listKategoriSQL = `
		SELECT DISTINCT kategori
		FROM daftar_kitab
		WHERE kategori IS NOT NULL
		ORDER BY kategori`

	// streamKontenChunkedSQL uses keyset pagination (WHERE id > $last_id) instead of
	// OFFSET to avoid sequential scans on large tables. This is the primary ingestion
	// query for the Elasticsearch sync pipeline.
	//
	// Requires index: CREATE INDEX idx_konten_kitab_id_id ON konten_kitab(kitab_id, id);
	//
	// Parameters: $1=kitab_id, $2=last_id (start from 0), $3=chunk_size
	streamKontenChunkedSQL = `
		SELECT
			kk.id,
			kk.kitab_id,
			kk.nomor_bagian,
			kk.isi_teks,
			dk.judul,
			dk.kategori,
			dk.penulis,
			dk.publisher
		FROM konten_kitab kk
		JOIN daftar_kitab dk ON dk.id = kk.kitab_id
		WHERE kk.kitab_id = $1
		  AND kk.id > $2
		ORDER BY kk.id
		LIMIT $3`

	// listKitabIDsSQL returns all distinct kitab_id values for distributing
	// sync work across worker goroutines.
	listKitabIDsSQL = `
		SELECT DISTINCT kitab_id
		FROM konten_kitab
		ORDER BY kitab_id`
)
