export const MAX_PROFILES = 8
export const MAX_BINDINGS = 12

export type ProfileState = 'idle' | 'picking' | 'ready' | 'running' | 'error'

export type KeyBinding = {
  code: string
  label: string
  delayMs: number
}

export type WindowInfo = {
  title: string
  processName: string
  pid: number
}

export type ProfileView = {
  id: string
  name: string
  bindings: KeyBinding[]
  target?: WindowInfo
  state: ProfileState
  lastError?: string
}

export type HotkeyConfig = {
  activeStart: string
  activeStop: string
  globalStart: string
  globalStop: string
}

export type Snapshot = {
  profiles: ProfileView[]
  activeProfileId: string
  hotkeys: HotkeyConfig
}

export const defaultHotkeys = (): HotkeyConfig => ({
  activeStart: 'pageup',
  activeStop: 'pagedown',
  globalStart: 'ctrl+pageup',
  globalStop: 'ctrl+pagedown',
})

export function defaultProfile(id: string, name: string): ProfileView {
  return {
    id,
    name,
    bindings: Array.from({ length: MAX_BINDINGS }, () => ({ code: '', label: '', delayMs: 100 })),
    state: 'idle',
  }
}

export function defaultSnapshot(): Snapshot {
  const profile = defaultProfile('profile-1', 'Tab 1')
  return { profiles: [profile], activeProfileId: profile.id, hotkeys: defaultHotkeys() }
}

export function isBindingValid(binding: KeyBinding): boolean {
  return (!binding.code && !binding.label) || (Boolean(binding.code) && binding.delayMs >= 0 && binding.delayMs <= 9_999_999)
}

export function displayKeyFromCode(code: string, key = ''): string {
  if (/^Key[A-Z]$/.test(code)) return code.slice(3)
  if (/^Digit[0-9]$/.test(code)) return code.slice(5)
  const names: Record<string, string> = {
    Space: 'Space',
    Enter: 'Enter',
    Escape: 'Esc',
    Backspace: 'Backspace',
    Delete: 'Delete',
    PageUp: 'PageUp',
    PageDown: 'PageDown',
    ArrowUp: '↑',
    ArrowDown: '↓',
    ArrowLeft: '←',
    ArrowRight: '→',
  }
  return names[code] ?? (key || code)
}

export function hotkeyFromKeyboardEvent(event: KeyboardEvent): string | null {
  const modifierOnly = ['Control', 'Alt', 'Shift', 'Meta'].includes(event.key)
  if (modifierOnly) return null
  const parts = [
    event.ctrlKey ? 'ctrl' : '',
    event.altKey ? 'alt' : '',
    event.shiftKey ? 'shift' : '',
    event.metaKey ? 'win' : '',
  ].filter(Boolean)
  const code = event.code.toLowerCase()
  const key = code.startsWith('key') ? code.slice(3) : code.startsWith('digit') ? code.slice(5) : code
  parts.push(key)
  return parts.join('+')
}
