package handler

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"log/slog"

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
	
	var page int
	if halStr := chi.URLParam(r, "hal"); halStr != "" {
		p, err := strconv.Atoi(halStr)
		if err != nil || p < 1 {
			writeError(w, http.StatusBadRequest, "BAD_REQUEST", fmt.Sprintf("invalid hal: %s", halStr))
			return
		}
		page = p
	} else {
		page = parseIntDefault(q.Get("page"), 1)
	}

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

	// Dynamic Translation support if 'lang' query parameter is provided.
	// Currently filters on 'id' (Indonesian) and 'en' (English).
	lang := q.Get("lang")
	if lang == "id" || lang == "en" {
		for i := range items {
			translated, err := translateText(r.Context(), items[i].IsiTeks, lang)
			if err != nil {
				slog.LogAttrs(r.Context(), slog.LevelError, "translation failed",
					slog.String("error", err.Error()),
					slog.Int("section_id", items[i].ID),
					slog.String("lang", lang),
				)
				continue
			}
			items[i].IsiTeks = translated
		}
	}

	writeJSON(w, http.StatusOK, model.PaginatedResponse[model.KontenResponse]{
		Data: model.KontenResponse{
			KitabID:   kitab.ID,
			Judul:     kitab.Judul,
			Penulis:   kitab.Penulis,
			Publisher: kitab.Publisher,
			Sections:  items,
		},
		Pagination: model.Pagination{
			Page:       page,
			Limit:      limit,
			Total:      total,
			TotalPages: totalPages(total, limit),
		},
	})
}
