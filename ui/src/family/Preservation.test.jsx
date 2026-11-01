import React from 'react'
import {
  render as renderUI,
  screen,
  fireEvent,
  waitFor,
} from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { beforeEach, afterEach, describe, it, expect, vi } from 'vitest'
const state = vi.hoisted(() => ({
  request: vi.fn(),
  binary: vi.fn(),
  dispatch: vi.fn(),
  getOne: vi.fn(),
}))
vi.mock('../dataProvider', () => ({ httpClient: state.request }))
vi.mock('./Media', () => ({ mediaFetch: state.binary }))
vi.mock('react-admin', () => ({
  Title: () => null,
  useDataProvider: () => ({ getOne: state.getOne }),
}))
vi.mock('react-redux', () => ({
  useDispatch: () => state.dispatch,
  useSelector: (fn) => fn({ theme: 'NordTheme' }),
}))
vi.mock('../subsonic', () => ({
  default: {
    star: vi.fn().mockResolvedValue({}),
    unstar: vi.fn().mockResolvedValue({}),
  },
}))
vi.mock('../common/Artwork', () => ({ Artwork: () => null }))
import Children from './Children'
import Kids from './Kids'
import Legacy from './Legacy'
const render = (element) => renderUI(<MemoryRouter>{element}</MemoryRouter>)
const policy = {
  interface: 'kids',
  density: 'large',
  storyLibraries: [],
  allowFamilyMessages: false,
  enabled: true,
}
beforeEach(() => {
  vi.clearAllMocks()
  localStorage.clear()
})
afterEach(() => localStorage.clear())
describe('Protected family accounts and personal preservation', () => {
  it('does not enumerate or create child accounts for an ordinary member', async () => {
    state.request.mockResolvedValue({ json: { admin: false } })
    render(<Children />)
    await screen.findByText('Only the family Owner can manage child accounts.')
    expect(state.request.mock.calls.map((x) => x[0])).toEqual([
      '/api/family/me',
    ])
    expect(
      screen.queryByRole('button', { name: 'Create child account' }),
    ).toBeNull()
  })
  it('leaves real accounts unchanged until an Owner explicitly confirms a creation', async () => {
    state.request.mockImplementation(async (url) => ({
      json: url.endsWith('/me') ? { admin: true } : [],
    }))
    render(<Children />)
    await screen.findByText(
      'No child accounts yet. None are created automatically.',
    )
    fireEvent.click(
      screen.getByRole('button', { name: 'Create child account' }),
    )
    expect(
      screen.getByRole('button', { name: 'Save protected account' }),
    ).toBeDisabled()
    expect(state.request.mock.calls.some((x) => x[1]?.method === 'POST')).toBe(
      false,
    )
  })
  it('keeps kids music metadata-only until a Play action uses the existing player', async () => {
    state.request.mockImplementation(async (url) => ({
      json: url.endsWith('/me')
        ? { child: policy }
        : {
            profile: policy,
            tracks: [
              {
                id: 'track0001',
                title: 'A synthetic story',
                artist: 'Family',
                starred: false,
                available: true,
              },
            ],
            shelves: [],
          },
    }))
    state.getOne.mockResolvedValue({
      data: { id: 'track0001', title: 'A synthetic story' },
    })
    render(<Kids />)
    await screen.findByText('A synthetic story')
    expect(state.getOne).not.toHaveBeenCalled()
    expect(state.dispatch).not.toHaveBeenCalled()
    fireEvent.click(
      screen.getByRole('button', { name: 'Play A synthetic story' }),
    )
    await waitFor(() =>
      expect(state.getOne).toHaveBeenCalledWith('song', { id: 'track0001' }),
    )
    await waitFor(() => expect(state.dispatch).toHaveBeenCalled())
    expect(screen.getByRole('button', { name: 'Stories' })).toBeVisible()
  })
  it('shows a paused account without playback controls', async () => {
    state.request.mockImplementation(async (url) => ({
      json: url.endsWith('/me')
        ? { child: { ...policy, enabled: false } }
        : { profile: { ...policy, enabled: false }, tracks: [], shelves: [] },
    }))
    render(<Kids />)
    await screen.findByText(
      'Your parent has paused this account. Your music history and favorites are safe.',
    )
    expect(screen.queryByRole('button', { name: 'Songs' })).toBeNull()
    expect(state.getOne).not.toHaveBeenCalled()
  })
  it('prepares only the selected private scope and labels its encryption and restore limits', async () => {
    state.request.mockResolvedValue({
      json: { id: 'export0001', expiresAt: '2026-10-31T20:00:00Z' },
    })
    render(<Legacy />)
    expect(screen.getByText(/This private ZIP is not encrypted/)).toBeVisible()
    fireEvent.click(
      screen.getByRole('button', { name: 'Prepare private archive' }),
    )
    await screen.findByRole('button', { name: 'Download archive' })
    const [url, opts] = state.request.mock.calls[0]
    expect(url).toBe('/api/family/legacy/prepare')
    expect(JSON.parse(opts.body)).toMatchObject({
      scope: 'personal',
      attachments: true,
      catalog: true,
      playlists: true,
    })
    expect(state.binary).not.toHaveBeenCalled()
    expect(
      screen.getByText(/Complete account and server restoration uses/),
    ).toBeVisible()
  })
  it('shows verification success only after the read-only verifier accepts an uploaded ZIP', async () => {
    state.binary.mockResolvedValue({
      ok: true,
      json: async () => ({ verified: true, entries: 2, files: 8 }),
    })
    render(<Legacy />)
    expect(screen.queryByText(/Verified:/)).toBeNull()
    const file = new File(['synthetic zip bytes'], 'owned.zip', {
      type: 'application/zip',
    })
    fireEvent.change(screen.getByLabelText('Archive to verify'), {
      target: { files: [file] },
    })
    await screen.findByText(
      'Verified: 2 family records, 8 files. The ZIP is unencrypted. No data was imported.',
    )
    expect(state.binary).toHaveBeenCalledWith(
      '/legacy/verify',
      expect.objectContaining({ method: 'POST', body: file }),
    )
  })
})
