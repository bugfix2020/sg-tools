import * as NativeClicker from '../../wailsjs/go/app/ClickerService'
import { EventsOn } from '../../wailsjs/runtime/runtime'
import type { app, clicker } from '../../wailsjs/go/models'
import type { HotkeyConfig, ProfileView, Snapshot, KeyBinding } from '../features/clicker/clicker.utils'

export type BatchResult = clicker.BatchResult

const nativeAvailable = () => typeof window !== 'undefined' && Boolean((window as unknown as { go?: unknown }).go)

function normalizeProfile(profile: app.ProfileView): ProfileView {
  return {
    ...profile,
    state: profile.state as ProfileView['state'],
  }
}

function normalizeSnapshot(remote: app.Snapshot): Snapshot {
  return {
    activeProfileId: remote.activeProfileId,
    hotkeys: remote.hotkeys,
    profiles: remote.profiles.map(normalizeProfile),
  }
}

export const clickerApi = {
  getSnapshot: async (): Promise<Snapshot | null> => nativeAvailable() ? normalizeSnapshot(await NativeClicker.GetSnapshot()) : null,
  createProfile: async () => normalizeProfile(await NativeClicker.CreateProfile()),
  closeProfile: (id: string) => NativeClicker.CloseProfile(id),
  renameProfile: (id: string, name: string) => NativeClicker.RenameProfile(id, name),
  setBinding: (id: string, index: number, binding: KeyBinding) => NativeClicker.SetBinding(id, index, binding as clicker.KeyBinding),
  clearBinding: (id: string, index: number) => NativeClicker.ClearBinding(id, index),
  beginWindowPick: (id: string) => NativeClicker.BeginWindowPick(id),
  cancelWindowPick: () => NativeClicker.CancelWindowPick(),
  setActiveProfile: (id: string) => NativeClicker.SetActiveProfile(id),
  startProfile: (id: string) => NativeClicker.StartProfile(id),
  stopProfile: (id: string) => NativeClicker.StopProfile(id),
  startAll: () => NativeClicker.StartAll(),
  stopAll: () => NativeClicker.StopAll(),
  getHotkeys: () => NativeClicker.GetHotkeys(),
  setHotkeys: (config: HotkeyConfig) => NativeClicker.SetHotkeys(config as clicker.HotkeyConfig),
  resetHotkeys: () => NativeClicker.ResetHotkeys(),
  restartAsAdministrator: () => NativeClicker.RestartAsAdministrator(),
  on: (eventName: string, handler: (payload: unknown) => void) => nativeAvailable() ? EventsOn(eventName, handler) : () => undefined,
}

export { nativeAvailable }
