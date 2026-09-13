import { describe, expect, it } from 'vitest'
import { MAX_PROFILES, MAX_BINDINGS, defaultProfile, defaultSnapshot, isBindingValid } from './clicker.utils'

describe('clicker model helpers', () => {
  it('creates a profile with twelve empty bindings', () => {
    const profile = defaultProfile('profile-1', 'Tab 1')
    expect(profile.bindings).toHaveLength(MAX_BINDINGS)
    expect(profile.bindings.every((binding) => binding.code === '')).toBe(true)
  })

  it('limits the workspace to eight profiles', () => {
    expect(MAX_PROFILES).toBe(8)
    expect(defaultSnapshot().profiles).toHaveLength(1)
  })

  it('accepts empty slots and rejects invalid delays', () => {
    expect(isBindingValid({ code: '', label: '', delayMs: 0 })).toBe(true)
    expect(isBindingValid({ code: 'KeyA', label: 'A', delayMs: 9_999_999 })).toBe(true)
    expect(isBindingValid({ code: 'KeyA', label: 'A', delayMs: 10_000_000 })).toBe(false)
  })
})

