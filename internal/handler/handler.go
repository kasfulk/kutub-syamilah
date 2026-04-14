// Package handler implements the HTTP handlers for the Kutub Syamilah API.
// All handlers follow the same structure: validate → call service → write JSON.
// Errors use sentinel values and are checked with errors.Is.
package handler

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"sync"

	"github.com/kasjfulk/kutub-syamilah/internal/service"
)

// Handler holds the shared dependencies for all HTTP handlers.
type Handler struct {
	svc service.Service
}

// New creates a new Handler with the given service.
func New(svc service.Service) *Handler {
	return &Handler{svc: svc}
}

// --- JSON response helpers ---

// errorResponse is the consistent JSON error body returned by all endpoints.
type errorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}

// jsonBufPool reuses byte buffers across requests to reduce GC pressure
// on high traffic (golang-performance: sync.Pool for hot-path objects).
var jsonBufPool = sync.Pool{
	New: func() any { return new(bytes.Buffer) },
}

// writeJSON serializes v to JSON and writes it to the response.
// Uses sync.Pool to reuse buffers, reducing allocations per request.
func writeJSON(w http.ResponseWriter, status int, v any) {
	buf := jsonBufPool.Get().(*bytes.Buffer)
	defer func() {
		buf.Reset()
		jsonBufPool.Put(buf)
	}()

	if err := json.NewEncoder(buf).Encode(v); err != nil {
		// Log with LogAttrs — zero-alloc when level is disabled
		// (golang-performance: slog.LogAttrs in hot paths).
		slog.LogAttrs(nil, slog.LevelError, "json encode failed",
			slog.String("error", err.Error()),
		)
		http.Error(w, `{"error":"INTERNAL_SERVER_ERROR","message":"failed to encode response"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_, _ = w.Write(buf.Bytes())
}

// writeError writes a consistent JSON error response.
func writeError(w http.ResponseWriter, status int, code, msg string) {
	writeJSON(w, status, errorResponse{Error: code, Message: msg})
}

// --- Query parameter helpers ---

// parseIntDefault parses an integer from a string, returning the default on error.
func parseIntDefault(s string, defaultVal int) int {
	if s == "" {
		return defaultVal
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return defaultVal
	}
	return n
}

// clampInt clamps n to the range [min, max].
func clampInt(n, min, max int) int {
	if n < min {
		return min
	}
	if n > max {
		return max
	}
	return n
}

// totalPages calculates the number of pages for pagination.
func totalPages(total, limit int) int {
	if limit <= 0 {
		return 0
	}
	pages := total / limit
	if total%limit > 0 {
		pages++
	}
	return pages
}
