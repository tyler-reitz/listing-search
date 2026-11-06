import { useState } from 'react'

type Listing = {
  id: string
  source: string
  address: string
  city: string
  price: number
  bedrooms: number
  listedDate: string
  score: number
}

type SearchResponse = {
  results: Listing[]
  page: number
  pageSize: number
  total: number
  totalPages: number
}

type FieldError = { field: string; message: string }

const FIELDS = [
  { name: 'minPrice', label: 'Min price', type: 'number' },
  { name: 'maxPrice', label: 'Max price', type: 'number' },
  { name: 'minBedrooms', label: 'Min bedrooms', type: 'number' },
  { name: 'city', label: 'City', type: 'text' },
  { name: 'keyword', label: 'Keyword', type: 'text' },
  { name: 'targetBudget', label: 'Target budget', type: 'number' },
  { name: 'pageSize', label: 'Page size', type: 'number' },
] as const

type Filters = Record<(typeof FIELDS)[number]['name'], string>

const INITIAL: Filters = {
  minPrice: '', maxPrice: '', minBedrooms: '', city: '', keyword: '', targetBudget: '', pageSize: '10',
}

const errorStyle = { color: 'crimson' }

type State =
  | { kind: 'idle' }
  | { kind: 'loading' }
  | { kind: 'error'; errors: FieldError[] }
  | { kind: 'done'; data: SearchResponse }

export default function App() {
  const [filters, setFilters] = useState<Filters>(INITIAL)
  // Paging reuses the filters from the last submit, not whatever is half-typed in the form.
  const [submitted, setSubmitted] = useState<Filters>(INITIAL)
  const [state, setState] = useState<State>({ kind: 'idle' })

  async function search(page: number, f: Filters) {
    const qs = new URLSearchParams()
    for (const [k, v] of Object.entries(f)) if (v.trim() !== '') qs.set(k, v.trim())
    qs.set('page', String(page))

    setState({ kind: 'loading' })
    try {
      const res = await fetch(`/api/listings/search?${qs}`)
      const body = await res.json()
      if (res.ok) setState({ kind: 'done', data: body })
      else setState({ kind: 'error', errors: body.errors ?? [{ field: '', message: `HTTP ${res.status}` }] })
    } catch (e) {
      setState({ kind: 'error', errors: [{ field: '', message: `Could not reach the server: ${String(e)}` }] })
    }
  }

  const errors = state.kind === 'error' ? state.errors : []
  const formFields: string[] = FIELDS.map((f) => f.name)
  const generalErrors = errors.filter((e) => !formFields.includes(e.field))

  return (
    <main>
      <h1>Listing search</h1>
      <form
        onSubmit={(e) => {
          e.preventDefault()
          setSubmitted(filters)
          search(1, filters)
        }}
      >
        {FIELDS.map((f) => (
          <div key={f.name}>
            <label>
              {f.label}{' '}
              <input
                type={f.type}
                value={filters[f.name]}
                onChange={(e) => setFilters({ ...filters, [f.name]: e.target.value })}
              />
            </label>
            {errors
              .filter((e) => e.field === f.name)
              .map((e) => (
                <span key={e.message} role="alert" style={errorStyle}> {e.message}</span>
              ))}
          </div>
        ))}
        <button type="submit" disabled={state.kind === 'loading'}>Search</button>
      </form>

      {generalErrors.map((e) => (
        <p key={e.message} role="alert" style={errorStyle}>{e.message}</p>
      ))}
      {state.kind === 'loading' && <p>Loading...</p>}
      {state.kind === 'done' && <Results data={state.data} onPage={(p) => search(p, submitted)} />}
    </main>
  )
}

function Results({ data, onPage }: { data: SearchResponse; onPage: (page: number) => void }) {
  if (data.total === 0) return <p>No listings match these filters.</p>

  return (
    <>
      <table>
        <thead>
          <tr>
            <th>Address</th><th>City</th><th>Price</th><th>Beds</th><th>Listed</th><th>Score</th>
          </tr>
        </thead>
        <tbody>
          {data.results.map((l) => (
            // ids are only unique per source feed
            <tr key={`${l.source}:${l.id}`}>
              <td>{l.address}</td>
              <td>{l.city}</td>
              <td>${l.price.toLocaleString()}</td>
              <td>{l.bedrooms}</td>
              <td>{l.listedDate}</td>
              <td>{l.score.toFixed(1)}</td>
            </tr>
          ))}
        </tbody>
      </table>
      <p>
        <button onClick={() => onPage(data.page - 1)} disabled={data.page <= 1}>Prev</button>{' '}
        Page {data.page} of {data.totalPages} ({data.total} listings){' '}
        <button onClick={() => onPage(data.page + 1)} disabled={data.page >= data.totalPages}>Next</button>
      </p>
    </>
  )
}
