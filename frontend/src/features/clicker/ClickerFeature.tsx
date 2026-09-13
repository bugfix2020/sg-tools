import { useState } from 'react'
import { Button, Card, Flex, Modal, Space, Tabs, Tag, Typography, message } from 'antd'
import { GlobalOutlined, PauseCircleOutlined, PlayCircleOutlined, PlusOutlined, SettingOutlined, ThunderboltOutlined } from '@ant-design/icons'
import { HotkeySettings } from './HotkeySettings'
import { ProfilePanel } from './ProfilePanel'
import { useClickerStore } from './use-clicker-store'

export function ClickerFeature() {
  const store = useClickerStore()
  const [settingsOpen, setSettingsOpen] = useState(false)

  const addProfile = async () => {
    if (store.snapshot.profiles.length >= 8) {
      message.warning('最多支持 8 个配置实例')
      return
    }
    await store.createProfile()
  }

  const removeProfile = (id: string) => {
    const profile = store.snapshot.profiles.find((item) => item.id === id)
    if (!profile) return
    Modal.confirm({
      title: `关闭 ${profile.name}？`,
      content: profile.state === 'running' ? '该实例正在运行，关闭前会先停止后台按键。' : '关闭后可以重新新建配置。',
      okText: '关闭实例',
      cancelText: '取消',
      onOk: () => store.closeProfile(id),
    })
  }

  const items = store.snapshot.profiles.map((profile) => ({
    key: profile.id,
    label: <Typography.Text editable={{ onChange: (name) => void store.renameProfile(profile.id, name) }}>{profile.name}</Typography.Text>,
    children: <ProfilePanel profile={profile} preview={store.preview[profile.id]} hotkeys={store.snapshot.hotkeys} onBind={() => store.beginWindowPick(profile.id)} onToggle={() => store.toggleProfile(profile)} onBindingChange={(index, binding) => store.setBinding(profile.id, index, binding)} />,
  }))

  return (
    <>
      <Flex className="feature-toolbar" align="center" justify="space-between" wrap gap={12}>
        <div>
          <Typography.Title level={2} className="feature-title">键盘连点器</Typography.Title>
          <Typography.Text type="secondary">为不同目标窗口配置不同按键序列，支持后台独立运行。</Typography.Text>
        </div>
        <Space wrap>
          <Button icon={<PlayCircleOutlined />} onClick={() => void store.startAll()}>全局启动</Button>
          <Button icon={<PauseCircleOutlined />} onClick={() => void store.stopAll()}>全局停止</Button>
          <Button icon={<SettingOutlined />} onClick={() => setSettingsOpen(true)}>快捷键设置</Button>
        </Space>
      </Flex>

      <Card className="workspace-card" variant="borderless">
        <Flex className="workspace-meta" justify="space-between" align="center" wrap gap={8}>
          <Space><ThunderboltOutlined className="blue-icon" /><Typography.Text strong>配置实例</Typography.Text><Tag>{store.snapshot.profiles.length}/8</Tag></Space>
          <Typography.Text type="secondary"><GlobalOutlined /> 全局快捷键：{store.snapshot.hotkeys.globalStart} / {store.snapshot.hotkeys.globalStop}</Typography.Text>
        </Flex>
        <Tabs type="editable-card" hideAdd activeKey={store.snapshot.activeProfileId} onChange={(id) => void store.setActiveProfile(id)} onEdit={(target, action) => action === 'add' ? void addProfile() : removeProfile(String(target))} items={items} tabBarExtraContent={<Button type="text" icon={<PlusOutlined />} onClick={() => void addProfile()}>新建实例</Button>} />
      </Card>

      <HotkeySettings open={settingsOpen} value={store.snapshot.hotkeys} onClose={() => setSettingsOpen(false)} onSave={store.setHotkeys} />
    </>
  )
}
