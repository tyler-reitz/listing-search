package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"listingsearch/internal/search"
)

const (
	defaultPageSize = 10
	maxPageSize     = 50
)

type FieldError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

type errorResponse struct {
	Errors []FieldError `json:"errors"`
}

func NewHandler(listings []search.Listing, now func() time.Time) http.Handler {
	cities := newCityIndex(listings)
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/listings/search", func(w http.ResponseWriter, r *http.Request) {
		p, errs := parseParams(r.URL.Query(), cities)
		if len(errs) > 0 {
			writeJSON(w, http.StatusUnprocessableEntity, errorResponse{errs})
			return
		}

		res, err := search.Search(listings, p, now())
		var pageErr *search.PageOutOfRangeError
		switch {
		case errors.As(err, &pageErr):
			writeJSON(w, http.StatusUnprocessableEntity, errorResponse{[]FieldError{{"page", pageErr.Error()}}})
		case err != nil:
			log.Printf("search: %v", err)
			writeJSON(w, http.StatusInternalServerError, errorResponse{[]FieldError{{"", "internal error"}}})
		default:
			writeJSON(w, http.StatusOK, res)
		}
	})
	return mux
}

// parseParams validates every param and returns all problems at once rather
// than stopping at the first.
func parseParams(q url.Values, cities cityIndex) (search.Params, []FieldError) {
	var errs []FieldError
	add := func(field, msg string) { errs = append(errs, FieldError{field, msg}) }

	p := search.Params{Page: 1, PageSize: defaultPageSize}
	p.MinPrice = optFloat(q, "minPrice", add)
	p.MaxPrice = optFloat(q, "maxPrice", add)
	p.TargetBudget = optFloat(q, "targetBudget", add)
	p.MinBedrooms = optInt(q, "minBedrooms", add)

	if p.MinPrice != nil && *p.MinPrice < 0 {
		add("minPrice", "minPrice must be >= 0")
	}
	if p.MaxPrice != nil && *p.MaxPrice < 0 {
		add("maxPrice", "maxPrice must be >= 0")
	}
	if p.MinBedrooms != nil && *p.MinBedrooms < 0 {
		add("minBedrooms", "minBedrooms must be >= 0")
	}
	if p.TargetBudget != nil && *p.TargetBudget <= 0 {
		add("targetBudget", "targetBudget must be > 0")
	}
	if p.MinPrice != nil && p.MaxPrice != nil && *p.MinPrice > *p.MaxPrice {
		add("maxPrice", fmt.Sprintf("maxPrice (%s) must be >= minPrice (%s)", fmtNum(*p.MaxPrice), fmtNum(*p.MinPrice)))
	}

	if v := optInt(q, "page", add); v != nil {
		if *v < 1 {
			add("page", "page must be >= 1")
		} else {
			p.Page = *v
		}
	}
	if v := optInt(q, "pageSize", add); v != nil {
		if *v < 1 || *v > maxPageSize {
			add("pageSize", fmt.Sprintf("pageSize must be between 1 and %d", maxPageSize))
		} else {
			p.PageSize = *v
		}
	}

	if city := q.Get("city"); search.Normalize(city) != "" {
		if !cities.keys[search.Normalize(city)] {
			add("city", fmt.Sprintf("unknown city %q; known cities: %s", strings.TrimSpace(city), strings.Join(cities.names, ", ")))
		}
		p.City = city
	}
	p.Keyword = q.Get("keyword")

	return p, errs
}

func optFloat(q url.Values, name string, add func(string, string)) *float64 {
	s := strings.TrimSpace(q.Get(name))
	if s == "" {
		return nil
	}
	// ParseFloat accepts "NaN" and "Inf", which would poison comparisons.
	v, err := strconv.ParseFloat(s, 64)
	if err != nil || math.IsNaN(v) || math.IsInf(v, 0) {
		add(name, fmt.Sprintf("%s must be a number, got %q", name, s))
		return nil
	}
	return &v
}

func optInt(q url.Values, name string, add func(string, string)) *int {
	s := strings.TrimSpace(q.Get(name))
	if s == "" {
		return nil
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		add(name, fmt.Sprintf("%s must be a whole number, got %q", name, s))
		return nil
	}
	return &v
}

// fmtNum avoids %g, which prints 3000000 as 3e+06.
func fmtNum(v float64) string { return strconv.FormatFloat(v, 'f', -1, 64) }

type cityIndex struct {
	keys  map[string]bool
	names []string
}

func newCityIndex(listings []search.Listing) cityIndex {
	idx := cityIndex{keys: map[string]bool{}}
	for _, l := range listings {
		k := search.Normalize(l.City)
		if !idx.keys[k] {
			idx.keys[k] = true
			idx.names = append(idx.names, strings.TrimSpace(l.City))
		}
	}
	sort.Strings(idx.names)
	return idx
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		log.Printf("write response: %v", err)
	}
}
