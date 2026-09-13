import { useEffect, useState } from 'react'
import { Button, Drawer, Flex, Input, Space, Typography, message } from 'antd'
import { CheckOutlined, ReloadOutlined, SettingOutlined } from '@ant-design/icons'
import { defaultHotkeys, hotkeyFromKeyboardEvent, type HotkeyConfig } from './clicker.utils'

type Props = {
  open: boolean
  value: HotkeyConfig
  onClose: () => void
  onSave: (value: HotkeyConfig) => Promise<void>
}

const fields: Array<{ key: keyof HotkeyConfig; label: string; hint: string }> = [
  { key: 'activeStart', label: '当前实例启动', hint: '默认 PageUp' },
  { key: 'activeStop', label: '当前实例停止', hint: '默认 PageDown' },
  { key: 'globalStart', label: '全局启动全部', hint: '默认 Ctrl + PageUp' },
  { key: 'globalStop', label: '全局停止全部', hint: '默认 Ctrl + PageDown' },
]

function HotkeyInput({ value, onChange }: { value: string; onChange: (value: string) => void }) {
  const [capturing, setCapturing] = useState(false)
  useEffect(() => {
    if (!capturing) return undefined
    const listener = (event: KeyboardEvent) => {
      const next = hotkeyFromKeyboardEvent(event)
      if (!next) return
      event.preventDefault()
      onChange(next)
      setCapturing(false)
    }
    window.addEventListener('keydown', listener, true)
    return () => window.removeEventListener('keydown', listener, true)
  }, [capturing, onChange])
  return <Input readOnly value={capturing ? '请按下快捷键…' : value} onClick={() => setCapturing(true)} />
}

export function HotkeySettings({ open, value, onClose, onSave }: Props) {
  const [draft, setDraft] = useState(value)
  const [saving, setSaving] = useState(false)
  useEffect(() => setDraft(value), [value, open])

  const save = async () => {
    const values = Object.values(draft)
    if (values.some((item) => !item) || new Set(values).size !== values.length) {
      message.error('快捷键不能为空且不能重复')
      return
    }
    setSaving(true)
    try {
      await onSave(draft)
      message.success('快捷键已即时生效')
      onClose()
    } catch (error) {
      message.error(error instanceof Error ? error.message : '保存快捷键失败')
    } finally {
      setSaving(false)
    }
  }

  return (
    <Drawer title={<Space><SettingOutlined />快捷键设置</Space>} open={open} onClose={onClose} width={390}
      extra={<Button type="primary" icon={<CheckOutlined />} loading={saving} onClick={save}>保存</Button>}>
      <Typography.Paragraph type="secondary">四个动作均可重新录制。快捷键由 Go 在后台注册，保存后无需重启应用。</Typography.Paragraph>
      <Flex vertical gap={18}>
        {fields.map((field) => (
          <div key={field.key}>
            <Typography.Text strong>{field.label}</Typography.Text>
            <Typography.Text type="secondary" className="hotkey-hint">{field.hint}</Typography.Text>
            <HotkeyInput value={draft[field.key]} onChange={(next) => setDraft((current) => ({ ...current, [field.key]: next }))} />
          </div>
        ))}
      </Flex>
      <Button icon={<ReloadOutlined />} onClick={() => setDraft(defaultHotkeys())} style={{ marginTop: 24 }}>恢复默认值</Button>
    </Drawer>
  )
}

