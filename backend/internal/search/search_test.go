package search

import (
	"os"
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
