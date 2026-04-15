package main

import (
"context"
"fmt"
"log"

"github.com/jackc/pgx/v5/pgxpool"
"github.com/kasjfulk/kutub-syamilah/internal/config"
)

func main() {
cfg, err := config.New()
if err != nil {
log.Fatal(err)
}
ctx := context.Background()
pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
if err != nil {
log.Fatal(err)
}
defer pool.Close()

fmt.Println("Creating index idx_konten_kitab_id...")
_, err = pool.Exec(ctx, "CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_konten_kitab_id ON konten_kitab(kitab_id);")
if err != nil {
log.Fatal(err)
}
fmt.Println("Creating index idx_daftar_kategori...")
_, err = pool.Exec(ctx, "CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_daftar_kategori ON daftar_kitab(kategori);")
if err != nil {
log.Fatal(err)
}

fmt.Println("Done.")
}
