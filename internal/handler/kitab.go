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

// ListKitab handles GET /v1/kitab.
// Supports filtering by Arabic kategori (exact) and judul (partial match).
func (h *Handler) ListKitab(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	filter := repository.KitabFilter{
		Judul:    q.Get("judul"),    // Arabic partial match
		Kategori: q.Get("kategori"), // Arabic exact kategori
		Page:     parseIntDefault(q.Get("page"), 1),
		Limit:    clampInt(parseIntDefault(q.Get("limit"), 20), 1, 100),
	}

	items, total, err := h.svc.ListKitab(r.Context(), filter)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_SERVER_ERROR", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, model.PaginatedResponse[[]model.DaftarKitab]{
		Data: items,
		Pagination: model.Pagination{
			Page:       filter.Page,
			Limit:      filter.Limit,
			Total:      total,
			TotalPages: totalPages(total, filter.Limit),
		},
	})
}

// GetKitab handles GET /v1/kitab/{id}.
func (h *Handler) GetKitab(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "BAD_REQUEST", fmt.Sprintf("invalid kitab id: %s", idStr))
		return
	}

	kitab, err := h.svc.GetKitab(r.Context(), id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			writeError(w, http.StatusNotFound, "NOT_FOUND", fmt.Sprintf("Kitab with id %d not found", id))
			return
		}
		writeError(w, http.StatusInternalServerError, "INTERNAL_SERVER_ERROR", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, model.SingleResponse[*model.DaftarKitab]{Data: kitab})
}
