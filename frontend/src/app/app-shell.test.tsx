import { render, screen, waitFor } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { AppShell } from './AppShell'

describe('AppShell', () => {
  it('shows one outer feature tab and the clicker steps', async () => {
    render(<AppShell />)
    await waitFor(() => {
      expect(screen.getByRole('tab', { name: '键盘连点器' })).toBeInTheDocument()
      expect(screen.getByText('绑定窗口')).toBeInTheDocument()
      expect(screen.getByText('配置按键')).toBeInTheDocument()
      expect(screen.getByText('开始运行')).toBeInTheDocument()
    })
  })
})
