import { readFileSync } from 'node:fs'
import { parseArgs } from 'node:util'
import { z } from 'zod'

import { createApp } from './app.ts'
import { parseListings } from './search.ts'

const { values } = parseArgs({
  options: {
    data: { type: 'string', default: 'data/sample_listings.json' },
    port: { type: 'string', default: '8080' },
    // Pin the clock to YYYY-MM-DD; the sample data is from Aug-Sep 2026.
    now: { type: 'string' },
  },
})

const listings = parseListings(readFileSync(values.data, 'utf8'))
const pinned = values.now === undefined ? undefined : z.iso.date().parse(values.now)
// en-CA formats as YYYY-MM-DD in the server's local time zone.
const today = () => pinned ?? new Date().toLocaleDateString('en-CA')
const port = Number(values.port)

createApp(listings, today).listen(port, () => {
  console.log(`serving ${listings.length} listings on :${port}`)
})
