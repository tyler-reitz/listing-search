package main

import (
	"flag"
	"log"
	"net/http"
	"os"
	"time"

	"listingsearch/internal/api"
	"listingsearch/internal/search"
)

func main() {
	dataPath := flag.String("data", "data/sample_listings.json", "path to listings JSON")
	addr := flag.String("addr", ":8080", "listen address")
	pinNow := flag.String("now", "", "pin the clock to YYYY-MM-DD (the sample data is from Aug-Sep 2026)")
	flag.Parse()

	f, err := os.Open(*dataPath)
	if err != nil {
		log.Fatalf("open data: %v", err)
	}
	listings, err := search.Load(f)
	f.Close()
	if err != nil {
		log.Fatal(err)
	}

	clock := time.Now
	if *pinNow != "" {
		pinned, err := time.Parse("2006-01-02", *pinNow)
		if err != nil {
			log.Fatalf("bad -now: %v", err)
		}
		clock = func() time.Time { return pinned }
	}

	log.Printf("serving %d listings on %s", len(listings), *addr)
	log.Fatal(http.ListenAndServe(*addr, api.NewHandler(listings, clock)))
}
