import { describe, expect, it } from 'vitest'
import { readFileSync } from 'node:fs'
import { ZodError } from 'zod'

import { PageOutOfRangeError, parseListings, round1, score, search, type Params, type Result } from '../src/search.ts'
import { fixture, listing, TODAY } from './factories.ts'

const DATA = new URL('../data/sample_listings.json', import.meta.url)
const ids = (r: Result) => r.results.map((l) => l.id)
const params = (p: Partial<Params> = {}): Params => ({ page: 1, pageSize: 10, ...p })

describe('parseListings', () => {
  it('loads the sample data', () => {
    const ls = parseListings(readFileSync(DATA, 'utf8'))
    expect(ls).toHaveLength(12)
    expect([ls[0].id, ls[0].price, ls[0].listedDate]).toEqual(['A1', 450000, '2026-08-29'])
  })

  it('rejects a non-ISO date', () => {
    const record = { ...JSON.parse(readFileSync(DATA, 'utf8'))[0], listedDate: '09/01/2026' }
    try {
      parseListings(JSON.stringify([record]))
      expect.unreachable()
    } catch (e) {
      expect(e).toBeInstanceOf(ZodError)
      expect((e as ZodError).issues.map((i) => i.path.at(-1))).toEqual(['listedDate'])
    }
  })
})

describe('score', () => {
  it.each([
    // 0.7*1.0 + 0.3*(1 - 9/90) = 0.97
    ['exact budget', 500000, '2026-09-01', 500000, 97],
    // 0.7*0.9 + 0.3*(1 - 30/90) = 0.83
    ['10% under', 450000, '2026-08-11', 500000, 83],
    ['10% over same as under', 550000, '2026-08-11', 500000, 83],
    ['clamps to 0', 1100000, '2026-05-01', 500000, 0],
    ['no budget', 500000, '2026-09-01', undefined, 90],
    ['future date', 500000, '2026-09-20', undefined, 100],
  ])('%s', (_, price, listed, budget, want) => {
    expect(score(listing('x', price, listed), budget, TODAY)).toBe(want)
  })
})

describe('search', () => {
  // No budget and equal dates means equal scores, so order is price asc.
  it.each<[string, Partial<Params>, string[]]>([
    ['no filters', {}, ['a', 'b', 'c']],
    ['minPrice inclusive', { minPrice: 500000 }, ['b', 'c']],
    ['maxPrice inclusive', { maxPrice: 500000 }, ['a', 'b']],
    ['minBedrooms', { minBedrooms: 3 }, ['b', 'c']],
    ['city normalized', { city: '  fairFAX ' }, ['b']],
    ['keyword case', { keyword: 'PET' }, ['a', 'c']],
    ['blank keyword', { keyword: '   ' }, ['a', 'b', 'c']],
    ['combined', { minPrice: 450000, keyword: 'pet' }, ['c']],
    ['no matches', { keyword: 'pool' }, []],
  ])('filters: %s', (_, p, want) => {
    const result = search(fixture(), params(p), TODAY)
    expect(ids(result)).toEqual(want)
    expect(result.total).toBe(want.length)
  })

  it('ranks by score', () => {
    // b is on budget; a and c are both 20% off, so they tie and a wins on price.
    expect(ids(search(fixture(), params({ targetBudget: 500000 }), TODAY))).toEqual(['b', 'a', 'c'])
  })

  it('breaks ties by price, then id', () => {
    const ls = [listing('X', 600000), listing('Y', 500000), listing('W', 500000)]
    expect(ids(search(ls, params(), TODAY))).toEqual(['W', 'Y', 'X'])
  })

  it('rounds scores half up', () => {
    // Half up, matching the Go and Python versions (Python's round() would give 0.2 / 83.2).
    expect([round1(0.25), round1(83.25)]).toEqual([0.3, 83.3])
  })
})

const makeListings = (n: number) =>
  Array.from({ length: n }, (_, i) => listing(`L${String(i).padStart(2, '0')}`, 100000 + i * 1000))

describe('pagination', () => {
  it.each([
    ['first page', 5, 1, 2, ['L00', 'L01'], 3],
    ['partial last page', 5, 3, 2, ['L04'], 3],
    ['exact multiple', 4, 2, 2, ['L02', 'L03'], 2],
    ['no results', 0, 1, 2, [], 0],
  ] as const)('%s', (_, n, page, pageSize, want, totalPages) => {
    const r = search(makeListings(n), params({ page, pageSize }), TODAY)
    expect(ids(r)).toEqual(want)
    expect([r.total, r.totalPages, r.page, r.pageSize]).toEqual([n, totalPages, page, pageSize])
  })

  it('throws PageOutOfRangeError past the end', () => {
    expect(() => search(makeListings(5), params({ page: 4, pageSize: 2 }), TODAY)).toThrowError(
      expect.objectContaining({ name: 'PageOutOfRangeError', totalPages: 3 }),
    )
    expect(() => search(makeListings(5), params({ page: 4, pageSize: 2 }), TODAY)).toThrow(PageOutOfRangeError)
  })

  it.each([{ page: 0 }, { pageSize: 0 }])('rejects invalid paging %o', (p) => {
    expect(() => search(fixture(), params(p), TODAY)).toThrow(RangeError)
  })
})
