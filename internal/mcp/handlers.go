package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os/exec"
	"strings"

	mcpLib "github.com/mark3labs/mcp-go/mcp"

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

// HandleListKitab handles the list_kitab tool.
func (s *Server) HandleListKitab(ctx context.Context, req mcpLib.CallToolRequest) (*mcpLib.CallToolResult, error) {
	filter := repository.KitabFilter{
		Judul:    req.GetString("judul", ""),
		Kategori: req.GetString("kategori", ""),
		Page:     int(req.GetFloat("page", 1)),
		Limit:    int(req.GetFloat("limit", 20)),
	}
	if filter.Page < 1 {
		filter.Page = 1
	}
	if filter.Limit < 1 || filter.Limit > 100 {
		filter.Limit = 20
	}

	items, total, err := s.svc.ListKitab(ctx, filter)
	if err != nil {
		return mcpLib.NewToolResultErrorFromErr("list_kitab", err), nil
	}

	resp := model.PaginatedResponse[[]model.DaftarKitab]{
		Data: items,
		Pagination: model.Pagination{
			Page:       filter.Page,
			Limit:      filter.Limit,
			Total:      total,
			TotalPages: (total + filter.Limit - 1) / filter.Limit,
		},
	}
	return jsonResult(resp)
}

// HandleGetKitab handles the get_kitab tool.
func (s *Server) HandleGetKitab(ctx context.Context, req mcpLib.CallToolRequest) (*mcpLib.CallToolResult, error) {
	id, err := req.RequireFloat("id")
	if err != nil {
		return mcpLib.NewToolResultError(err.Error()), nil
	}

	kitab, err := s.svc.GetKitab(ctx, int(id))
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return mcpLib.NewToolResultError(fmt.Sprintf("kitab with id %d not found", int(id))), nil
		}
		return mcpLib.NewToolResultErrorFromErr("get_kitab", err), nil
	}

	return jsonResult(model.SingleResponse[*model.DaftarKitab]{Data: kitab})
}

// HandleGetKonten handles the get_konten tool with optional translation.
func (s *Server) HandleGetKonten(ctx context.Context, req mcpLib.CallToolRequest) (*mcpLib.CallToolResult, error) {
	kitabID, err := req.RequireFloat("kitab_id")
	if err != nil {
		return mcpLib.NewToolResultError(err.Error()), nil
	}

	page := int(req.GetFloat("page", 1))
	limit := int(req.GetFloat("limit", 20))
	lang := req.GetString("lang", "")

	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}

	kitab, err := s.svc.GetKitab(ctx, int(kitabID))
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return mcpLib.NewToolResultError(fmt.Sprintf("kitab with id %d not found", int(kitabID))), nil
		}
		return mcpLib.NewToolResultErrorFromErr("get_konten", err), nil
	}

	items, total, err := s.svc.GetKonten(ctx, int(kitabID), page, limit)
	if err != nil {
		return mcpLib.NewToolResultErrorFromErr("get_konten", err), nil
	}

	// Translate if lang is specified and supported.
	if lang == "id" || lang == "en" {
		for i := range items {
			translated, trErr := translateText(ctx, items[i].IsiTeks, lang)
			if trErr != nil {
				slog.LogAttrs(ctx, slog.LevelWarn, "translation failed",
					slog.String("error", trErr.Error()),
					slog.Int("section_id", items[i].ID),
					slog.String("lang", lang),
				)
				continue
			}
			items[i].IsiTeks = translated
		}
	}

	resp := model.PaginatedResponse[model.KontenResponse]{
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
			TotalPages: (total + limit - 1) / limit,
		},
	}
	return jsonResult(resp)
}

// HandleSearchKitab handles the search_kitab tool.
func (s *Server) HandleSearchKitab(ctx context.Context, req mcpLib.CallToolRequest) (*mcpLib.CallToolResult, error) {
	query, err := req.RequireString("query")
	if err != nil {
		return mcpLib.NewToolResultError(err.Error()), nil
	}

	filter := repository.SearchFilter{
		Query:     query,
		Kategori:  req.GetString("kategori", ""),
		Page:      int(req.GetFloat("page", 1)),
		Limit:     int(req.GetFloat("limit", 20)),
		Fuzzy:     req.GetFloat("fuzzy", 0) != 0,
		Highlight: req.GetFloat("highlight", 1) != 0,
	}
	if filter.Page < 1 {
		filter.Page = 1
	}
	if filter.Limit < 1 || filter.Limit > 100 {
		filter.Limit = 20
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

// HandleListKategori handles the list_kategori tool.
func (s *Server) HandleListKategori(ctx context.Context, _ mcpLib.CallToolRequest) (*mcpLib.CallToolResult, error) {
	kategoris, err := s.svc.ListKategori(ctx)
	if err != nil {
		return mcpLib.NewToolResultErrorFromErr("list_kategori", err), nil
	}

	return jsonResult(model.KategoriResponse{
		Data:  kategoris,
		Total: len(kategoris),
	})
}

// HandleSearchElastic handles the search_elastic tool - direct Elasticsearch query without cache.
func (s *Server) HandleSearchElastic(ctx context.Context, req mcpLib.CallToolRequest) (*mcpLib.CallToolResult, error) {
	query, err := req.RequireString("query")
	if err != nil {
		return mcpLib.NewToolResultError(err.Error()), nil
	}

	filter := repository.SearchFilter{
		Query:     query,
		Kategori:  req.GetString("kategori", ""),
		Page:      int(req.GetFloat("page", 1)),
		Limit:     int(req.GetFloat("limit", 20)),
		Fuzzy:     req.GetFloat("fuzzy", 0) != 0,
		Highlight: req.GetFloat("highlight", 1) != 0,
	}
	if filter.Page < 1 {
		filter.Page = 1
	}
	if filter.Limit < 1 || filter.Limit > 100 {
		filter.Limit = 20
	}

	// Direct repository call - bypasses cache for real-time results
	results, total, err := s.svc.SearchDirect(ctx, filter)
	if err != nil {
		return mcpLib.NewToolResultErrorFromErr("search_elastic", err), nil
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

// translateText calls the 'trans' (translate-shell) utility to translate text.
func translateText(ctx context.Context, text, targetLang string) (string, error) {
	if text == "" {
		return "", nil
	}
	cmd := exec.CommandContext(ctx, "trans", "-b", ":"+targetLang, text)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("trans failed: %w (output: %s)", err, string(output))
	}
	return strings.TrimSpace(string(output)), nil
}