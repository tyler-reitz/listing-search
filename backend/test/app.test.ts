import request from 'supertest'
import { describe, expect, it } from 'vitest'

import { createApp } from '../src/app.ts'
import { fixture, TODAY } from './factories.ts'

const app = createApp(fixture(), () => TODAY)
const get = (query: string) => request(app).get(`/api/listings/search?${query}`)
const fields = (body: { errors: { field: string }[] }) => body.errors.map((e) => e.field)

describe('validation', () => {
  it.each([
    ['minPrice=abc', ['minPrice']],
    ['minPrice=NaN', ['minPrice']],
    ['minPrice=Infinity', ['minPrice']],
    ['minPrice=-1', ['minPrice']],
    ['maxPrice=-1', ['maxPrice']],
    ['minBedrooms=2.5', ['minBedrooms']],
    ['minBedrooms=-1', ['minBedrooms']],
    ['targetBudget=0', ['targetBudget']],
    ['minPrice=500000&maxPrice=300000', ['maxPrice']],
    ['page=0', ['page']],
    ['pageSize=0', ['pageSize']],
    ['pageSize=-1', ['pageSize']],
    ['pageSize=51', ['pageSize']],
    ['city=Atlantis', ['city']],
    ['page=99', ['page']],
    ['minPrice=abc&pageSize=0&city=Atlantis', ['minPrice', 'pageSize', 'city']],
    // Repeated params arrive as arrays; reject rather than silently pick one.
    ['city=Fairfax&city=Reston', ['city']],
  ])('%s', async (query, want) => {
    const res = await get(query)
    expect(res.status).toBe(422)
    expect(fields(res.body)).toEqual(want)
  })

  it('lists known cities for an unknown one', async () => {
    expect((await get('city=Atlantis')).text).toContain('Fairfax, Reston, Springfield')
  })

  it('reports the last page when past the end', async () => {
    expect((await get('page=99')).text).toContain('(1)')
  })

  it('writes a readable min/max message', async () => {
    const res = await get('minPrice=500000&maxPrice=300000')
    expect(res.body.errors[0].message).toBe('maxPrice (300000) must be >= minPrice (500000)')
  })
})

describe('success', () => {
  it('returns 200 with an empty list for a valid query with no matches', async () => {
    const res = await get('city=Fairfax&keyword=pool')
    expect(res.status).toBe(200)
    expect(res.text).toContain('"results":[]')
    expect(res.body.total).toBe(0)
  })

  it('matches city ignoring case and whitespace', async () => {
    const res = await get('city=%20FAIRFAX%20')
    expect([res.status, res.body.total]).toEqual([200, 1])
  })

  it('treats blank params as unset', async () => {
    const res = await get('minPrice=&page=&city=')
    expect([res.status, res.body.total, res.body.page]).toEqual([200, 3, 1])
  })

  it('returns the ranked page', async () => {
    const res = await get('targetBudget=500000&pageSize=2')
    expect(res.status).toBe(200)
    expect(res.headers['content-type']).toMatch(/^application\/json/)
    const { page, pageSize, total, totalPages, results } = res.body
    expect({ page, pageSize, total, totalPages }).toEqual({ page: 1, pageSize: 2, total: 3, totalPages: 2 })
    expect(results.map((r: { id: string; score: number }) => [r.id, r.score])).toEqual([['b', 97], ['a', 83]])
    expect(results[0].listedDate).toBe('2026-09-01')
  })
})
