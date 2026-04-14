package repository

// Raw SQL constants for full control over PostgreSQL-specific features
// (GIN, websearch_to_tsquery, ts_rank). No ORM overhead on hot paths.
// All queries use parameterized placeholders ($1, $2, ...) to prevent SQL injection.
const (
	// listKitabSQL filters by Arabic judul (ILIKE partial) and/or
	// Arabic kategori (exact byte match, = is sufficient since Arabic has no case).
	// NULL parameter means "no filter" — elegant parameterized optional filtering.
	listKitabSQL = `
		SELECT id, judul, kategori, path_orig
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
		SELECT id, judul, kategori, path_orig
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

	// searchKontenSQL uses websearch_to_tsquery for safe natural-language
	// Arabic input (handles spaces, special chars, no syntax errors).
	// Leverages the existing idx_konten_search GIN index on isi_teks.
	searchKontenSQL = `
		SELECT
			dk.id, dk.judul, dk.kategori,
			kk.id, kk.nomor_bagian, kk.isi_teks,
			ts_rank(to_tsvector('arabic', kk.isi_teks),
					websearch_to_tsquery('arabic', $1)) AS rank
		FROM konten_kitab kk
		JOIN daftar_kitab dk ON dk.id = kk.kitab_id
		WHERE
			to_tsvector('arabic', kk.isi_teks) @@ websearch_to_tsquery('arabic', $1)
			AND ($2::text IS NULL OR dk.kategori = $2)
		ORDER BY rank DESC
		LIMIT $3 OFFSET $4`

	countSearchSQL = `
		SELECT COUNT(*)
		FROM konten_kitab kk
		JOIN daftar_kitab dk ON dk.id = kk.kitab_id
		WHERE
			to_tsvector('arabic', kk.isi_teks) @@ websearch_to_tsquery('arabic', $1)
			AND ($2::text IS NULL OR dk.kategori = $2)`

	listKategoriSQL = `
		SELECT DISTINCT kategori
		FROM daftar_kitab
		WHERE kategori IS NOT NULL
		ORDER BY kategori`
)
