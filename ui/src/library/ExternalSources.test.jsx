import React from 'react'
import {
  render,
  screen,
  fireEvent,
  waitFor,
  cleanup,
} from '@testing-library/react'
import { describe, it, expect, vi, afterEach } from 'vitest'
import ExternalSources from './ExternalSources'
const state = vi.hoisted(() => ({ permissions: 'admin', request: vi.fn() }))
vi.mock('react-admin', () => ({
  Title: () => null,
  usePermissions: () => ({ permissions: state.permissions }),
}))
vi.mock('../dataProvider', () => ({
  httpClient: (...args) => state.request(...args),
}))
afterEach(() => {
  cleanup()
  vi.clearAllMocks()
  state.permissions = 'admin'
})
describe('External Music Sources', () => {
  it('denies non-admin configuration without a request', () => {
    state.permissions = 'regular'
    render(<ExternalSources />)
    expect(screen.getByText('Administrator access required.')).toBeTruthy()
    expect(state.request).not.toHaveBeenCalled()
  })
  it('shows approved sources and requires confirmation before catalog removal', async () => {
    state.request.mockResolvedValue({
      json: [
        {
          id: 'home-nas',
          type: 'webdav',
          enabled: true,
          status: 'Offline',
          libraryId: 2,
        },
      ],
    })
    const confirm = vi.spyOn(window, 'confirm').mockReturnValue(false)
    render(<ExternalSources />)
    await screen.findByText('home-nas')
    expect(screen.getByText('Offline')).toBeTruthy()
    fireEvent.click(screen.getByText('Remove association'))
    expect(confirm).toHaveBeenCalled()
    expect(state.request).toHaveBeenCalledTimes(1)
    confirm.mockRestore()
  })
  it('tests approved IDs without submitting an endpoint or secret', async () => {
    state.request.mockResolvedValue({
      json: [
        {
          id: 'archive',
          type: 's3',
          enabled: true,
          status: 'Not tested',
          libraryId: 0,
        },
      ],
    })
    render(<ExternalSources />)
    await screen.findByText('archive')
    fireEvent.click(screen.getByText('Test connection'))
    await waitFor(() =>
      expect(state.request).toHaveBeenCalledWith(
        '/api/external-source/archive/test',
        expect.objectContaining({ method: 'POST' }),
      ),
    )
  })
})
