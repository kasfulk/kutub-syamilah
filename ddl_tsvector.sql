ALTER TABLE konten_kitab ADD COLUMN IF NOT EXISTS search_vector tsvector GENERATED ALWAYS AS (to_tsvector('arabic'::regconfig, isi_teks)) STORED;
CREATE INDEX IF NOT EXISTS idx_konten_vector_search ON konten_kitab USING gin (search_vector);
