package repository

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/elastic/go-elasticsearch/v8/typedapi/core/search"
	"github.com/elastic/go-elasticsearch/v8/typedapi/types"
	"github.com/elastic/go-elasticsearch/v8/typedapi/types/enums/operator"
	"github.com/elastic/go-elasticsearch/v8/typedapi/types/enums/textquerytype"
	"github.com/kasjfulk/kutub-syamilah/internal/model"
)

// ElasticRepo implements KitabRepository.SearchKonten using Elasticsearch.
// All other methods delegate to the embedded PostgresRepo.
type ElasticRepo struct {
	*PostgresRepo // embedded for delegation
	es            *elasticsearch.TypedClient
	indexName     string
}

// NewElastic creates a new ElasticRepo.
func NewElastic(pg *PostgresRepo, es *elasticsearch.TypedClient, indexName string) *ElasticRepo {
	return &ElasticRepo{
		PostgresRepo: pg,
		es:           es,
		indexName:    indexName,
	}
}

// SearchKonten performs Arabic full-text search using Elasticsearch.
func (r *ElasticRepo) SearchKonten(ctx context.Context, f SearchFilter) ([]model.SearchResult, int, error) {
	offset := (f.Page - 1) * f.Limit

	// Priority 0: Exact phrase match (isi_teks) - Tier 1
	mpq := types.MatchPhraseQuery{
		Query:    f.Query,
		Boost:    func(f float32) *float32 { return &f }(100.0),
		Analyzer: func(s string) *string { return &s }("arabic_custom"),
	}

	// Priority 1: Match on content (isi_teks) with high weight - Tier 2
	mq := types.MatchQuery{
		Query:              f.Query,
		Boost:              func(f float32) *float32 { return &f }(10.0), // Main content match
		Analyzer:           func(s string) *string { return &s }("arabic_custom"),
		MinimumShouldMatch: func(s string) *string { return &s }("75%"),
	}

	// Priority 2: Multi-match on metadata (judul, penulis, etc.) as "similarity" - Tier 3
	mmq := types.MultiMatchQuery{
		Query:              f.Query,
		Fields:             []string{"judul^4", "penulis^2", "kategori^1.5", "publisher^1"},
		Analyzer:           func(s string) *string { return &s }("arabic_custom"),
		Type:               &textquerytype.Bestfields,
		Operator:           &operator.Or,
		MinimumShouldMatch: func(s string) *string { return &s }("75%"),
	}

	// Apply fuzziness if requested
	if f.Fuzzy {
		mq.Fuzziness = func(s string) *string { return &s }("AUTO")
		mmq.Fuzziness = "AUTO"
	}

	query := types.Query{
		Bool: &types.BoolQuery{
			Should: []types.Query{
				{MatchPhrase: map[string]types.MatchPhraseQuery{"isi_teks": mpq}},
				{Match:       map[string]types.MatchQuery{"isi_teks": mq}},
				{MultiMatch:  &mmq},
			},
		},
	}

	// Apply filter
	var finalQuery types.Query
	if f.Kategori != "" {
		finalQuery = types.Query{
			Bool: &types.BoolQuery{
				Must: []types.Query{query},
				Filter: []types.Query{
					{
						Term: map[string]types.TermQuery{
							"kategori": {Value: f.Kategori},
						},
					},
				},
			},
		}
	} else {
		finalQuery = query
	}

	// Highlight
	var highlight *types.Highlight
	if f.Highlight {
		highlight = &types.Highlight{
			PreTags:  []string{"<mark>"},
			PostTags: []string{"</mark>"},
			Fields: map[string]types.HighlightField{
				"isi_teks": {
					FragmentSize:      func(i int) *int { return &i }(150),
					NumberOfFragments: func(i int) *int { return &i }(1),
				},
			},
		}
	}

	// Execute search
	res, err := r.es.Search().
		Index(r.indexName).
		Request(&search.Request{
			Query:     &finalQuery,
			Highlight: highlight,
			From:      func(i int) *int { return &i }(offset),
			Size:      func(i int) *int { return &i }(f.Limit),
		}).
		Do(ctx)

	if err != nil {
		return nil, 0, fmt.Errorf("elasticsearch search: %w", err)
	}

	total := int(res.Hits.Total.Value)

	items := make([]model.SearchResult, 0, len(res.Hits.Hits))
	for _, hit := range res.Hits.Hits {
		var s model.SearchResult

		if err := json.Unmarshal(hit.Source_, &s); err != nil {
			return nil, 0, fmt.Errorf("json unmarshal _source: %w", err)
		}

		if hit.Score_ != nil {
			s.Rank = float64(*hit.Score_)
		}

		if f.Highlight && len(hit.Highlight) > 0 {
			if snippets, ok := hit.Highlight["isi_teks"]; ok && len(snippets) > 0 {
				s.Highlight = snippets[0]
			}
		}

		// truncate isi_teks
		if len(s.IsiTeks) > 100 {
			runes := []rune(s.IsiTeks)
			if len(runes) > 100 {
				s.IsiTeks = string(runes[:100]) + "..."
			}
		}

		items = append(items, s)
	}

	return items, total, nil
}
