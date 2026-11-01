import React from 'react'
import {
  render,
  screen,
  fireEvent,
  waitFor,
  cleanup,
} from '@testing-library/react'
import { describe, it, expect, vi, afterEach } from 'vitest'
import FamilyHome from './FamilyHome'
import Shelves from './Shelves'
import Passport from './Passport'
const state = vi.hoisted(() => ({ request: vi.fn(), params: {} }))
vi.mock('react-admin', () => ({ Title: () => null }))
vi.mock('react-redux', () => ({
  useSelector: (fn) => fn({ player: { queue: [] } }),
}))
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
})
const preferences = {
  widgets: ['shelves', 'forgotten'],
  forgottenEnabled: false,
  forgottenDays: 180,
  forgottenMode: 'long',
}
describe('Family Home', () => {
  it('does not request rediscovery until enabled and persists personal widget settings', async () => {
    state.request.mockImplementation(async (url, opts) => ({
      json: url.endsWith('/preferences')
        ? opts?.body
          ? JSON.parse(opts.body)
          : preferences
        : [],
    }))
    render(<FamilyHome />)
    await screen.findByText(
      'Rediscovery is off. Enable it in Customize Home when you would like suggestions.',
    )
    expect(
      state.request.mock.calls.some(([u]) => u.includes('/albums/forgotten')),
    ).toBe(false)
    fireEvent.click(screen.getByText('Customize Home'))
    fireEvent.click(screen.getByLabelText('Enable forgotten album suggestions'))
    fireEvent.click(screen.getByText('Save Home'))
    await waitFor(() =>
      expect(
        state.request.mock.calls.some(
          ([u, o]) =>
            u.endsWith('/preferences') &&
            o?.method === 'PUT' &&
            JSON.parse(o.body).forgottenEnabled,
        ),
      ).toBe(true),
    )
    await waitFor(() =>
      expect(
        state.request.mock.calls.some(([u]) => u.includes('/albums/forgotten')),
      ).toBe(true),
    )
  })
  it('creates a real private shelf request and keeps shared shelves read-only', async () => {
    const shelf = {
      id: 'shared',
      title: 'From family',
      canEdit: false,
      body: { description: 'A memory', references: [] },
      sharedWith: [],
    }
    state.request.mockImplementation(async (url, opts) => ({
      json: url.endsWith('/people')
        ? [{ id: 'me', name: 'admin' }]
        : url.endsWith('/me')
          ? { id: 'me' }
          : opts?.method === 'POST'
            ? { ...JSON.parse(opts.body), id: 'new' }
            : [shelf],
    }))
    render(<Shelves />)
    await screen.findByText('From family')
    expect(screen.queryByText('Edit shelf')).toBeNull()
    fireEvent.click(screen.getByText('Create shelf'))
    fireEvent.change(screen.getByLabelText('Shelf title'), {
      target: { value: 'Our first shelf 🐰' },
    })
    fireEvent.click(screen.getByText('Save shelf'))
    await waitFor(() =>
      expect(
        state.request.mock.calls.some(
          ([u, o]) =>
            u.endsWith('/shelves') &&
            o?.method === 'POST' &&
            JSON.parse(o.body).title === 'Our first shelf 🐰' &&
            JSON.parse(o.body).sharedWith.length === 0,
        ),
      ).toBe(true),
    )
  })
  it('shows passport unknowns without converting sample rate into a quality verdict', async () => {
    state.request.mockImplementation(async (url) => ({
      json: url.includes('/search?')
        ? [{ id: 't1', kind: 'track', title: 'Sample', available: true }]
        : {
            id: 't1',
            title: 'Sample',
            sampleRate: 192000,
            bitDepth: null,
            provenance: 'Unknown',
            integrity: 'Not analyzed',
          },
    }))
    render(<Passport />)
    fireEvent.change(screen.getByLabelText('Search your library'), {
      target: { value: 'Sample' },
    })
    fireEvent.click(screen.getByText('Search'))
    fireEvent.click(await screen.findByText('Select Sample'))
    await screen.findByText('192000')
    expect(screen.getByText('Not analyzed')).toBeTruthy()
    expect(screen.getAllByText('Unknown').length).toBeGreaterThan(1)
    expect(screen.queryByText('Best quality')).toBeNull()
  })
})

it('keeps a stable shelf detail after editing instead of loading unrelated shelves', async () => {
  state.params = { id: 'owned' }
  let entry = {
    id: 'owned',
    title: 'One shelf',
    kind: 'shelf',
    canEdit: true,
    sharedWith: [],
    revision: 1,
    body: { description: '', references: [], hidden: false },
  }
  state.request.mockImplementation(async (url, opts) => {
    if (url.endsWith('/people')) return { json: [] }
    if (url.endsWith('/me')) return { json: { id: 'me' } }
    if (opts?.method === 'PUT') entry = JSON.parse(opts.body)
    return { json: entry }
  })
  render(<Shelves />)
  await screen.findByText('One shelf')
  fireEvent.click(screen.getByText('Edit shelf'))
  fireEvent.change(screen.getByLabelText('Shelf title'), {
    target: { value: 'Edited shelf' },
  })
  fireEvent.click(screen.getByText('Save shelf'))
  await screen.findByText('Edited shelf')
  expect(
    state.request.mock.calls.some(([url]) => url.includes('/shelves?')),
  ).toBe(false)
})
