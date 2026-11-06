package search

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// Params holds validated search input. Nil pointers mean "filter not set",
// which is different from a filter of 0.
type Params struct {
	MinPrice     *float64
	MaxPrice     *float64
	MinBedrooms  *int
	City         string
	Keyword      string
	TargetBudget *float64
	Page         int
	PageSize     int
}

type ScoredListing struct {
	Listing
	Score float64 `json:"score"`
}

type Result struct {
	Results    []ScoredListing `json:"results"`
	Page       int             `json:"page"`
	PageSize   int             `json:"pageSize"`
	Total      int             `json:"total"`
	TotalPages int             `json:"totalPages"`
}

type PageOutOfRangeError struct {
	Page, TotalPages int
}

func (e *PageOutOfRangeError) Error() string {
	return fmt.Sprintf("page %d is past the last page (%d)", e.Page, e.TotalPages)
}

func Normalize(s string) string { return strings.ToLower(strings.TrimSpace(s)) }

func Search(listings []Listing, p Params, now time.Time) (Result, error) {
	if p.Page < 1 || p.PageSize < 1 {
		return Result{}, fmt.Errorf("invalid paging: page=%d pageSize=%d", p.Page, p.PageSize)
	}

	scored := []ScoredListing{}
	for _, l := range listings {
		if matches(l, p) {
			scored = append(scored, ScoredListing{Listing: l, Score: Score(l, p.TargetBudget, now)})
		}
	}

	sort.SliceStable(scored, func(i, j int) bool {
		a, b := scored[i], scored[j]
		if a.Score != b.Score {
			return a.Score > b.Score
		}
		if a.Price != b.Price {
			return a.Price < b.Price
		}
		return a.ID < b.ID
	})

	total := len(scored)
	totalPages := (total + p.PageSize - 1) / p.PageSize
	if total > 0 && p.Page > totalPages {
		return Result{}, &PageOutOfRangeError{Page: p.Page, TotalPages: totalPages}
	}
	start := min((p.Page-1)*p.PageSize, total)
	end := min(start+p.PageSize, total)

	return Result{
		Results:    scored[start:end],
		Page:       p.Page,
		PageSize:   p.PageSize,
		Total:      total,
		TotalPages: totalPages,
	}, nil
}

func matches(l Listing, p Params) bool {
	if p.MinPrice != nil && l.Price < *p.MinPrice {
		return false
	}
	if p.MaxPrice != nil && l.Price > *p.MaxPrice {
		return false
	}
	if p.MinBedrooms != nil && l.Bedrooms < *p.MinBedrooms {
		return false
	}
	if c := Normalize(p.City); c != "" && Normalize(l.City) != c {
		return false
	}
	if k := Normalize(p.Keyword); k != "" && !strings.Contains(strings.ToLower(l.Description), k) {
		return false
	}
	return true
}
