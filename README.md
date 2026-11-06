# Listing Search

Search, rank and paginate property listings. Python API (FastAPI, Pydantic) plus a React/TypeScript UI.

This is the Python port of the Go version on `main`. Same API contract, scoring and tests; the frontend is unchanged.

## Run

Requires [uv](https://docs.astral.sh/uv/) (Python 3.12+) and Node 20.19+ (or 22.12+).

```bash
# API on :8080
cd backend && uv run python -m app --now 2026-09-05

# UI on :5173, proxies /api to :8080
cd frontend && npm install && npm run dev
```

`--now` pins the clock. The sample listings are from Aug and Sep 2026, so against today's date most are past the 90-day recency window and recency contributes almost nothing. Omit it to use the real time.

Tests: `cd backend && uv run pytest`

## API

`GET /api/listings/search`

| Param | Rule |
|---|---|
| minPrice, maxPrice | number >= 0; minPrice <= maxPrice |
| minBedrooms | integer >= 0 |
| city | must be a city in the data (case and surrounding spaces ignored) |
| keyword | case-insensitive substring of the description |
| targetBudget | number > 0 |
| page | integer >= 1, default 1 |
| pageSize | integer 1 to 50, default 10 |

Returns `{results, page, pageSize, total, totalPages}`. Each result is the listing plus `score`.

## Scoring

```
budgetFit = max(0, 1 - |price - targetBudget| / targetBudget)
recency   = max(0, 1 - ageDays / 90)
score     = 100 * (0.7 * budgetFit + 0.3 * recency)     rounded to 1 decimal
```

Without a targetBudget, score is `100 * recency`.

Example: target $500,000, a $450,000 listing that is 30 days old. budgetFit is 0.9, recency is 0.667, so the score is 100 * (0.63 + 0.2) = 83.0.

Why this shape: price is the buyer's hard constraint, so budget fit carries most of the weight, and recency nudges fresh listings up. Both terms are linear and clamped to 0..1, so any score can be checked by hand. The weights and window are named constants in `app/search.py`.

Ties sort by price ascending, then id, so results are deterministic. Ties compare the rounded score, so listings that display the same score sort as tied.

## Validation

Bad input returns 422 with every problem at once:

```json
{ "errors": [ { "field": "maxPrice", "message": "maxPrice (300000) must be >= minPrice (500000)" } ] }
```

A valid query that matches nothing returns 200 with an empty list. The distinction is deliberate: an unknown city or a page past the end is a client mistake worth reporting, while "no listings match" is a legitimate answer.

`NaN` and `inf` are rejected (`allow_inf_nan=False`). `ge=0` alone would let `inf` through.

Scores round half up, like the Go version. Python's built-in `round()` rounds half to even (`round(0.25, 1) == 0.2`).

## Design

- `app/search.py` is pure: filter, score, sort, paginate, with the clock passed in. All ranking logic and its tests live there.
- `app/main.py` declares the query as a Pydantic model (`SearchQuery`), so types, ranges and the min/max check are declarative. An exception handler reshapes Pydantic's 422 into the `errors` format above.
- Two checks need the loaded data or the filtered total, so they run in the endpoint: unknown city and page past the end. Because FastAPI rejects the request before the endpoint runs when a field is invalid, those two are reported only once every field is valid. Likewise the min/max check runs only when `minPrice` itself parsed. (The Go version reports all of these together.)
- Error messages for type and range problems are Pydantic's wording; the `field` names match the Go version.
- The UI does no validation of its own beyond input types. The server owns the rules, so they live in one place. Paging reuses the last submitted filters, not unsubmitted edits in the form.

## Tradeoffs and known gaps

- **No cross-feed dedup.** A1/B7, A2/B8, A3/B9 and A5/B11 are the same properties from two feeds, so each appears twice. With a $450k budget, A1 and B7 rank first and second.
- **No status filter.** All listings are searched, including `pending` (A7).
- **Symmetric budget penalty.** $50k over budget scores the same as $50k under. Real buyers likely tolerate under more than over; this would be a one-line change.
- **City validation is tied to the loaded data.** Fine for a fixed file. With a database this becomes a lookup or goes away.
- **Unknown and repeated query params are not rejected.** A typo like `minprice` is ignored, and only the first of repeated values is used.
- **Age uses the server's local calendar date,** so a listing's age ticks over at local midnight, not UTC.
- **Rapid paging can race.** Two quick clicks send two requests and the last response wins. An `AbortController` per request would fix it.
- **In-memory, linear scan.** Fine for 12 listings. At scale, filtering moves to an indexed store.
