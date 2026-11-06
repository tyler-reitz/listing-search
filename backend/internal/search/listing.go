package search

import (
	"encoding/json"
	"fmt"
	"io"
	"time"
)

const dateLayout = "2006-01-02"

// Date is a calendar date serialized as "YYYY-MM-DD". Parsing at load time means
// a bad date fails startup instead of surfacing mid-request.
type Date struct{ time.Time }

func (d *Date) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	t, err := time.Parse(dateLayout, s)
	if err != nil {
		return fmt.Errorf("listedDate %q: %w", s, err)
	}
	d.Time = t
	return nil
}

func (d Date) MarshalJSON() ([]byte, error) {
	return json.Marshal(d.Format(dateLayout))
}

type Listing struct {
	ID          string  `json:"id"`
	Source      string  `json:"source"`
	Address     string  `json:"address"`
	City        string  `json:"city"`
	State       string  `json:"state"`
	Zip         string  `json:"zip"`
	Price       float64 `json:"price"`
	Bedrooms    int     `json:"bedrooms"`
	Bathrooms   float64 `json:"bathrooms"`
	Sqft        int     `json:"sqft"`
	Latitude    float64 `json:"latitude"`
	Longitude   float64 `json:"longitude"`
	ListedDate  Date    `json:"listedDate"`
	Status      string  `json:"status"`
	Description string  `json:"description"`
}

func Load(r io.Reader) ([]Listing, error) {
	var ls []Listing
	if err := json.NewDecoder(r).Decode(&ls); err != nil {
		return nil, fmt.Errorf("decode listings: %w", err)
	}
	return ls, nil
}
