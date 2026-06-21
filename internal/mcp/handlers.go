package mcp

import (
	"context"
	"encoding/json"

	mcpLib "github.com/mark3labs/mcp-go/mcp"
	"golang.org/x/text/unicode/norm"

	"github.com/kasjfulk/kutub-syamilah/internal/model"
	"github.com/kasjfulk/kutub-syamilah/internal/repository"
	"github.com/kasjfulk/kutub-syamilah/internal/service"
)

// Server holds the MCP server's dependencies.
type Server struct {
	svc service.Service
}

// NewServer creates a new MCP Server backed by the given service.
func NewServer(svc service.Service) *Server {
	return &Server{svc: svc}
}

// containsArabic reports whether s contains any rune in the Arabic core
// block (U+0600–U+06FF: ا–ي, hamzah, alif madda, harakat, numbers).
//
// Forms-B (U+FE70–U+FEFF) / Forms-A (U+FB50–U+FDFF) are handled upstream by
// NFKC normalization in HandleSearchKitab before this check, so they reach
// here already collapsed to core chars. The core-only check is intentional:
// the live index stores only core Arabic (probe confirmed 0 docs in the
// Forms/Supplement/Extended-A ranges).
func containsArabic(s string) bool {
	for _, r := range s {
		if r >= 0x0600 && r <= 0x06FF {
			return true
		}
	}
	return false
}

// HandleSearchKitab handles the search_kitab tool. Rejects non-Arabic queries
// to force clients to supply Arabic search terms before searching.
func (s *Server) HandleSearchKitab(ctx context.Context, req mcpLib.CallToolRequest) (*mcpLib.CallToolResult, error) {
	query, err := req.RequireString("query")
	if err != nil {
		return mcpLib.NewToolResultError(err.Error()), nil
	}

	// NFKC collapses Presentation Forms (U+FB50–U+FEFF) and other compatibility
	// decompositions into the core Arabic block — required so a Forms query
	// (e.g. pasted from a legacy PDF) matches documents stored in core form.
	// Verified empirically: U+FE8D → U+0627, U+FDF2 → U+0627+U+0644+U+0644+U+0647.
	// Core and Supplement input are unchanged.
	query = norm.NFKC.String(query)

	if !containsArabic(query) {
		return mcpLib.NewToolResultError(
			"query must be in Arabic script. " +
				"Translate the request to Arabic first, then call search_kitab with Arabic keywords."), nil
	}

	filter := repository.SearchFilter{
		Query:     query,
		Kategori:  req.GetString("kategori", ""),
		Page:      int(req.GetFloat("page", 1)),
		Limit:     int(req.GetFloat("limit", 5)),
		Fuzzy:     req.GetFloat("fuzzy", 0) != 0,
		Highlight: req.GetFloat("highlight", 1) != 0,
	}
	if filter.Page < 1 {
		filter.Page = 1
	}
	if filter.Limit < 1 || filter.Limit > 100 {
		filter.Limit = 5
	}

	results, total, err := s.svc.Search(ctx, filter)
	if err != nil {
		return mcpLib.NewToolResultErrorFromErr("search_kitab", err), nil
	}

	resp := model.PaginatedResponse[[]model.SearchResult]{
		Data: results,
		Pagination: model.Pagination{
			Page:       filter.Page,
			Limit:      filter.Limit,
			Total:      total,
			TotalPages: (total + filter.Limit - 1) / filter.Limit,
		},
	}
	return jsonResult(resp)
}

// --- Helpers ---

// jsonResult serializes v to JSON and wraps it in an MCP text result.
func jsonResult(v any) (*mcpLib.CallToolResult, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return mcpLib.NewToolResultErrorFromErr("json", err), nil
	}
	return mcpLib.NewToolResultText(string(b)), nil
}