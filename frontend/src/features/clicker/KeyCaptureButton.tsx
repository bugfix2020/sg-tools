import { useEffect, useState } from 'react'
import { Button } from 'antd'
import { KeyOutlined } from '@ant-design/icons'
import { displayKeyFromCode, type KeyBinding } from './clicker.utils'

type Props = {
  value: KeyBinding
  disabled?: boolean
  onChange: (binding: KeyBinding) => void
}

export function KeyCaptureButton({ value, disabled, onChange }: Props) {
  const [capturing, setCapturing] = useState(false)

  useEffect(() => {
    if (!capturing) return undefined
    const onKeyDown = (event: KeyboardEvent) => {
      if (['Control', 'Alt', 'Shift', 'Meta'].includes(event.key)) return
      event.preventDefault()
      onChange({ ...value, code: event.code, label: displayKeyFromCode(event.code, event.key) })
      setCapturing(false)
    }
    window.addEventListener('keydown', onKeyDown, true)
    return () => window.removeEventListener('keydown', onKeyDown, true)
  }, [capturing, onChange, value])

  return (
    <Button
      className={value.code ? 'key-capture-button has-key' : 'key-capture-button'}
      icon={<KeyOutlined />}
      disabled={disabled}
      onClick={() => setCapturing(true)}
    >
      {capturing ? '请按键…' : value.label || '点击设置按键'}
    </Button>
  )
}
