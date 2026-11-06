import { z } from 'zod'

export const BUDGET_WEIGHT = 0.7
export const RECENCY_WEIGHT = 0.3
export const RECENCY_WINDOW_DAYS = 90

const DAY_MS = 24 * 60 * 60 * 1000

export const ListingSchema = z.object({
  id: z.string(),
  source: z.string(),
  address: z.string(),
  city: z.string(),
  state: z.string(),
  zip: z.string(),
  price: z.number(),
  bedrooms: z.int(),
  bathrooms: z.number(),
  sqft: z.int(),
  latitude: z.number(),
  longitude: z.number(),
  listedDate: z.iso.date(),
  status: z.string(),
  description: z.string(),
})

export type Listing = z.infer<typeof ListingSchema>
export type ScoredListing = Listing & { score: number }

/** Validated search input. undefined means "filter not set", which differs from 0. */
export type Params = {
  minPrice?: number
  maxPrice?: number
  minBedrooms?: number
  city?: string
  keyword?: string
  targetBudget?: number
  page: number
  pageSize: number
}

export type Result = {
  results: ScoredListing[]
  page: number
  pageSize: number
  total: number
  totalPages: number
}

export class PageOutOfRangeError extends Error {
  override name = 'PageOutOfRangeError'
  readonly page: number
  readonly totalPages: number

  constructor(page: number, totalPages: number) {
    super(`page ${page} is past the last page (${totalPages})`)
    this.page = page
    this.totalPages = totalPages
  }
}

export const normalize = (s = '') => s.trim().toLowerCase()

/** Parses and validates listings JSON. A bad date fails here, not mid-request. */
export function parseListings(json: string): Listing[] {
  return z.array(ListingSchema).parse(JSON.parse(json))
}

/**
 * Relevance in 0..100. Both terms are linear and clamped to 0..1: budget fit falls
 * off with relative distance from the target (symmetric over and under), and
 * recency falls to 0 at RECENCY_WINDOW_DAYS. Dates are "YYYY-MM-DD" strings, which
 * Date.parse reads as UTC midnight, so the difference is a whole number of days.
 */
export function score(l: Listing, targetBudget: number | undefined, today: string): number {
  const ageDays = Math.max(0, (Date.parse(today) - Date.parse(l.listedDate)) / DAY_MS)
  const recency = Math.max(0, 1 - ageDays / RECENCY_WINDOW_DAYS)
  if (targetBudget === undefined) return round1(100 * recency)
  const budgetFit = Math.max(0, 1 - Math.abs(l.price - targetBudget) / targetBudget)
  return round1(100 * (BUDGET_WEIGHT * budgetFit + RECENCY_WEIGHT * recency))
}

export const round1 = (x: number) => Math.round(x * 10) / 10

export function search(listings: Listing[], p: Params, today: string): Result {
  if (p.page < 1 || p.pageSize < 1) {
    throw new RangeError(`invalid paging: page=${p.page} pageSize=${p.pageSize}`)
  }

  const scored = listings
    .filter((l) => matches(l, p))
    .map((l) => ({ ...l, score: score(l, p.targetBudget, today) }))
    .sort((a, b) => b.score - a.score || a.price - b.price || (a.id < b.id ? -1 : a.id > b.id ? 1 : 0))

  const total = scored.length
  const totalPages = Math.ceil(total / p.pageSize)
  if (total > 0 && p.page > totalPages) throw new PageOutOfRangeError(p.page, totalPages)
  const start = (p.page - 1) * p.pageSize

  return {
    results: scored.slice(start, start + p.pageSize),
    page: p.page,
    pageSize: p.pageSize,
    total,
    totalPages,
  }
}

function matches(l: Listing, p: Params): boolean {
  const city = normalize(p.city)
  const keyword = normalize(p.keyword)
  return (
    (p.minPrice === undefined || l.price >= p.minPrice) &&
    (p.maxPrice === undefined || l.price <= p.maxPrice) &&
    (p.minBedrooms === undefined || l.bedrooms >= p.minBedrooms) &&
    (!city || normalize(l.city) === city) &&
    (!keyword || l.description.toLowerCase().includes(keyword))
  )
}
