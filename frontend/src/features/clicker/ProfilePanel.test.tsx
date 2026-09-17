import { render, screen } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import { ProfilePanel } from './ProfilePanel'
import { defaultHotkeys, defaultProfile } from './clicker.utils'

describe('ProfilePanel window picking', () => {
  it('shows the hold-and-release instruction while a window is being picked', () => {
    const profile = defaultProfile('profile-1', 'Tab 1')
    profile.state = 'picking'

    render(
      <ProfilePanel
        profile={profile}
        hotkeys={{ activeStart: defaultHotkeys().activeStart, activeStop: defaultHotkeys().activeStop }}
        onBind={vi.fn(async () => undefined)}
        onToggle={vi.fn(async () => undefined)}
        onBindingChange={vi.fn(async () => undefined)}
      />,
    )

    expect(screen.getByText('正在选择窗口')).toBeInTheDocument()
    expect(screen.getByText(/继续按住鼠标左键，移动到目标窗口，松开鼠标左键完成选择/)).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /继续移动/ })).toBeInTheDocument()
    expect(screen.getByTestId('clicker-window-pick')).toHaveClass('is-picking')
  })
})
