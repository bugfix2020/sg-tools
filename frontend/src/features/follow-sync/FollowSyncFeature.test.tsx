import { fireEvent, render, screen } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { FollowSyncFeature } from './FollowSyncFeature'

const mockStore = vi.hoisted(() => ({
  snapshot: {
    state: 'idle' as const,
    main: undefined,
    follows: [],
    rules: { include: [], exclude: [] },
    capturedCount: 0,
  },
  preview: undefined as { role: 'main' | 'follow'; target?: { title: string; processName: string; pid: number } } | undefined,
  pickerRole: undefined as 'main' | 'follow' | undefined,
  beginMainWindowPick: vi.fn(async () => undefined),
  beginFollowWindowPick: vi.fn(async () => undefined),
  cancelWindowPick: vi.fn(async () => undefined),
  removeFollowWindow: vi.fn(async () => undefined),
  clearTargets: vi.fn(async () => undefined),
  setRules: vi.fn(async () => undefined),
  start: vi.fn(async () => undefined),
  stop: vi.fn(async () => undefined),
}))

vi.mock('./use-follow-sync-store', () => ({
  useFollowSyncStore: () => mockStore,
}))

describe('FollowSyncFeature window picking', () => {
  beforeEach(() => {
    mockStore.snapshot = {
      state: 'idle',
      main: undefined,
      follows: [],
      rules: { include: [], exclude: [] },
      capturedCount: 0,
    }
    mockStore.preview = undefined
    mockStore.pickerRole = undefined
    mockStore.beginMainWindowPick.mockClear()
    mockStore.beginFollowWindowPick.mockClear()
    mockStore.cancelWindowPick.mockClear()
  })

  it('starts picking on mouse down instead of click release', () => {
    render(<FollowSyncFeature />)
    const button = screen.getByRole('button', { name: /选择主窗口/ })

    fireEvent.click(button)
    expect(mockStore.beginMainWindowPick).not.toHaveBeenCalled()

    fireEvent.mouseDown(button)
    expect(mockStore.beginMainWindowPick).toHaveBeenCalledTimes(1)
  })

  it('shows a clear active instruction and candidate state while picking', () => {
    mockStore.pickerRole = 'main'
    mockStore.preview = { role: 'main', target: { title: 'Demo', processName: 'demo.exe', pid: 42 } }

    render(<FollowSyncFeature />)

    expect(screen.getByText('正在选择主窗口')).toBeInTheDocument()
    expect(screen.getAllByText(/松开鼠标左键完成选择/).length).toBeGreaterThan(0)
    expect(screen.getAllByText(/当前候选：Demo/).length).toBeGreaterThan(0)
    expect(screen.getByRole('button', { name: /取消选择/ })).toBeInTheDocument()
    expect(screen.getByTestId('sync-main-window-row')).toHaveClass('is-picking')
  })
})
