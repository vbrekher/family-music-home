import React from 'react'
import { cleanup, render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { afterEach, describe, expect, it, vi } from 'vitest'
import FamilyNav from './FamilyNav'
vi.mock('./childPolicy', () => ({ useChildPolicy: () => null }))
afterEach(() => cleanup())
describe('Family navigation detail routes', () => {
  it('keeps Shelves selected while an individual shelf is open', () => {
    const { container } = render(
      <MemoryRouter initialEntries={['/family/shelves/synthetic-shelf']}>
        <FamilyNav />
      </MemoryRouter>,
    )
    expect(
      screen
        .getByRole('link', { name: 'Shelves', exact: true })
        .getAttribute('aria-current'),
    ).toBe('page')
    expect(
      screen
        .getByRole('link', { name: 'Home', exact: true })
        .getAttribute('aria-current'),
    ).toBeNull()
    expect(container.querySelector('select').value).toBe('shelves')
  })
  it('names and marks an overflow section while its detail page is open', () => {
    const { container } = render(
      <MemoryRouter initialEntries={['/family/versions/synthetic-edition']}>
        <FamilyNav />
      </MemoryRouter>,
    )
    expect(
      screen
        .getByRole('button', { name: 'Editions', exact: true })
        .getAttribute('aria-current'),
    ).toBe('page')
    expect(
      screen
        .getByRole('link', { name: 'Home', exact: true })
        .getAttribute('aria-current'),
    ).toBeNull()
    expect(container.querySelector('select').value).toBe('versions')
  })
})
