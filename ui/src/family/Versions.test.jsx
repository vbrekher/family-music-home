import React from 'react'
import {
  render,
  screen,
  fireEvent,
  waitFor,
  cleanup,
} from '@testing-library/react'
import { it, expect, vi, afterEach } from 'vitest'
import Versions from './Versions'
const request = vi.hoisted(() => vi.fn())
vi.mock('react-admin', () => ({ Title: () => null }))
vi.mock('react-router-dom', () => ({
  useLocation: () => ({ pathname: '/family' }),
  useParams: () => ({}),
}))
vi.mock('../dataProvider', () => ({
  httpClient: (...args) => request(...args),
}))
vi.mock('../common/Artwork', () => ({ Artwork: () => null }))
afterEach(() => {
  cleanup()
  vi.clearAllMocks()
})
it('shows per-edition metadata and saves preference for the current account only', async () => {
  request.mockImplementation(async (u) => ({
    json: u.endsWith('/me')
      ? { admin: false }
      : [
          {
            id: 'group',
            basis: 'confirmed',
            members: [
              {
                id: 'album-one',
                kind: 'album',
                title: 'Original CD',
                available: false,
                library: 'CD collection',
                technical: [
                  {
                    container: 'flac',
                    codec: '',
                    sampleRate: 44100,
                    bitDepth: 16,
                    channels: 2,
                  },
                ],
                sampledTracks: 1,
                totalTracks: 10,
                firstTrackId: 'track-one',
              },
              {
                id: 'album-two',
                kind: 'album',
                title: 'Later edition',
                available: true,
                library: 'HiRes collection',
                technical: [],
                sampledTracks: 0,
                totalTracks: 10,
              },
            ],
          },
        ],
  }))
  render(<Versions />)
  await screen.findAllByText('Original CD')
  expect(screen.getByText(/Codec unknown/)).toBeTruthy()
  expect(screen.getByText(/Temporarily unavailable/)).toBeTruthy()
  expect(screen.queryByText('Link editions')).toBeNull()
  fireEvent.click(screen.getAllByText('Prefer this edition')[0])
  await waitFor(() =>
    expect(
      request.mock.calls.some(
        ([u, o]) =>
          u.endsWith('/versions/group/preference') &&
          o?.method === 'PUT' &&
          JSON.parse(o.body).albumId === 'album-one' &&
          !('userId' in JSON.parse(o.body)),
      ),
    ).toBe(true),
  )
})
