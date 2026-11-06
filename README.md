# Listing Search

Search, rank and paginate property listings. Node API (Express 5, Zod, TypeScript) plus a React/TypeScript UI.

This is the Node port of the Go version on `main`. Same API contract, scoring and tests; the frontend is unchanged. (The exercise allows Python, Go or Java backends; this branch is a comparison, not the submission.)

## Run

Requires Node 22.18+, which runs TypeScript directly (no build step).

```bash
# API on :8080
cd backend && npm install && npm start -- --now 2026-09-05

# UI on :5173, proxies /api to :8080
cd frontend && npm install && npm run dev
```

`--now` pins the clock. The sample listings are from Aug and Sep 2026, so against today's date most are past the 90-day recency window and recency contributes almost nothing. Omit it to use the real time.

Tests: `cd backend && npm test` (type-check: `npm run typecheck`)

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

Why this shape: price is the buyer's hard constraint, so budget fit carries most of the weight, and recency nudges fresh listings up. Both terms are linear and clamped to 0..1, so any score can be checked by hand. The weights and window are named constants in `src/search.ts`.

Ties sort by price ascending, then id, so results are deterministic. Ties compare the rounded score, so listings that display the same score sort as tied.

## Validation

Bad input returns 422 with every problem at once:

```json
{ "errors": [ { "field": "maxPrice", "message": "maxPrice (300000) must be >= minPrice (500000)" } ] }
```

A valid query that matches nothing returns 200 with an empty list. The distinction is deliberate: an unknown city or a page past the end is a client mistake worth reporting, while "no listings match" is a legitimate answer.

`NaN` and `Infinity` are rejected: `z.coerce.number()` turns them into numbers, and Zod 4's `number` refuses non-finite values.

## Design

- `src/search.ts` is pure: filter, score, sort, paginate, with the clock passed in. All ranking logic and its tests live there. Listings are validated with a Zod schema at load time.
- `src/app.ts` declares the query as a Zod schema built inside `createApp`, so it can close over the loaded cities. Field problems, including an unknown city, are reported together; the min/max check runs once every field has parsed.
- Type and range messages are Zod's wording; the `field` names match the Go version.
- The UI does no validation of its own beyond input types. The server owns the rules, so they live in one place. Paging reuses the last submitted filters, not unsubmitted edits in the form.

## Tradeoffs and known gaps

- **No cross-feed dedup.** A1/B7, A2/B8, A3/B9 and A5/B11 are the same properties from two feeds, so each appears twice. With a $450k budget, A1 and B7 rank first and second.
- **No status filter.** All listings are searched, including `pending` (A7).
- **Symmetric budget penalty.** $50k over budget scores the same as $50k under. Real buyers likely tolerate under more than over; this would be a one-line change.
- **City validation is tied to the loaded data.** Fine for a fixed file. With a database this becomes a lookup or goes away.
- **Unknown query params are ignored.** A typo like `minprice` applies no filter. (`z.strictObject` would reject them.) Repeated params are rejected with a 422.
- **Age uses the server's local calendar date,** so a listing's age ticks over at local midnight, not UTC.
- **Rapid paging can race.** Two quick clicks send two requests and the last response wins. An `AbortController` per request would fix it.
- **In-memory, linear scan.** Fine for 12 listings. At scale, filtering moves to an indexed store.
