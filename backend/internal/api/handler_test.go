package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"listingsearch/internal/search"
)

var now = time.Date(2026, 9, 10, 0, 0, 0, 0, time.UTC)

func fixture() []search.Listing {
	d := search.Date{Time: time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)}
	return []search.Listing{
		{ID: "a", City: "Springfield", Price: 400000, Bedrooms: 2, ListedDate: d, Description: "Pet friendly condo"},
		{ID: "b", City: "Fairfax", Price: 500000, Bedrooms: 3, ListedDate: d, Description: "Fenced yard"},
		{ID: "c", City: "Reston", Price: 600000, Bedrooms: 4, ListedDate: d, Description: "Near the Town Center, pets welcome"},
	}
}

func get(t *testing.T, query string) *httptest.ResponseRecorder {
	t.Helper()
	h := NewHandler(fixture(), func() time.Time { return now })
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/listings/search?"+query, nil))
	return rec
}

type errorBody struct {
	Errors []FieldError `json:"errors"`
}

func TestValidationErrors(t *testing.T) {
	tests := []struct {
		query      string
		wantFields []string
	}{
		{"minPrice=abc", []string{"minPrice"}},
		{"minPrice=NaN", []string{"minPrice"}},
		{"minPrice=-1", []string{"minPrice"}},
		{"maxPrice=-1", []string{"maxPrice"}},
		{"minBedrooms=2.5", []string{"minBedrooms"}},
		{"minBedrooms=-1", []string{"minBedrooms"}},
		{"targetBudget=0", []string{"targetBudget"}},
		{"minPrice=500000&maxPrice=300000", []string{"maxPrice"}},
		{"page=0", []string{"page"}},
		{"pageSize=0", []string{"pageSize"}},
		{"pageSize=-1", []string{"pageSize"}},
		{"pageSize=51", []string{"pageSize"}},
		{"city=Atlantis", []string{"city"}},
		{"page=99", []string{"page"}},
		{"minPrice=abc&pageSize=0&city=Atlantis", []string{"minPrice", "pageSize", "city"}},
	}
	for _, tt := range tests {
		t.Run(tt.query, func(t *testing.T) {
			rec := get(t, tt.query)
			if rec.Code != http.StatusUnprocessableEntity {
				t.Fatalf("status = %d, want 422; body %s", rec.Code, rec.Body)
			}
			var body errorBody
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			fields := []string{}
			for _, e := range body.Errors {
				fields = append(fields, e.Field)
			}
			if !reflect.DeepEqual(fields, tt.wantFields) {
				t.Errorf("fields = %v, want %v; body %s", fields, tt.wantFields, rec.Body)
			}
		})
	}
}

func TestUnknownCityListsKnownCities(t *testing.T) {
	rec := get(t, "city=Atlantis")
	if !strings.Contains(rec.Body.String(), "Fairfax, Reston, Springfield") {
		t.Errorf("message should list known cities; body %s", rec.Body)
	}
}

func TestPagePastEndReportsLastPage(t *testing.T) {
	rec := get(t, "page=99")
	if !strings.Contains(rec.Body.String(), "(1)") {
		t.Errorf("message should include totalPages; body %s", rec.Body)
	}
}

func TestValidQueryWithNoMatchesIs200(t *testing.T) {
	rec := get(t, "city=Fairfax&keyword=pool")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body %s", rec.Code, rec.Body)
	}
	if !strings.Contains(rec.Body.String(), `"results":[]`) || !strings.Contains(rec.Body.String(), `"total":0`) {
		t.Errorf("want empty results array and total 0; body %s", rec.Body)
	}
}

func TestCityMatchIgnoresCaseAndWhitespace(t *testing.T) {
	rec := get(t, "city=%20FAIRFAX%20")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"total":1`) {
		t.Errorf("status %d body %s", rec.Code, rec.Body)
	}
}

func TestHappyPath(t *testing.T) {
	rec := get(t, "targetBudget=500000&pageSize=2")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d; body %s", rec.Code, rec.Body)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q", ct)
	}
	var res search.Result
	if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
		t.Fatal(err)
	}
	if res.Page != 1 || res.PageSize != 2 || res.Total != 3 || res.TotalPages != 2 || len(res.Results) != 2 {
		t.Fatalf("unexpected envelope: %+v", res)
	}
	if res.Results[0].ID != "b" || res.Results[0].Score != 97.0 || res.Results[1].ID != "a" {
		t.Errorf("unexpected ranking: %+v", res.Results)
	}
}

func TestErrorMessagesAreNotHTMLEscaped(t *testing.T) {
	rec := get(t, "minPrice=500000&maxPrice=300000")
	if !strings.Contains(rec.Body.String(), "must be >= minPrice") {
		t.Errorf("want literal >= in body; got %s", rec.Body)
	}
}
