import React from 'react'
import { render, screen, cleanup } from '@testing-library/react'
import { afterEach, describe, it, expect, vi } from 'vitest'
const test = vi.hoisted(() => ({ child: null, library: vi.fn() }))
vi.mock('../family/childPolicy', () => ({ useChildPolicy: () => test.child }))
vi.mock('../common/LibrarySelector', () => ({
  default: () => {
    test.library()
    return <span>Library selector</span>
  },
}))
vi.mock('./SubMenu', () => ({
  default: ({ children }) => <div>{children}</div>,
}))
vi.mock('./PlaylistsSubMenu', () => ({ default: () => null }))
vi.mock('../config', () => ({ default: { devSidebarPlaylists: false } }))
vi.mock('react-redux', () => ({
  useSelector: (fn) =>
    fn({ admin: { ui: { sidebarOpen: true } }, player: { queue: [] } }),
}))
vi.mock('react-admin', () => ({
  usePermissions: () => ({ permissions: 'user' }),
  useTranslate: () => (key, options) => options?._ || key,
  getResources: () => [
    { name: 'album', hasList: true, options: {} },
    { name: 'song', hasList: true, options: {} },
    { name: 'radio', hasList: true, options: {} },
    { name: 'playlist', hasList: true, options: { subMenu: 'playlist' } },
  ],
  MenuItemLink: ({ primaryText }) => <span>{primaryText}</span>,
}))
import Menu from './Menu'
afterEach(() => {
  cleanup()
  test.child = null
  vi.clearAllMocks()
})
describe('Protected account catalog navigation', () => {
  it('keeps adult library selection and radio navigation', () => {
    render(<Menu />)
    expect(test.library).toHaveBeenCalled()
    expect(screen.getByText('Radios')).toBeInTheDocument()
  })
  it('keeps full protected catalog navigation without mounting privileged user lookup or external radios', () => {
    test.child = { interface: 'full', enabled: true }
    render(<Menu />)
    expect(test.library).not.toHaveBeenCalled()
    expect(screen.queryByText('Radios')).toBeNull()
    expect(screen.getByText('Songs')).toBeInTheDocument()
    expect(screen.getByText('Family Home')).toBeInTheDocument()
  })
  it('keeps simplified protected navigation focused on music and requests', () => {
    test.child = { interface: 'kids', enabled: true }
    render(<Menu />)
    expect(screen.getByText('Your music')).toBeInTheDocument()
    expect(screen.getByText('Ask for music')).toBeInTheDocument()
    expect(test.library).not.toHaveBeenCalled()
    expect(screen.queryByText('Radios')).toBeNull()
  })
})
