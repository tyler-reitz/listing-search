import type { Listing } from '../src/search.ts'

export const TODAY = '2026-09-10'

export function listing(id = 'x', price = 500000, listedDate = '2026-09-01', extra: Partial<Listing> = {}): Listing {
  return {
    id, price, listedDate,
    source: 'T', address: '', city: '', state: '', zip: '', bedrooms: 0, bathrooms: 0,
    sqft: 0, latitude: 0, longitude: 0, status: 'active', description: '',
    ...extra,
  }
}

export function fixture(): Listing[] {
  return [
    listing('a', 400000, undefined, { city: 'Springfield', bedrooms: 2, description: 'Pet friendly condo' }),
    listing('b', 500000, undefined, { city: 'Fairfax', bedrooms: 3, description: 'Fenced yard' }),
    listing('c', 600000, undefined, { city: 'Reston', bedrooms: 4, description: 'Near the Town Center, pets welcome' }),
  ]
}
