// Package model defines the domain types for the Kutub Syamilah API.
// These types mirror the PostgreSQL schema and provide JSON serialization
// for the REST API responses.
package model

// DaftarKitab mirrors the daftar_kitab table.
// judul and kategori are Arabic sentences stored as UTF-8 text.
type DaftarKitab struct {
	ID        int    `json:"id"`
	Judul     string `json:"judul"`      // Arabic title sentence
	Kategori  string `json:"kategori"`   // Arabic category name
	PathOrig  string `json:"path_orig"`  // Internal file path (opaque to clients)
	Penulis   string `json:"penulis"`    // Arabic author name
	Publisher string `json:"publisher"`  // Arabic publisher name
}

// KontenKitab mirrors the konten_kitab table.
type KontenKitab struct {
	ID          int    `json:"id"`
	KitabID     int    `json:"kitab_id"`
	NomorBagian int    `json:"nomor_bagian"`
	IsiTeks     string `json:"isi_teks"` // Arabic text content
}

// SearchResult is assembled from Elasticsearch (primary) or a JOIN between
// daftar_kitab and konten_kitab (PostgreSQL fallback).
// Rank is the Elasticsearch _score or PostgreSQL ts_rank.
// Highlight carries the HTML snippet with <mark> tags from Elasticsearch highlight.
type SearchResult struct {
	KitabID     int     `json:"kitab_id"`
	Judul       string  `json:"judul"`
	Kategori    string  `json:"kategori"`
	Penulis     string  `json:"penulis"`
	Publisher   string  `json:"publisher"`
	SectionID   int     `json:"section_id"`
	NomorBagian int     `json:"nomor_bagian"`
	IsiTeks     string  `json:"isi_teks"`
	Rank        float64 `json:"rank"`
	Highlight   string  `json:"highlight,omitempty"` // HTML snippet with <mark> tags from Elasticsearch
}

// KontenResponse wraps konten sections with parent kitab metadata,
// matching the API contract for GET /kitab/:id/konten.
type KontenResponse struct {
	KitabID   int           `json:"kitab_id"`
	Judul     string        `json:"judul"`
	Penulis   string        `json:"penulis"`
	Publisher string        `json:"publisher"`
	Sections  []KontenKitab `json:"sections"`
}

// PaginatedResponse is a generic response wrapper for paginated endpoints.
// Uses Go generics to avoid duplicating pagination boilerplate per type.
type PaginatedResponse[T any] struct {
	Data       T          `json:"data"`
	Pagination Pagination `json:"pagination"`
}

// Pagination holds the pagination metadata included in every paginated response.
type Pagination struct {
	Page       int `json:"page"`
	Limit      int `json:"limit"`
	Total      int `json:"total"`
	TotalPages int `json:"total_pages"`
}

// KategoriResponse wraps the list of distinct kategori values.
type KategoriResponse struct {
	Data  []string `json:"data"`
	Total int      `json:"total"`
}

// SingleResponse wraps a single resource response (e.g. GET /kitab/:id).
type SingleResponse[T any] struct {
	Data T `json:"data"`
}
