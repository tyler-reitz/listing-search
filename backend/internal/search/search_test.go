package search

import (
	"errors"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"
)

var now = time.Date(2026, 9, 10, 0, 0, 0, 0, time.UTC)

func ptr[T any](v T) *T { return &v }

func date(s string) Date {
	t, err := time.Parse(dateLayout, s)
	if err != nil {
		panic(err)
	}
	return Date{t}
}

func TestLoadSampleData(t *testing.T) {
	f, err := os.Open("../../data/sample_listings.json")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	ls, err := Load(f)
	if err != nil {
		t.Fatal(err)
	}
	if len(ls) != 12 {
		t.Fatalf("got %d listings, want 12", len(ls))
	}
	first := ls[0]
	if first.ID != "A1" || first.Price != 450000 || first.ListedDate.Format(dateLayout) != "2026-08-29" {
		t.Errorf("first listing decoded wrong: %+v", first)
	}
}

func TestLoadRejectsBadDate(t *testing.T) {
	_, err := Load(strings.NewReader(`[{"id":"x","listedDate":"09/01/2026"}]`))
	if err == nil {
		t.Fatal("want error for non-ISO date")
	}
}

func TestScore(t *testing.T) {
	tests := []struct {
		name   string
		price  float64
		listed string
		budget *float64
		want   float64
	}{
		// 0.7*1.0 + 0.3*(1 - 9/90) = 0.97
		{"exact budget, 9 days old", 500000, "2026-09-01", ptr(500000.0), 97.0},
		// 0.7*0.9 + 0.3*(1 - 30/90) = 0.83
		{"10% under budget, 30 days old", 450000, "2026-08-11", ptr(500000.0), 83.0},
		{"10% over budget scores the same as under", 550000, "2026-08-11", ptr(500000.0), 83.0},
		{"far over budget and stale clamps to 0", 1100000, "2026-05-01", ptr(500000.0), 0},
		{"no budget uses recency only", 500000, "2026-09-01", nil, 90.0},
		{"future listing date counts as brand new", 500000, "2026-09-20", nil, 100.0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := Listing{Price: tt.price, ListedDate: date(tt.listed)}
			if got := Score(l, tt.budget, now); got != tt.want {
				t.Errorf("Score = %v, want %v", got, tt.want)
			}
		})
	}
}

func fixture() []Listing {
	d := date("2026-09-01")
	return []Listing{
		{ID: "a", City: "Springfield", Price: 400000, Bedrooms: 2, ListedDate: d, Description: "Pet friendly condo"},
		{ID: "b", City: "Fairfax", Price: 500000, Bedrooms: 3, ListedDate: d, Description: "Fenced yard"},
		{ID: "c", City: "Reston", Price: 600000, Bedrooms: 4, ListedDate: d, Description: "Near the Town Center, pets welcome"},
	}
}

func ids(r Result) []string {
	out := []string{}
	for _, l := range r.Results {
		out = append(out, l.ID)
	}
	return out
}

func TestSearchFilters(t *testing.T) {
	// No budget and equal dates means equal scores, so order is price asc.
	tests := []struct {
		name string
		p    Params
		want []string
	}{
		{"no filters", Params{}, []string{"a", "b", "c"}},
		{"minPrice is inclusive", Params{MinPrice: ptr(500000.0)}, []string{"b", "c"}},
		{"maxPrice is inclusive", Params{MaxPrice: ptr(500000.0)}, []string{"a", "b"}},
		{"minBedrooms", Params{MinBedrooms: ptr(3)}, []string{"b", "c"}},
		{"city ignores case and whitespace", Params{City: "  fairFAX "}, []string{"b"}},
		{"keyword ignores case", Params{Keyword: "PET"}, []string{"a", "c"}},
		{"blank keyword is ignored", Params{Keyword: "   "}, []string{"a", "b", "c"}},
		{"filters combine", Params{MinPrice: ptr(450000.0), Keyword: "pet"}, []string{"c"}},
		{"no matches", Params{Keyword: "pool"}, []string{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.p.Page, tt.p.PageSize = 1, 10
			res, err := Search(fixture(), tt.p, now)
			if err != nil {
				t.Fatal(err)
			}
			if got := ids(res); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ids = %v, want %v", got, tt.want)
			}
			if res.Total != len(tt.want) {
				t.Errorf("Total = %d, want %d", res.Total, len(tt.want))
			}
		})
	}
}

func TestSearchRanksByScore(t *testing.T) {
	// b is on budget; a and c are both 20% off, so they tie and a wins on price.
	res, err := Search(fixture(), Params{TargetBudget: ptr(500000.0), Page: 1, PageSize: 10}, now)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := ids(res), []string{"b", "a", "c"}; !reflect.DeepEqual(got, want) {
		t.Errorf("ids = %v, want %v", got, want)
	}
}

func TestSearchTieBreaksByPriceThenID(t *testing.T) {
	d := date("2026-09-01")
	ls := []Listing{
		{ID: "X", Price: 600000, ListedDate: d},
		{ID: "Y", Price: 500000, ListedDate: d},
		{ID: "W", Price: 500000, ListedDate: d},
	}
	res, err := Search(ls, Params{Page: 1, PageSize: 10}, now)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := ids(res), []string{"W", "Y", "X"}; !reflect.DeepEqual(got, want) {
		t.Errorf("ids = %v, want %v", got, want)
	}
}

func makeListings(n int) []Listing {
	ls := make([]Listing, n)
	for i := range ls {
		ls[i] = Listing{ID: fmt.Sprintf("L%02d", i), Price: float64(100000 + i*1000), ListedDate: date("2026-09-01")}
	}
	return ls
}

func TestSearchPagination(t *testing.T) {
	tests := []struct {
		name           string
		n, page, size  int
		want           []string
		wantTotalPages int
	}{
		{"first page", 5, 1, 2, []string{"L00", "L01"}, 3},
		{"partial last page", 5, 3, 2, []string{"L04"}, 3},
		{"exact multiple, last page", 4, 2, 2, []string{"L02", "L03"}, 2},
		{"no results", 0, 1, 2, []string{}, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := Search(makeListings(tt.n), Params{Page: tt.page, PageSize: tt.size}, now)
			if err != nil {
				t.Fatal(err)
			}
			if got := ids(res); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ids = %v, want %v", got, tt.want)
			}
			if res.Total != tt.n || res.TotalPages != tt.wantTotalPages || res.Page != tt.page || res.PageSize != tt.size {
				t.Errorf("got total=%d totalPages=%d page=%d pageSize=%d", res.Total, res.TotalPages, res.Page, res.PageSize)
			}
		})
	}
}

func TestSearchPagePastEnd(t *testing.T) {
	_, err := Search(makeListings(5), Params{Page: 4, PageSize: 2}, now)
	var pe *PageOutOfRangeError
	if !errors.As(err, &pe) || pe.TotalPages != 3 {
		t.Fatalf("want PageOutOfRangeError with TotalPages 3, got %v", err)
	}
}

func TestSearchRejectsInvalidPaging(t *testing.T) {
	for _, p := range []Params{{Page: 0, PageSize: 10}, {Page: 1, PageSize: 0}} {
		if _, err := Search(fixture(), p, now); err == nil {
			t.Errorf("want error for %+v", p)
		}
	}
}
