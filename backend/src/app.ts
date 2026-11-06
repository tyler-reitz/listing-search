import express, { type ErrorRequestHandler } from 'express'
import { z } from 'zod'

import { normalize, PageOutOfRangeError, search, type Listing } from './search.ts'

const MAX_PAGE_SIZE = 50

type FieldError = { field: string; message: string }

// Blank query values ("?minPrice=") mean "not set", not 0.
const blankIsUnset = <T extends z.ZodType>(schema: T) =>
  z.preprocess((v) => (typeof v === 'string' && v.trim() === '' ? undefined : v), schema)

// z.coerce.number() turns "abc" into NaN, and Zod 4 rejects both NaN and Infinity.
const num = () => z.coerce.number()
const int = () => z.coerce.number().int()

function searchQuery(knownCities: Map<string, string>) {
  const cityList = [...knownCities.values()].sort().join(', ')

  // Key order is error order: field problems are all reported together.
  return z
    .object({
      minPrice: blankIsUnset(num().min(0).optional()),
      maxPrice: blankIsUnset(num().min(0).optional()),
      targetBudget: blankIsUnset(num().positive().optional()),
      minBedrooms: blankIsUnset(int().min(0).optional()),
      page: blankIsUnset(int().min(1).default(1)),
      pageSize: blankIsUnset(int().min(1).max(MAX_PAGE_SIZE).default(10)),
      city: blankIsUnset(z.string().optional()).refine((c) => !c || knownCities.has(normalize(c)), {
        error: (issue) => `unknown city "${String(issue.input).trim()}"; known cities: ${cityList}`,
      }),
      keyword: blankIsUnset(z.string().optional()),
    })
    .superRefine((q, ctx) => {
      if (q.minPrice !== undefined && q.maxPrice !== undefined && q.minPrice > q.maxPrice) {
        ctx.addIssue({
          code: 'custom',
          path: ['maxPrice'],
          message: `maxPrice (${q.maxPrice}) must be >= minPrice (${q.minPrice})`,
        })
      }
    })
}

const toErrors = (e: z.ZodError): FieldError[] =>
  e.issues.map((i) => ({ field: String(i.path[0] ?? ''), message: i.message }))

export function createApp(listings: Listing[], today: () => string) {
  const knownCities = new Map<string, string>()
  for (const l of listings) {
    if (!knownCities.has(normalize(l.city))) knownCities.set(normalize(l.city), l.city.trim())
  }
  const SearchQuery = searchQuery(knownCities)

  const app = express()
  app.disable('x-powered-by')

  app.get('/api/listings/search', (req, res) => {
    const q = SearchQuery.safeParse(req.query)
    if (!q.success) {
      res.status(422).json({ errors: toErrors(q.error) })
      return
    }
    try {
      res.json(search(listings, q.data, today()))
    } catch (e) {
      if (!(e instanceof PageOutOfRangeError)) throw e
      res.status(422).json({ errors: [{ field: 'page', message: e.message }] })
    }
  })

  const onError: ErrorRequestHandler = (err, _req, res, _next) => {
    console.error(err)
    res.status(500).json({ errors: [{ field: '', message: 'internal error' }] })
  }
  app.use(onError)

  return app
}
