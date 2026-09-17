import * as NativeFollowSync from '../../../wailsjs/go/app/FollowSyncService'
import { EventsOn } from '../../../wailsjs/runtime/runtime'
import type { app, clicker } from '../../../wailsjs/go/models'
import type { FollowSyncSnapshot, RuleConfig, SyncState, WindowInfo } from './follow-sync.utils'

const nativeAvailable = () => typeof window !== 'undefined' && Boolean((window as unknown as { go?: unknown }).go)

function normalizeWindow(window?: app.WindowInfo): WindowInfo | undefined {
  return window ? { title: window.title, processName: window.processName, pid: window.pid } : undefined
}

function normalizeSnapshot(remote: app.FollowSyncSnapshot): FollowSyncSnapshot {
  return {
    state: remote.state as SyncState,
    main: normalizeWindow(remote.main),
    follows: remote.follows.map((window) => normalizeWindow(window)!).filter(Boolean),
    rules: { include: [...remote.rules.include], exclude: [...remote.rules.exclude] },
    capturedCount: remote.capturedCount,
    lastCode: remote.lastCode,
    lastError: remote.lastError,
  }
}

export const followSyncApi = {
  getSnapshot: async (): Promise<FollowSyncSnapshot | null> => nativeAvailable() ? normalizeSnapshot(await NativeFollowSync.GetSnapshot()) : null,
  setRules: (rules: RuleConfig) => NativeFollowSync.SetRules(rules as clicker.KeyRuleConfig),
  beginMainWindowPick: () => NativeFollowSync.BeginMainWindowPick(),
  beginFollowWindowPick: () => NativeFollowSync.BeginFollowWindowPick(),
  cancelWindowPick: () => NativeFollowSync.CancelWindowPick(),
  removeFollowWindow: (index: number) => NativeFollowSync.RemoveFollowWindow(index),
  clearTargets: () => NativeFollowSync.ClearTargets(),
  start: () => NativeFollowSync.Start(),
  stop: () => NativeFollowSync.Stop(),
  on: (eventName: string, handler: (payload: unknown) => void) => nativeAvailable() ? EventsOn(eventName, handler) : () => undefined,
}

export { nativeAvailable }
