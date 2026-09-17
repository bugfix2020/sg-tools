import { useMemo } from 'react'
import { Button, Card, Col, Flex, InputNumber, Row, Space, Steps, Tag, Typography } from 'antd'
import { AimOutlined, CheckCircleOutlined, PauseCircleOutlined, PlayCircleOutlined, ThunderboltOutlined, WarningOutlined } from '@ant-design/icons'
import { KeyCaptureButton } from './KeyCaptureButton'
import { MAX_BINDINGS, type ProfileView, type WindowInfo } from './clicker.utils'

type Props = {
  profile: ProfileView
  preview?: WindowInfo
  hotkeys: { activeStart: string; activeStop: string }
  onBind: () => Promise<void>
  onToggle: () => Promise<void>
  onBindingChange: (index: number, code: ProfileView['bindings'][number]) => Promise<void>
}

export function ProfilePanel({ profile, preview, hotkeys, onBind, onToggle, onBindingChange }: Props) {
  const hasKeys = profile.bindings.some((binding) => binding.code)
  const step = profile.state === 'running' ? 2 : profile.target && hasKeys ? 2 : profile.target ? 1 : 0
  const picking = profile.state === 'picking'
  const displayTarget = preview ?? (picking ? undefined : profile.target)
  const isRunning = profile.state === 'running'

  const stepItems = useMemo(() => [
    { title: '绑定窗口', icon: <AimOutlined /> },
    { title: '配置按键', icon: <ThunderboltOutlined /> },
    { title: '开始运行', icon: isRunning ? <PauseCircleOutlined /> : <PlayCircleOutlined /> },
  ], [isRunning])

  return (
    <div className="profile-panel">
      <Steps current={step} status={profile.state === 'error' ? 'error' : undefined} items={stepItems} className="workflow-steps" />

      <Card id={`bind-${profile.id}`} className={`workflow-card ${picking ? 'window-pick-active' : ''}`} title={<Space><span className="step-index">01</span>窗口绑定</Space>} extra={picking ? <Tag color="processing">正在选择…</Tag> : profile.target ? <Tag color="success" icon={<CheckCircleOutlined />}>已绑定</Tag> : <Tag>未绑定</Tag>}>
        <Flex data-testid="clicker-window-pick" className={`window-pick-row ${picking ? 'is-picking' : ''}`} align="center" justify="space-between" gap={16} wrap>
          <Flex align="center" gap={14}>
            <div className={`aim-badge ${picking ? 'is-picking' : ''}`}><AimOutlined /></div>
            <div>
              <Typography.Text strong>{picking ? displayTarget?.title || '正在选择窗口' : displayTarget?.title || '尚未选择目标窗口'}</Typography.Text>
              <Typography.Paragraph type="secondary" className="compact-paragraph">
                {picking ? displayTarget ? `当前候选：${displayTarget.title || displayTarget.processName || `PID ${displayTarget.pid}`} · 继续按住鼠标左键，松开鼠标左键完成选择` : '继续按住鼠标左键，移动到目标窗口，松开鼠标左键完成选择' : displayTarget ? `PID ${displayTarget.pid || '—'} · 选中光标下的实际控件` : '按住按钮并拖动准心到目标窗口，松开后完成绑定'}
              </Typography.Paragraph>
            </div>
          </Flex>
          <Button type="primary" icon={<AimOutlined />} disabled={isRunning || picking} onMouseDown={(event) => { event.preventDefault(); void onBind() }}>{picking ? '继续移动，松开完成' : '按住选择窗口'}</Button>
        </Flex>
      </Card>

      <Card id={`keys-${profile.id}`} className="workflow-card" title={<Space><span className="step-index">02</span>按键序列</Space>} extra={<Typography.Text type="secondary">共 {MAX_BINDINGS} 组 · 间隔单位 ms</Typography.Text>}>
        <div className="binding-header"><Typography.Text type="secondary">按键</Typography.Text><Typography.Text type="secondary">间隔（毫秒）</Typography.Text><Typography.Text type="secondary">状态</Typography.Text></div>
        <Flex vertical gap={8}>
          {profile.bindings.map((binding, index) => (
            <Row key={`${profile.id}-${index}`} gutter={12} align="middle" className={binding.code ? 'binding-row configured' : 'binding-row'}>
              <Col flex="1 1 210px"><KeyCaptureButton value={binding} disabled={isRunning} onChange={(next) => void onBindingChange(index, next)} /></Col>
              <Col flex="0 0 150px">
                <Space.Compact block>
                  <InputNumber min={0} max={9_999_999} precision={0} value={binding.delayMs} disabled={isRunning || !binding.code} onChange={(value) => void onBindingChange(index, { ...binding, delayMs: value ?? 0 })} style={{ width: 'calc(100% - 34px)' }} />
                  <span className="input-suffix">ms</span>
                </Space.Compact>
              </Col>
              <Col flex="0 0 88px">{binding.code ? <Tag color="blue">已配置</Tag> : <Typography.Text type="secondary">空闲</Typography.Text>}</Col>
            </Row>
          ))}
        </Flex>
      </Card>

      <Card id={`run-${profile.id}`} className="workflow-card run-card" title={<Space><span className="step-index">03</span>运行控制</Space>}>
        <Flex align="center" justify="space-between" wrap gap={16}>
          <div>
            <Typography.Title level={4} className="run-title">{isRunning ? '正在后台运行' : profile.state === 'error' ? '运行已停止' : '准备就绪后开始'}</Typography.Title>
            <Typography.Text type="secondary">当前实例：{hotkeys.activeStart} 启动 · {hotkeys.activeStop} 停止</Typography.Text>
            {profile.lastError && <Typography.Paragraph type="danger" className="compact-paragraph"><WarningOutlined /> {profile.lastError}</Typography.Paragraph>}
          </div>
          <Button className="run-button" type={isRunning ? 'default' : 'primary'} danger={isRunning} size="large" icon={isRunning ? <PauseCircleOutlined /> : <PlayCircleOutlined />} onClick={() => void onToggle()}>
            {isRunning ? '停止当前实例' : '启动当前实例'}
          </Button>
        </Flex>
      </Card>
    </div>
  )
}
