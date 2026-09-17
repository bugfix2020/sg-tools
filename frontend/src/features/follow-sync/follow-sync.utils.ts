import type { app, clicker } from '../../../wailsjs/go/models'

export type SyncState = 'idle' | 'ready' | 'running' | 'error'

export type WindowInfo = app.WindowInfo
export type RuleConfig = clicker.KeyRuleConfig
export type WindowPickRole = 'main' | 'follow'

export type FollowSyncSnapshot = {
  state: SyncState
  main?: WindowInfo
  follows: WindowInfo[]
  rules: RuleConfig
  capturedCount: number
  lastCode?: string
  lastError?: string
}

export const defaultFollowSyncSnapshot = (): FollowSyncSnapshot => ({
  state: 'idle',
  main: undefined,
  follows: [],
  rules: { include: [], exclude: [] },
  capturedCount: 0,
  lastCode: undefined,
  lastError: undefined,
})

export function parseRuleText(value: string): string[] {
  const result: string[] = []
  const seen = new Set<string>()
  for (const part of value.split(',')) {
    const rule = part.trim()
    const key = rule.toLowerCase()
    if (!rule || seen.has(key)) continue
    seen.add(key)
    result.push(rule)
  }
  return result
}

export function formatWindow(window?: WindowInfo): string {
  if (!window) return '尚未绑定窗口'
  return window.title || window.processName || `PID ${window.pid}`
}

export function windowRoleLabel(role: WindowPickRole): string {
  return role === 'main' ? '主窗口' : '跟随窗口'
}

export function windowPickDescription(role: WindowPickRole, target?: WindowInfo): string {
  if (target) return `当前候选：${formatWindow(target)}。继续按住鼠标左键，松开鼠标左键完成选择。`
  return `正在扫描${windowRoleLabel(role)}：请继续按住鼠标左键移动到目标窗口，松开鼠标左键完成选择。`
}
