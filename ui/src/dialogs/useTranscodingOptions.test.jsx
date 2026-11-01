import React from 'react'
import { render, cleanup } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { useGetList } from 'react-admin'
import { useChildPolicy } from '../family/childPolicy'
import { useTranscodingOptions } from './useTranscodingOptions'

vi.mock('react-admin', () => ({
  BooleanInput: () => null,
  SelectInput: () => null,
  useGetList: vi.fn(),
  useTranslate: () => (key) => key,
}))
vi.mock('../family/childPolicy', () => ({ useChildPolicy: vi.fn() }))

const Harness = () => {
  useTranscodingOptions()
  return null
}

describe('download transcoding options permissions', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    useGetList.mockReturnValue({ data: {}, loading: false })
    useChildPolicy.mockReturnValue(null)
  })
  afterEach(cleanup)

  it('loads available profiles for an ordinary adult', () => {
    render(<Harness />)
    expect(useGetList.mock.calls[0][4]).toEqual({ enabled: true })
  })

  it.each([true, false])('does not fetch administrative profiles for a protected account (enabled=%s)', (enabled) => {
    useChildPolicy.mockReturnValue({ enabled, interface: 'kids' })
    useGetList.mockReturnValue({ data: undefined, loading: false })
    render(<Harness />)
    expect(useGetList.mock.calls[0][4]).toEqual({ enabled: false })
  })
})
