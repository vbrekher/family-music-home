import React from 'react'
import {
  render,
  screen,
  fireEvent,
  waitFor,
  cleanup,
} from '@testing-library/react'
import { it, expect, vi, afterEach } from 'vitest'
import Doctor, { Spectrum } from './Doctor'
const state = vi.hoisted(() => ({ request: vi.fn(), params: {} }))
vi.mock('react-admin', () => ({ Title: () => null }))
vi.mock('react-router-dom', () => ({
  useLocation: () => ({ pathname: '/family' }),
  useParams: () => state.params,
}))
vi.mock('../dataProvider', () => ({
  httpClient: (...args) => state.request(...args),
}))
vi.mock('../common/Artwork', () => ({ Artwork: () => null }))
afterEach(() => {
  cleanup()
  vi.clearAllMocks()
  state.params = {}
  window.location.hash = ''
})
it('does not request privileged diagnostics for a regular member', async () => {
  state.request.mockResolvedValue({ json: { admin: false } })
  render(<Doctor />)
  await screen.findByText(/available to the Owner/)
  expect(state.request.mock.calls.some(([u]) => u.includes('/doctor/'))).toBe(
    false,
  )
})
it('creates a bounded SQL-only audit without any audio request', async () => {
  state.request.mockImplementation(async (u, o) => ({
    json: u.endsWith('/me')
      ? { admin: true }
      : o?.body
        ? { id: 'run00001', ...JSON.parse(o.body) }
        : [],
  }))
  render(<Doctor />)
  await waitFor(() =>
    expect(
      screen.getByRole('button', { name: 'New catalog audit', exact: true })
        .disabled,
    ).toBe(false),
  )
  fireEvent.click(screen.getByText('New catalog audit'))
  fireEvent.change(
    screen.getByLabelText('Maximum catalog records in this window'),
    { target: { value: '500' } },
  )
  fireEvent.click(screen.getByText('Create bounded run'))
  await waitFor(() =>
    expect(
      state.request.mock.calls.some(
        ([u, o]) =>
          u.endsWith('/doctor/jobs') &&
          o?.method === 'POST' &&
          JSON.parse(o.body).mode === 'metadata' &&
          JSON.parse(o.body).settings.maxRows === 500,
      ),
    ).toBe(true),
  )
  expect(state.request.mock.calls.some(([u]) => u.endsWith('/content'))).toBe(
    false,
  )
})
it('explains report-only removal and leaves it untouched when the user keeps it', async () => {
  state.params = { id: 'run00001' }
  const job = {
    id: 'run00001',
    mode: 'metadata',
    state: 'completed',
    processed: 1,
    findingCount: 0,
    settings: { maxRows: 1 },
    counts: { completed: 1 },
    budgets: [],
  }
  state.request.mockImplementation(async (u) => ({
    json: u.endsWith('/me')
      ? { admin: true }
      : u.endsWith('/jobs/run00001')
        ? job
        : [],
  }))
  render(<Doctor />)
  await screen.findByText('Remove finished report')
  fireEvent.click(screen.getByText('Remove finished report'))
  await screen.findByText(/Original music, catalog entries/)
  fireEvent.click(screen.getByText('Keep report'))
  expect(
    state.request.mock.calls.some(([u]) => u.endsWith('/remove-report')),
  ).toBe(false)
})
it('labels spectral evidence as a limited measurement rather than a quality score', () => {
  render(
    <Spectrum
      result={{
        spectralBands: [0.8, 0.1, 0.1],
        measuredSampleRate: 44100,
        bandwidthNote: 'Limited sampled windows; not proof of provenance.',
      }}
    />,
  )
  expect(
    screen.getByRole('img', { name: /not proof of audio provenance/ }),
  ).toBeTruthy()
  expect(screen.getByText(/not a quality score/)).toBeTruthy()
})
it('keeps selection preview metadata-only and requires explicit source approval', async () => {
  state.request.mockImplementation(async (u, o) => ({
    json: u.endsWith('/me')
      ? { admin: true }
      : u.includes('/search?')
        ? [
            {
              id: 'track0001',
              kind: 'track',
              title: 'Synthetic recording',
              available: true,
            },
          ]
        : u.endsWith('/doctor/tracks/track0001')
          ? {
              id: 'track0001',
              title: 'Synthetic recording',
              libraryId: 1,
              library: 'Fixture',
              size: 100,
              available: true,
            }
          : [],
  }))
  render(<Doctor />)
  await waitFor(() =>
    expect(
      screen.getByRole('button', {
        name: 'Investigate selected audio',
        exact: true,
      }).disabled,
    ).toBe(false),
  )
  fireEvent.click(screen.getByText('Investigate selected audio'))
  fireEvent.change(screen.getByLabelText('Search your library'), {
    target: { value: 'Synthetic' },
  })
  fireEvent.click(screen.getByText('Search'))
  fireEvent.click(await screen.findByText('Select Synthetic recording'))
  await screen.findByText('Fixture · 0.00 MiB')
  expect(
    screen.getByRole('button', { name: 'Create bounded run', exact: true })
      .disabled,
  ).toBe(true)
  expect(state.request.mock.calls.some(([u]) => u.endsWith('/content'))).toBe(
    false,
  )
})
