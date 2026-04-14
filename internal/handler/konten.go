package handler

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/kasjfulk/kutub-syamilah/internal/model"
	"github.com/kasjfulk/kutub-syamilah/internal/repository"
)

// GetKonten handles GET /v1/kitab/{id}/konten.
// Returns paginated content sections for a given kitab, including parent
// kitab metadata (kitab_id, judul) per the API contract.
func (h *Handler) GetKonten(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "BAD_REQUEST", fmt.Sprintf("invalid kitab id: %s", idStr))
		return
	}

	q := r.URL.Query()
	page := parseIntDefault(q.Get("page"), 1)
	limit := clampInt(parseIntDefault(q.Get("limit"), 20), 1, 100)

	// Verify the kitab exists before fetching konten.
	kitab, err := h.svc.GetKitab(r.Context(), id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			writeError(w, http.StatusNotFound, "NOT_FOUND", fmt.Sprintf("Kitab with id %d not found", id))
			return
		}
		writeError(w, http.StatusInternalServerError, "INTERNAL_SERVER_ERROR", err.Error())
		return
	}

	items, total, err := h.svc.GetKonten(r.Context(), id, page, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_SERVER_ERROR", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, model.PaginatedResponse[model.KontenResponse]{
		Data: model.KontenResponse{
			KitabID:  kitab.ID,
			Judul:    kitab.Judul,
			Sections: items,
		},
		Pagination: model.Pagination{
			Page:       page,
			Limit:      limit,
			Total:      total,
			TotalPages: totalPages(total, limit),
		},
	})
}
