import { describe, it, expect } from 'vitest'
import merge from '../../vendor/deepmerge/index.cjs'

describe('configuration merge prototype isolation', () => {
  it('does not copy prototype-control keys from either input', () => {
    const hostile = JSON.parse('{"__proto__":{"injected":true},"constructor":{"prototype":{"injected":true}},"safe":{"value":1}}')
    for (const result of [merge(hostile, {}), merge({}, hostile), merge(Object.create(null), hostile)]) {
      expect(Object.getPrototypeOf(result)).toBe(Object.prototype)
      expect(result.injected).toBeUndefined()
      expect(Object.hasOwn(result, '__proto__')).toBe(false)
      expect(Object.hasOwn(result, 'constructor')).toBe(false)
      expect(result.safe.value).toBe(1)
    }
    expect(Object.prototype.injected).toBeUndefined()
  })

  it('isolates nested objects and array elements while preserving normal merges', () => {
    const hostile = JSON.parse('{"settings":{"__proto__":{"injected":true},"title":"Hello"},"items":[{"__proto__":{"injected":true},"id":1}]}')
    const result = merge(hostile, { settings: { language: 'ru' }, items: [{ id: 2 }] })
    expect(result.settings).toEqual({ title: 'Hello', language: 'ru' })
    expect(result.settings.injected).toBeUndefined()
    expect(result.items).toEqual([{ id: 1 }, { id: 2 }])
    expect(result.items[0].injected).toBeUndefined()
  })

  it('keeps the existing explicit array replacement and all API', () => {
    expect(merge({ list: [1] }, { list: [2] }, { arrayMerge: (_, source) => source })).toEqual({ list: [2] })
    expect(merge.all([{ a: 1 }, { b: 2 }])).toEqual({ a: 1, b: 2 })
  })
})
