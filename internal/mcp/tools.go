package mcp

import (
	mcpLib "github.com/mark3labs/mcp-go/mcp"
)

// SearchKitabTool defines the only MCP tool: Arabic full-text search across all manuscripts.
var SearchKitabTool = mcpLib.NewTool("search_kitab",
	mcpLib.WithDescription("Full-text search across all Arabic manuscript content using Elasticsearch. "+
		"The 'query' parameter MUST be in Arabic script (core block U+0600–U+06FF; "+
		"Presentation Forms and other Arabic blocks are accepted and NFKC-normalized to core). "+
		"Before calling this tool, translate any non-Arabic user request into Arabic search terms first; "+
		"do not call with Latin/transliterated keywords. "+
		"Returns up to 5 strongly ranked results with full text content and highlight snippets. "+
		"Supports Arabic stemming, normalization, and optional fuzzy matching."),
	mcpLib.WithString("query",
		mcpLib.Required(),
		mcpLib.Description("Arabic search terms (required, Arabic script only)"),
	),
	mcpLib.WithString("kategori",
		mcpLib.Description("Filter results by exact Arabic category name"),
	),
	mcpLib.WithNumber("page",
		mcpLib.Description("Page number (1-based)"),
		mcpLib.DefaultNumber(1),
		mcpLib.Min(1),
	),
	mcpLib.WithNumber("limit",
		mcpLib.Description("Number of results per page"),
		mcpLib.DefaultNumber(5),
		mcpLib.Min(1),
		mcpLib.Max(100),
	),
	mcpLib.WithBoolean("fuzzy",
		mcpLib.Description("Enable fuzzy matching for typos and variations"),
		mcpLib.DefaultBool(false),
	),
	mcpLib.WithBoolean("highlight",
		mcpLib.Description("Include highlighted snippets in results"),
		mcpLib.DefaultBool(true),
	),
)