import { useCallback, useEffect, useState } from 'react'
import { followSyncApi, nativeAvailable } from './follow-sync-api'
import { defaultFollowSyncSnapshot, type FollowSyncSnapshot, type RuleConfig, type WindowInfo, type WindowPickRole } from './follow-sync.utils'

type StateEvent = { state: FollowSyncSnapshot['state']; capturedCount: number; lastCode?: string; error?: string }
type WindowPickEvent = { kind: string; role: 'main' | 'follow'; target?: WindowInfo; error?: string }

export function useFollowSyncStore() {
  const [snapshot, setSnapshot] = useState<FollowSyncSnapshot>(() => defaultFollowSyncSnapshot())
  const [preview, setPreview] = useState<{ role: 'main' | 'follow'; target?: WindowInfo }>()
  const [pickerRole, setPickerRole] = useState<WindowPickRole>()

  useEffect(() => {
    let disposed = false
    void followSyncApi.getSnapshot().then((remote) => {
      if (!disposed && remote) setSnapshot(remote)
    }).catch(() => undefined)

    const offState = followSyncApi.on('follow-sync:state', (payload) => {
      const event = payload as StateEvent
      setSnapshot((current) => ({ ...current, state: event.state, capturedCount: event.capturedCount, lastCode: event.lastCode, lastError: event.error }))
    })
    const offPreview = followSyncApi.on('follow-sync:window-pick-preview', (payload) => {
      const event = payload as WindowPickEvent
      setPickerRole(event.role)
      setPreview({ role: event.role, target: event.target })
    })
    const offComplete = followSyncApi.on('follow-sync:window-pick-complete', (payload) => {
      const event = payload as WindowPickEvent
      setPickerRole(undefined)
      setPreview(undefined)
      if (!event.target) return
      const target = event.target
      setSnapshot((current) => event.role === 'main'
        ? { ...current, main: target, state: current.follows.length ? 'ready' : 'idle', lastError: undefined }
        : { ...current, follows: [...current.follows, target], state: current.main ? 'ready' : 'idle', lastError: undefined })
    })
    const offCancelled = followSyncApi.on('follow-sync:window-pick-cancelled', () => {
      setPickerRole(undefined)
      setPreview(undefined)
    })
    const offError = followSyncApi.on('follow-sync:error', (payload) => {
      const event = payload as { message?: string }
      if (event.message) {
        setPickerRole(undefined)
        setPreview(undefined)
        setSnapshot((current) => ({ ...current, lastError: event.message }))
      }
    })

    return () => {
      disposed = true
      offState()
      offPreview()
      offComplete()
      offCancelled()
      offError()
    }
  }, [])

  const setRules = useCallback(async (rules: RuleConfig) => {
    if (nativeAvailable()) await followSyncApi.setRules(rules)
    setSnapshot((current) => ({ ...current, rules, lastError: undefined }))
  }, [])

  const beginMainWindowPick = useCallback(async () => {
    setPickerRole('main')
    setPreview({ role: 'main' })
    try {
      if (nativeAvailable()) await followSyncApi.beginMainWindowPick()
    } catch (error) {
      setPickerRole(undefined)
      setPreview(undefined)
      throw error
    }
  }, [])

  const beginFollowWindowPick = useCallback(async () => {
    setPickerRole('follow')
    setPreview({ role: 'follow' })
    try {
      if (nativeAvailable()) await followSyncApi.beginFollowWindowPick()
    } catch (error) {
      setPickerRole(undefined)
      setPreview(undefined)
      throw error
    }
  }, [])

  const cancelWindowPick = useCallback(async () => {
    if (nativeAvailable()) await followSyncApi.cancelWindowPick()
    setPickerRole(undefined)
    setPreview(undefined)
  }, [])

  const removeFollowWindow = useCallback(async (index: number) => {
    if (nativeAvailable()) await followSyncApi.removeFollowWindow(index)
    setSnapshot((current) => {
      const follows = current.follows.filter((_, itemIndex) => itemIndex !== index)
      return { ...current, follows, state: current.main && follows.length ? 'ready' : 'idle' }
    })
  }, [])

  const clearTargets = useCallback(async () => {
    if (nativeAvailable()) await followSyncApi.clearTargets()
    setSnapshot((current) => ({ ...current, main: undefined, follows: [], state: 'idle', lastError: undefined }))
  }, [])

  const start = useCallback(async () => {
    if (nativeAvailable()) await followSyncApi.start()
    setSnapshot((current) => ({ ...current, state: 'running', lastError: undefined }))
  }, [])

  const stop = useCallback(async () => {
    if (nativeAvailable()) await followSyncApi.stop()
    setSnapshot((current) => ({ ...current, state: current.main && current.follows.length ? 'ready' : 'idle' }))
  }, [])

  return { snapshot, preview, pickerRole, setRules, beginMainWindowPick, beginFollowWindowPick, cancelWindowPick, removeFollowWindow, clearTargets, start, stop }
}
