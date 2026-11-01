import React from 'react'
import {
  render,
  screen,
  fireEvent,
  waitFor,
  cleanup,
} from '@testing-library/react'
import { describe, it, expect, vi, afterEach } from 'vitest'
import Journal, { Letters, safeMessage } from './Journal'
import Requests from './Requests'
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
})
const response = (url, options) => ({
  json: url.endsWith('/me')
    ? { id: 'owner', admin: true }
    : url.endsWith('/people')
      ? [
          { id: 'owner', name: 'admin' },
          { id: 'member', name: 'member' },
        ]
      : options?.body
        ? { ...JSON.parse(options.body), id: 'saved' }
        : [],
})
describe('Family interactions', () => {
  it('does not overwrite a newly created detail with the previous capsule', async () => {
    state.params = { id: 'previous' }
    state.request.mockImplementation(async (u, o) =>
      u.endsWith('/capsules/previous')
        ? {
            json: {
              id: 'previous',
              title: 'Previous memory',
              canEdit: true,
              sharedWith: [],
              body: {
                state: 'draft',
                description: '',
                references: [],
                dateFrom: '2026-10-31',
                timezone: 'UTC',
              },
            },
          }
        : response(u, o),
    )
    render(<Journal />)
    await screen.findByText('Previous memory')
    const oldReads = state.request.mock.calls.filter(([u]) =>
      u.endsWith('/capsules/previous'),
    ).length
    fireEvent.click(screen.getByText('Create capsule'))
    fireEvent.change(screen.getByLabelText('Title'), {
      target: { value: 'New memory' },
    })
    fireEvent.click(screen.getByText('Save capsule'))
    await waitFor(() =>
      expect(window.location.hash).toBe('#/family/capsules/saved'),
    )
    expect(
      state.request.mock.calls.filter(([u]) => u.endsWith('/capsules/previous'))
        .length,
    ).toBe(oldReads)
  })

  it('composes a formatted letter draft without requiring HTML entry', async () => {
    state.request.mockImplementation(async (...args) => response(...args))
    render(<Letters />)
    fireEvent.click(screen.getByText('Create letter'))
    fireEvent.change(screen.getByLabelText('Title'), {
      target: { value: 'Our weekend' },
    })
    const message = screen.getByRole('textbox', {
      name: 'Message',
      exact: true,
    })
    message.innerHTML = '<p>A <strong>beautiful</strong> song</p>'
    fireEvent.input(message)
    expect(
      screen.getByRole('button', { name: 'Bold', exact: true }),
    ).toBeTruthy()
    fireEvent.click(screen.getByText('Save draft'))
    await waitFor(() =>
      expect(
        state.request.mock.calls.some(
          ([u, o]) =>
            u === '/api/family/letters' &&
            o?.method === 'POST' &&
            JSON.parse(o.body).body.message.includes(
              '<strong>beautiful</strong>',
            ) &&
            JSON.parse(o.body).body.state === 'draft',
        ),
      ).toBe(true),
    )
  })

  it('removes executable and remote-resource markup but keeps simple rich formatting', () => {
    const clean = safeMessage(
      '<strong onclick="bad()">Hello</strong><img src="https://private.example/pixel"><svg onload="bad()"></svg><a href="javascript:bad()">link</a><script>bad()</script>',
    )
    expect(clean).toContain('<strong>Hello</strong>')
    expect(clean).not.toMatch(/onclick|src=|svg|script|href=/)
  })
  it('creates a private dated capsule draft with Unicode notes', async () => {
    state.request.mockImplementation(async (...args) => response(...args))
    render(<Journal />)
    fireEvent.click(screen.getByText('Create capsule'))
    fireEvent.change(screen.getByLabelText('Title'), {
      target: { value: 'Наше лето 🐰' },
    })
    fireEvent.change(screen.getByLabelText('Notes'), {
      target: { value: 'Музыка у моря' },
    })
    fireEvent.click(screen.getByText('Save capsule'))
    await waitFor(() =>
      expect(
        state.request.mock.calls.some(
          ([u, o]) =>
            u === '/api/family/capsules' &&
            o?.method === 'POST' &&
            JSON.parse(o.body).sharedWith.length === 0 &&
            JSON.parse(o.body).body.state === 'draft' &&
            JSON.parse(o.body).body.description === 'Музыка у моря',
        ),
      ).toBe(true),
    )
  })
  it('previews a deduplicated ordered batch then persists each item without automatic acquisition', async () => {
    state.request.mockImplementation(async (u, o) =>
      u.endsWith('/request-preview')
        ? {
            json: {
              items: [
                { kind: 'track', query: 'First song' },
                { kind: 'track', query: 'Second song' },
              ],
              duplicates: 1,
            },
          }
        : response(u, o),
    )
    render(<Requests />)
    fireEvent.click(screen.getByText('Create music request'))
    fireEvent.change(screen.getByLabelText('Request title'), {
      target: { value: 'For our weekend' },
    })
    fireEvent.change(
      screen.getByLabelText('One title or artist per line (up to 100)'),
      { target: { value: 'First song\nFirst song\nSecond song' } },
    )
    fireEvent.click(screen.getByText('Preview list'))
    await screen.findByText(/duplicates removed/)
    fireEvent.click(screen.getByText('Add preview to draft'))
    fireEvent.click(screen.getByText('Save draft'))
    await waitFor(() =>
      expect(
        state.request.mock.calls.some(
          ([u, o]) =>
            u === '/api/family/requests' &&
            o?.method === 'POST' &&
            JSON.parse(o.body)
              .body.items.map((x) => x.query)
              .join(',') === 'First song,Second song',
        ),
      ).toBe(true),
    )
  })
  it('marks a delivered letter read only when recipient opens its detail', async () => {
    state.params = { id: 'received' }
    state.request.mockImplementation(async (u, o) =>
      u.endsWith('/letters/received')
        ? {
            json: {
              id: 'received',
              title: 'A private song',
              canEdit: false,
              unread: true,
              sharedWith: ['owner'],
              body: { state: 'sent', message: '<b>Hi</b>', references: [] },
            },
          }
        : response(u, o),
    )
    render(<Letters />)
    await screen.findByText('A private song')
    await waitFor(() =>
      expect(
        state.request.mock.calls.some(([u]) =>
          u.endsWith('/letters/received/read'),
        ),
      ).toBe(true),
    )
    expect(screen.queryByText('Edit letter')).toBeNull()
    expect(screen.queryByText('Send letter')).toBeNull()
  })
})
