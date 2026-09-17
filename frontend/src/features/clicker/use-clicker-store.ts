import { useCallback, useEffect, useMemo, useState } from 'react'
import { clickerApi, nativeAvailable } from '../../lib/clicker-api'
import {
  defaultSnapshot,
  type HotkeyConfig,
  type KeyBinding,
  type ProfileState,
  type Snapshot,
  type WindowInfo,
} from './clicker.utils'

type StateEvent = { profileId: string; state: ProfileState; error?: string }
type PickEvent = { kind: string; profileId: string; target?: WindowInfo; error?: string }

export function useClickerStore() {
  const [snapshot, setSnapshot] = useState<Snapshot>(() => defaultSnapshot())
  const [preview, setPreview] = useState<Record<string, WindowInfo | undefined>>({})

  useEffect(() => {
    let disposed = false
    void clickerApi.getSnapshot().then((remote) => {
      if (!disposed && remote) setSnapshot(remote)
    }).catch(() => undefined)

    const offState = clickerApi.on('clicker:profile-state', (payload) => {
      const event = payload as StateEvent
      setSnapshot((current) => ({
        ...current,
        profiles: current.profiles.map((profile) => profile.id === event.profileId ? { ...profile, state: event.state, lastError: event.error } : profile),
      }))
    })
    const offPreview = clickerApi.on('clicker:window-pick-preview', (payload) => {
      const event = payload as PickEvent
      setPreview((current) => ({ ...current, [event.profileId]: event.target }))
    })
    const offComplete = clickerApi.on('clicker:window-pick-complete', (payload) => {
      const event = payload as PickEvent
      setPreview((current) => ({ ...current, [event.profileId]: undefined }))
      if (event.target) setSnapshot((current) => ({ ...current, profiles: current.profiles.map((profile) => profile.id === event.profileId ? { ...profile, target: event.target } : profile) }))
    })
    const offHotkeys = clickerApi.on('clicker:hotkeys-changed', (payload) => setSnapshot((current) => ({ ...current, hotkeys: payload as HotkeyConfig })))

    return () => {
      disposed = true
      offState()
      offPreview()
      offComplete()
      offHotkeys()
    }
  }, [])

  const updateProfile = useCallback((id: string, update: (profile: Snapshot['profiles'][number]) => Snapshot['profiles'][number]) => {
    setSnapshot((current) => ({ ...current, profiles: current.profiles.map((profile) => profile.id === id ? update(profile) : profile) }))
  }, [])

  const createProfile = useCallback(async () => {
    if (snapshot.profiles.length >= 8) return
    if (nativeAvailable()) {
      const created = await clickerApi.createProfile()
      setSnapshot((current) => ({ ...current, profiles: [...current.profiles, created], activeProfileId: created.id }))
      return
    }
    const index = snapshot.profiles.length + 1
    const created = { ...defaultSnapshot().profiles[0], id: `profile-${index}`, name: `Tab ${index}` }
    setSnapshot((current) => ({ ...current, profiles: [...current.profiles, created], activeProfileId: created.id }))
  }, [snapshot.profiles.length])

  const closeProfile = useCallback(async (id: string) => {
    if (nativeAvailable()) await clickerApi.closeProfile(id)
    setSnapshot((current) => {
      const profiles = current.profiles.filter((profile) => profile.id !== id)
      const nextActive = current.activeProfileId === id ? profiles[0]?.id ?? '' : current.activeProfileId
      return { ...current, profiles, activeProfileId: nextActive }
    })
  }, [])

  const renameProfile = useCallback(async (id: string, name: string) => {
    if (nativeAvailable()) await clickerApi.renameProfile(id, name)
    updateProfile(id, (profile) => ({ ...profile, name }))
  }, [updateProfile])

  const setActiveProfile = useCallback(async (id: string) => {
    if (nativeAvailable()) await clickerApi.setActiveProfile(id)
    setSnapshot((current) => ({ ...current, activeProfileId: id }))
  }, [])

  const setBinding = useCallback(async (id: string, index: number, binding: KeyBinding) => {
    if (nativeAvailable()) await clickerApi.setBinding(id, index, binding)
    updateProfile(id, (profile) => ({ ...profile, bindings: profile.bindings.map((item, itemIndex) => itemIndex === index ? binding : item) }))
  }, [updateProfile])

  const beginWindowPick = useCallback(async (id: string) => {
    if (nativeAvailable()) await clickerApi.beginWindowPick(id)
    else updateProfile(id, (profile) => ({ ...profile, state: 'picking' }))
  }, [updateProfile])

  const toggleProfile = useCallback(async (profile: Snapshot['profiles'][number]) => {
    if (profile.state === 'running') {
      if (nativeAvailable()) await clickerApi.stopProfile(profile.id)
      updateProfile(profile.id, (item) => ({ ...item, state: 'ready' }))
    } else {
      if (nativeAvailable()) await clickerApi.startProfile(profile.id)
      updateProfile(profile.id, (item) => ({ ...item, state: 'running', lastError: undefined }))
    }
  }, [updateProfile])

  const startAll = useCallback(async () => {
    if (nativeAvailable()) await clickerApi.startAll()
    setSnapshot((current) => ({ ...current, profiles: current.profiles.map((profile) => profile.target && profile.bindings.some((binding) => binding.code) ? { ...profile, state: 'running' } : profile) }))
  }, [])

  const stopAll = useCallback(async () => {
    if (nativeAvailable()) await clickerApi.stopAll()
    setSnapshot((current) => ({ ...current, profiles: current.profiles.map((profile) => profile.state === 'running' ? { ...profile, state: 'ready' } : profile) }))
  }, [])

  const setHotkeys = useCallback(async (hotkeys: HotkeyConfig) => {
    if (nativeAvailable()) await clickerApi.setHotkeys(hotkeys)
    setSnapshot((current) => ({ ...current, hotkeys }))
  }, [])

  const activeProfile = useMemo(() => snapshot.profiles.find((profile) => profile.id === snapshot.activeProfileId) ?? snapshot.profiles[0], [snapshot])

  return { snapshot, preview, activeProfile, createProfile, closeProfile, renameProfile, setActiveProfile, setBinding, beginWindowPick, toggleProfile, startAll, stopAll, setHotkeys }
}

