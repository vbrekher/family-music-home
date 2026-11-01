import React from 'react'
import {
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import FamilyHome from './FamilyHome'

const state = vi.hoisted(() => ({ request: vi.fn() }))
vi.mock('react-admin', () => ({ Title: () => null }))
vi.mock('react-redux', () => ({
  useSelector: (select) => select({ player: { queue: [] } }),
}))
vi.mock('../dataProvider', () => ({
  httpClient: (...args) => state.request(...args),
}))
vi.mock('./childPolicy', () => ({ useChildPolicy: () => null }))
vi.mock('./FamilyNav', () => ({ default: () => null }))
vi.mock('./Notifications', () => ({ default: () => null }))
vi.mock('../common/Artwork', () => ({ Artwork: () => null }))
afterEach(() => {
  cleanup()
  vi.clearAllMocks()
})

describe('Home customization', () => {
  it('shows the saved section order, reflects a move immediately and saves the same visible order', async () => {
    const preferences = {
      widgets: ['new', 'shelves'],
      forgottenEnabled: false,
      forgottenDays: 180,
      forgottenMode: 'long',
    }
    state.request.mockImplementation(async (path, options) => ({
      json: path.endsWith('/preferences')
        ? options?.body
          ? JSON.parse(options.body)
          : preferences
        : [],
    }))
    render(<FamilyHome />)
    await waitFor(() =>
      expect(
        screen.getByRole('button', { name: 'Customize Home' }).disabled,
      ).toBe(false),
    )
    fireEvent.click(screen.getByRole('button', { name: 'Customize Home' }))
    const dialog = screen.getByRole('dialog')
    const row = (label) =>
      within(dialog)
        .getByRole('checkbox', { name: label, exact: true })
        .closest('label').parentElement
    const precedes = (first, second) =>
      Boolean(
        row(first).compareDocumentPosition(row(second)) &
        Node.DOCUMENT_POSITION_FOLLOWING,
      )
    expect(precedes('Newly Added', 'My Shelves')).toBe(true)
    fireEvent.click(
      within(row('My Shelves')).getByRole('button', {
        name: 'Up',
        exact: true,
      }),
    )
    expect(precedes('My Shelves', 'Newly Added')).toBe(true)
    fireEvent.click(within(dialog).getByRole('button', { name: 'Save Home' }))
    await waitFor(() =>
      expect(
        state.request.mock.calls.some(
          ([path, options]) =>
            path.endsWith('/preferences') &&
            options?.method === 'PUT' &&
            JSON.stringify(JSON.parse(options.body).widgets) ===
              JSON.stringify(['shelves', 'new']),
        ),
      ).toBe(true),
    )
    await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull())
    expect(
      screen
        .getAllByRole('region')
        .map((section) => section.getAttribute('aria-label')),
    ).toEqual(['My Shelves', 'Newly Added'])
  })
})
