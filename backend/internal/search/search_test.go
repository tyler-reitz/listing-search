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
