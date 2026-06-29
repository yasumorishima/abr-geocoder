package levenshtein

import (
	"context"
	"testing"

	"abrg/internal/model"
	"abrg/internal/repository"
)

// stubCityQuerier returns a fixed city result for FindCityByAddress.
type stubCityQuerier struct {
	city *repository.CityResult
}

func (s *stubCityQuerier) FindBasicByLevenshtein(_ context.Context, _ repository.LevenshteinParams) ([]repository.BasicResult, error) {
	return nil, nil
}

func (s *stubCityQuerier) FindBasicByPrefix(_ context.Context, _ repository.PrefixParams) ([]repository.BasicResult, error) {
	return nil, nil
}

func (s *stubCityQuerier) FindCityByAddress(_ context.Context, _ repository.CitySearchParams) (*repository.CityResult, error) {
	return s.city, nil
}

// A fully matched city-level input must yield nil UnmatchedAddress (JSON null),
// never an empty non-nil slice (JSON []). See model.MatchedResult.UnmatchedAddress.
func TestTryFallbackCitySearchByScore_FullMatchYieldsNilUnmatched(t *testing.T) {
	repo := &stubCityQuerier{city: &repository.CityResult{
		LgCode: "342025",
		Pref:   "広島県",
		City:   "福山市",
	}}

	results, err := tryFallbackCitySearchByScore(context.Background(), repo, SearchParams{
		Category:         model.CategoryAll,
		StandardizedAddr: "福山市",
		SearchAddr:       "福山市",
		Limit:            1,
	})
	if err != nil {
		t.Fatalf("tryFallbackCitySearchByScore() error = %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("len(results) = %d, want 1", len(results))
	}
	if results[0].UnmatchedAddress != nil {
		t.Errorf("UnmatchedAddress = %#v, want nil", results[0].UnmatchedAddress)
	}
}

// recordingQuerier records which finders were invoked so a test can assert that
// an anchorless search short-circuits before touching the database. See #247.
type recordingQuerier struct {
	levenshteinCalled bool
	prefixCalled      bool
	prefixLgCode      string
}

func (r *recordingQuerier) FindBasicByLevenshtein(_ context.Context, _ repository.LevenshteinParams) ([]repository.BasicResult, error) {
	r.levenshteinCalled = true
	return nil, nil
}

func (r *recordingQuerier) FindBasicByPrefix(_ context.Context, p repository.PrefixParams) ([]repository.BasicResult, error) {
	r.prefixCalled = true
	r.prefixLgCode = p.LgCode
	return nil, nil
}

func (r *recordingQuerier) FindCityByAddress(_ context.Context, _ repository.CitySearchParams) (*repository.CityResult, error) {
	return nil, nil
}

// Issue #247: a search with no region anchor (no prefecture, local-government, or
// machiaza filter) must not run the unbounded editdist3 / prefix scans, which can
// exceed queryTimeout under batch contention.
func TestSearch_NoRegionAnchorSkipsScan(t *testing.T) {
	repo := &recordingQuerier{}
	results, err := Search(context.Background(), repo, SearchParams{
		Category:   model.CategoryBasic,
		SearchAddr: "no recoverable prefecture or city",
		Pref:       model.All,
		Limit:      1,
	})
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if results != nil {
		t.Errorf("results = %#v, want nil", results)
	}
	if repo.levenshteinCalled {
		t.Error("FindBasicByLevenshtein was called for an anchorless search; want skipped")
	}
	if repo.prefixCalled {
		t.Error("FindBasicByPrefix was called for an anchorless search; want skipped")
	}
}

// A search that carries a region anchor (here a local-government code) must still
// reach the database.
func TestSearch_WithRegionAnchorQueriesDB(t *testing.T) {
	repo := &recordingQuerier{}
	if _, err := Search(context.Background(), repo, SearchParams{
		Category:   model.CategoryBasic,
		SearchAddr: "anchored input",
		LgCode:     "342025",
		Limit:      1,
	}); err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if !repo.levenshteinCalled {
		t.Error("FindBasicByLevenshtein was not called for an anchored search; want invoked")
	}
	if repo.prefixLgCode != "342025" {
		t.Errorf("prefix fallback LgCode = %q, want %q: lg_code must scope the prefix scan", repo.prefixLgCode, "342025")
	}
}

// Issue #247 (review): a machiaza ID without an lg_code is not a usable anchor
// (FindBasicByLevenshtein only applies the machiaza filter together with lg_code),
// so such a search must also skip the scan.
func TestSearch_MachiazaIDAloneIsNotAnchor(t *testing.T) {
	repo := &recordingQuerier{}
	results, err := Search(context.Background(), repo, SearchParams{
		Category:   model.CategoryBasic,
		SearchAddr: "machiaza id without lg code",
		MachiazaID: "0001000",
		Pref:       model.All,
		Limit:      1,
	})
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if results != nil {
		t.Errorf("results = %#v, want nil", results)
	}
	if repo.levenshteinCalled {
		t.Error("FindBasicByLevenshtein was called for a machiaza-only (no lg_code) search; want skipped")
	}
}
