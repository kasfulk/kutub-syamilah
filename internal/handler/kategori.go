package handler

import (
	"net/http"

	"github.com/kasjfulk/kutub-syamilah/internal/model"
)

// ListKategori handles GET /v1/kategori.
// Returns a distinct list of all available kategori values.
func (h *Handler) ListKategori(w http.ResponseWriter, r *http.Request) {
	items, err := h.svc.ListKategori(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_SERVER_ERROR", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, model.KategoriResponse{
		Data:  items,
		Total: len(items),
	})
}
