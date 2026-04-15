package handler

import (
	"net/http"

	"github.com/kasjfulk/kutub-syamilah/internal/model"
	"github.com/kasjfulk/kutub-syamilah/internal/repository"
)

// Search handles GET /v1/search.
// Performs Arabic full-text search across all konten using PostgreSQL GIN index.
// The "q" parameter is required; "kategori" is optional for narrowing results.
func (h *Handler) Search(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	query := q.Get("q")
	if query == "" {
		writeError(w, http.StatusBadRequest, "BAD_REQUEST", "Query parameter 'q' is required")
		return
	}

	kategori := q.Get("kategori")

	filter := repository.SearchFilter{
		Query:     query,
		Kategori:  kategori,
		Page:      parseIntDefault(q.Get("page"), 1),
		Limit:     clampInt(parseIntDefault(q.Get("limit"), 20), 1, 100),
		Fuzzy:     q.Get("fuzzy") == "true",
		Highlight: q.Get("highlight") != "false", // default to true
	}

	items, total, err := h.svc.Search(r.Context(), filter)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_SERVER_ERROR", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, model.PaginatedResponse[[]model.SearchResult]{
		Data: items,
		Pagination: model.Pagination{
			Page:       filter.Page,
			Limit:      filter.Limit,
			Total:      total,
			TotalPages: totalPages(total, filter.Limit),
		},
	})
}
