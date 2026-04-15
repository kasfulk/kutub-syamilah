package repository

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/elastic/go-elasticsearch/v8/typedapi/core/search"
	"github.com/elastic/go-elasticsearch/v8/typedapi/types"
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

	// Base multi_match query
	query := types.Query{
		MultiMatch: &types.MultiMatchQuery{
			Query:  f.Query,
			Fields: []string{"judul^3", "kategori^2", "isi_teks"},
		},
	}

	// Apply fuzziness if requested
	if f.Fuzzy {
		query.MultiMatch.Fuzziness = "AUTO"
	}

	// Apply filter using function_score format if we need to apply weighting
	// or bool query for strict filtering. We use a bool query.
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

	// Setup Highlight
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

	// Perform the search
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
	
	// Map results
	items := make([]model.SearchResult, 0, len(res.Hits.Hits))
	for _, hit := range res.Hits.Hits {
		var s model.SearchResult
		if err := json.Unmarshal(hit.Source_, &s); err != nil {
			return nil, 0, fmt.Errorf("json unmarshal _source: %w", err)
		}

		s.Rank = float64(*hit.Score_)

		if f.Highlight && len(hit.Highlight) > 0 {
			if snippets, ok := hit.Highlight["isi_teks"]; ok && len(snippets) > 0 {
				s.Highlight = snippets[0]
			}
		}

		// Fallback for IsiTeks logic — same as Postgres LEFT(isi_teks, 50)
		if len(s.IsiTeks) > 100 { // Assume safe truncation for Arabic 100 runes
			runes := []rune(s.IsiTeks)
			if len(runes) > 100 {
				s.IsiTeks = string(runes[:100]) + "..."
			}
		}

		items = append(items, s)
	}

	return items, total, nil
}
