# Listing Search: Design Spec

Take-home exercise. A Go REST backend plus a React/TS frontend that search the 12 listings in `sample_listings.json`.

## Scope

In: filters (minPrice, maxPrice, minBedrooms, city, keyword), relevance scoring from targetBudget and recency, pagination, deliberate validation errors, backend tests, README.

Out of v1, noted in the README as known gaps: status filtering (one listing is `pending`) and cross-feed dedup (A1/B7, A2/B8, A3/B9 and A5/B11 are the same properties). All listings are searched as given.

## Layout

```
backend/
  go.mod
  main.go                       load data/sample_listings.json, build mux, listen :8080
  data/sample_listings.json
  internal/search/search.go     Listing, Params, Result, Search(listings, p, now)   (pure, no net/http)
  internal/search/search_test.go
  internal/api/handler.go       parseParams(url.Values, knownCities) -> (Params, []FieldError); JSON responses
  internal/api/handler_test.go  httptest against the real mux
frontend/                       Vite + React + TS; dev server proxies /api to :8080
README.md
```

Stdlib only on the backend. Routing uses Go 1.22+ patterns: `mux.HandleFunc("GET /api/listings/search", ...)`.

`search.Search` takes plain params and an injected `now time.Time`, and returns plain data. All logic lives there so tests are fast and deterministic, and live extensions (dedup, status, sort options) land in one pure function.

## API

`GET /api/listings/search`

| Param | Type | Default | Meaning |
|---|---|---|---|
| minPrice | number >= 0 | none | price >= minPrice |
| maxPrice | number >= 0 | none | price <= maxPrice |
| minBedrooms | integer >= 0 | none | bedrooms >= minBedrooms |
| city | string | none | case-insensitive match after trimming |
| keyword | string | none | case-insensitive substring of `description`; blank is treated as absent |
| targetBudget | number > 0 | none | used in scoring |
| page | integer >= 1 | 1 | |
| pageSize | integer 1..50 | 10 | |

Optional filters are pointers in `Params` so "not set" differs from 0.

Success:

```json
200 {
  "results": [ { ...all listing fields..., "score": 87.4 } ],
  "page": 1, "pageSize": 10, "total": 12, "totalPages": 2
}
```

`totalPages` is `ceil(total / pageSize)`, and 0 when `total` is 0.

## Scoring

```
budgetFit = max(0, 1 - |price - targetBudget| / targetBudget)
ageDays   = max(0, whole days between listedDate and now)
recency   = max(0, 1 - ageDays / 90)
score     = round1(100 * (0.7 * budgetFit + 0.3 * recency))
```

- With no targetBudget: `score = round1(100 * recency)`.
- `round1` rounds to one decimal (`math.Round(x*10)/10`).
- A listing dated in the future gets ageDays 0 (recency 1).
- Weights (0.7, 0.3) and the 90-day window are named constants.

Rationale: budget fit dominates because price is the buyer's hard constraint, and recency nudges fresh listings up. Both terms are linear and clamped to 0..1, so a score is easy to explain by hand. Under-budget and over-budget listings are penalized symmetrically. That's a deliberate simplification, and the README will name it.

Ordering: score descending, then price ascending, then id ascending, using `sort.SliceStable`. Ties compare the rounded score, so listings that display the same score sort as tied.

Pipeline: filter, score, sort, paginate.

## Validation and errors

Every error has one shape:

```json
422 { "errors": [ { "field": "maxPrice", "message": "maxPrice (300000) must be >= minPrice (500000)" } ] }
```

`parseParams` collects all errors before returning, so the response lists every problem.

| Input | Response |
|---|---|
| Non-numeric value for any numeric param | 422 |
| Negative minPrice, maxPrice or minBedrooms | 422 |
| targetBudget <= 0 (it is a divisor) | 422 |
| minPrice > maxPrice | 422 |
| page < 1, pageSize < 1, pageSize > 50 | 422 |
| page > totalPages when total > 0 | 422, message includes totalPages |
| city not in the dataset (after trim, case-insensitive) | 422, message lists known cities |
| Valid filters matching nothing | 200 with `total: 0`, `results: []` |
| Unexpected server failure | 500 with the same `errors` shape |

Principle: bad input gets a 422, while a valid query with no matches gets a 200 with an empty list.

The page-past-end check needs the filtered total, so it runs after filtering. `Search` returns a typed error for it and the handler maps that to a 422.

Tradeoff: validating city against the loaded data couples validation to the dataset. With a real store this becomes a lookup or is dropped.

## Frontend

A single `App.tsx`:

- **Form:** all seven filter inputs (`type="number"` where numeric) and page size. Submitting resets to page 1. Empty fields are left out of the query string.
- **States:** loading; error (each `errors[]` entry shown next to its field); empty ("No listings match these filters."); results.
- **Results table:** address, city, price, beds, listed date, score.
- **Pager:** Prev/Next and "Page x of y". The buttons are disabled at the ends, and changing page refetches with the same filters.

There's no client-side validation beyond input types, because the server owns the rules. There are no frontend tests, since the brief asks for tests of the core logic.

## Tests

Table-driven, `go test ./...`.

**search:**
- each filter on its own
- keyword and city case-insensitivity; city whitespace trimming
- known score values for hand-computed inputs
- no targetBudget means recency only
- recency clamps beyond 90 days and for future dates
- tied scores ordered by price, then id
- pagination: first page, partial last page, exact multiple, total 0, page past end returns the typed error

**api:**
- every 422 row in the table above
- multiple errors returned together
- valid query with no matches returns 200, total 0
- happy-path status code and response shape

Verification check: with the tie test green, remove the tie-breaker and confirm the test fails.

## README

Covers how to run both halves, the scoring formula and its rationale, the validation principle, and the tradeoffs and known gaps (symmetric budget penalty, city validation coupled to the data, no status filter, no dedup).
