// Package levenshtein provides fuzzy address matching using Levenshtein distance.
package levenshtein

import (
	"time"

	"abrg/internal/model"
)

// Levenshtein search configuration constants.
const (
	// queryTimeout is the maximum time allowed for a single database query.
	queryTimeout = 5 * time.Second

	// minPrefixMatchLength is the minimum length of base address required for prefix matching.
	// Addresses shorter than this (e.g., less than city + some characters) are skipped.
	minPrefixMatchLength = 5
)

// SearchParams holds parameters for Levenshtein search.
type SearchParams struct {
	Category         model.Category // Search category (basic, residential, parcel)
	StandardizedAddr string         // Standardized address for computing unmatched parts
	SearchAddr       string         // Address to search for in DB
	Pref             string         // Prefecture code to filter by
	LgCode           string         // Local government code
	MachiazaID       string         // Machiaza ID for filtering
	NormalizedAddr   string         // Basic-normalized address (for extracting unmatched parts)
	Limit            int            // Maximum number of results
}

// hasRegionAnchor reports whether the search is constrained to at least one
// location filter that FindBasicByLevenshtein can actually apply: a
// local-government code or a concrete prefecture code. (A machiaza ID is only
// used together with an lg_code, so it is not an anchor on its own.) Without an
// anchor the query has no location predicate and degrades into a full editdist3
// scan of cache_machiaza
// (600k+ rows), which can exceed queryTimeout under batch contention. See #247.
func (p SearchParams) hasRegionAnchor() bool {
	return p.LgCode != "" || (p.Pref != "" && p.Pref != model.All)
}
