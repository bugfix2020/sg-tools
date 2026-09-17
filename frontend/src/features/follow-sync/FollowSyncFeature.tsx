import { useEffect, useMemo, useState } from 'react'
import { Alert, Button, Card, Divider, Flex, Input, List, Space, Tag, Typography, message } from 'antd'
import { AimOutlined, BranchesOutlined, CheckCircleOutlined, CloseOutlined, DeleteOutlined, InfoCircleOutlined, PauseCircleOutlined, PlayCircleOutlined, PlusOutlined, SyncOutlined, WarningOutlined } from '@ant-design/icons'
import { useFollowSyncStore } from './use-follow-sync-store'
import { formatWindow, parseRuleText, type RuleConfig, windowPickDescription } from './follow-sync.utils'

function windowDescription(window?: { title: string; processName: string; pid: number }) {
  if (!window) return '尚未选择窗口'
  return `${formatWindow(window)} · ${window.processName || '未知进程'} · PID ${window.pid || '—'}`
}

export function FollowSyncFeature() {
  const store = useFollowSyncStore()
  const [draftRules, setDraftRules] = useState<RuleConfig>(store.snapshot.rules)
  const [savingRules, setSavingRules] = useState(false)
  const running = store.snapshot.state === 'running'
  const ready = Boolean(store.snapshot.main) && store.snapshot.follows.length > 0
  const pickerRole = store.pickerRole
  const pickingMain = pickerRole === 'main'
  const pickingFollow = pickerRole === 'follow'
  const pickerTarget = pickerRole && store.preview?.role === pickerRole ? store.preview.target : undefined
  const picking = Boolean(pickerRole)
  const pickDisabled = running || picking

  useEffect(() => setDraftRules(store.snapshot.rules), [store.snapshot.rules])

  const conflict = useMemo(() => {
    const excludes = new Set(draftRules.exclude.map((rule) => rule.toLowerCase()))
    return draftRules.include.some((rule) => excludes.has(rule.toLowerCase()))
  }, [draftRules])

  const saveRules = async () => {
    if (conflict) {
      message.error('同一个按键不能同时出现在包含和排除集合中')
      return
    }
    setSavingRules(true)
    try {
      await store.setRules(draftRules)
      message.success('同步规则已保存')
    } catch (error) {
      message.error(error instanceof Error ? error.message : '保存同步规则失败')
    } finally {
      setSavingRules(false)
    }
  }

  const pick = async (action: () => Promise<void>) => {
    try {
      await action()
    } catch (error) {
      message.error(error instanceof Error ? error.message : '窗口选择失败')
    }
  }

  const ruleInput = (key: keyof RuleConfig, placeholder: string) => (
    <Input value={draftRules[key].join(', ')} disabled={running || picking} placeholder={placeholder} onChange={(event) => setDraftRules((current) => ({ ...current, [key]: parseRuleText(event.target.value) }))} />
  )

  return (
    <>
      <Flex className="feature-toolbar" align="center" justify="space-between" wrap gap={12}>
        <div>
          <Typography.Title level={2} className="feature-title">主窗口同步</Typography.Title>
          <Typography.Text type="secondary">捕获主窗口键盘输入，并按原始 down/up 生命周期广播到多个跟随窗口。</Typography.Text>
        </div>
        <Tag color={picking ? 'processing' : running ? 'processing' : ready ? 'success' : 'default'} icon={picking ? <AimOutlined /> : running ? <SyncOutlined spin /> : ready ? <CheckCircleOutlined /> : <InfoCircleOutlined />}>
          {picking ? `正在选择${pickerRole === 'main' ? '主窗口' : '跟随窗口'}` : running ? '同步运行中' : ready ? '已准备' : '待绑定窗口'}
        </Tag>
      </Flex>

      <Alert className="sync-boundary-alert" type="info" showIcon icon={<InfoCircleOutlined />} message="仅主窗口拥有运行控制" description="跟随窗口只接收广播，不支持单独启动、停止或取消。窗口句柄仅在本次运行期间有效，重启 SG Tools 后需要重新绑定。" />

      <Card className="workflow-card" title={<Space><span className="step-index">01</span><BranchesOutlined />窗口关系</Space>} extra={<Button type="text" danger disabled={running || picking} onClick={() => void pick(store.clearTargets)}>清空绑定</Button>}>
        <Flex vertical gap={14}>
          {pickerRole && <Alert
            className={`sync-picker-alert ${pickerTarget ? 'has-target' : ''}`}
            type={pickerTarget ? 'success' : 'warning'}
            showIcon
            icon={<AimOutlined />}
            message={<Space><Typography.Text strong>按住鼠标左键拖到目标窗口</Typography.Text><Tag color={pickerTarget ? 'success' : 'warning'}>{pickerTarget ? '候选已锁定' : '等待移动'}</Tag></Space>}
            description={windowPickDescription(pickerRole, pickerTarget)}
            action={<Button type="link" size="small" icon={<CloseOutlined />} onClick={() => void pick(store.cancelWindowPick)}>取消选择</Button>}
          />}
          <Flex data-testid="sync-main-window-row" className={`sync-window-row ${pickingMain ? 'is-picking' : ''} ${pickingMain && pickerTarget ? 'has-candidate' : ''}`} align="center" justify="space-between" gap={14} wrap>
            <Flex align="center" gap={12}>
              <div className={`sync-window-icon main ${pickingMain ? 'is-picking' : ''}`}><AimOutlined /></div>
              <div>
                <Typography.Text strong>{pickingMain ? pickerTarget ? '当前候选主窗口' : '正在扫描主窗口' : '主窗口'}</Typography.Text>
                <Typography.Paragraph className="compact-paragraph" type="secondary">{pickingMain ? windowPickDescription('main', pickerTarget) : windowDescription(store.snapshot.main)}</Typography.Paragraph>
              </div>
            </Flex>
            <Button type="primary" icon={<AimOutlined />} disabled={pickDisabled} onMouseDown={(event) => { event.preventDefault(); void pick(store.beginMainWindowPick) }}>{pickingMain ? '继续拖动，松开完成' : '按住选择主窗口'}</Button>
          </Flex>
          <Divider className="sync-divider" />
          <Flex align="center" justify="space-between" wrap gap={10}>
            <Space><Typography.Text strong>跟随窗口</Typography.Text><Tag>{store.snapshot.follows.length}</Tag></Space>
            <Button icon={<PlusOutlined />} disabled={pickDisabled} onMouseDown={(event) => { event.preventDefault(); void pick(store.beginFollowWindowPick) }}>{pickingFollow ? '继续拖动，松开完成' : '按住添加跟随窗口'}</Button>
          </Flex>
          <List
            className="sync-follow-list"
            bordered={store.snapshot.follows.length > 0}
            locale={{ emptyText: '至少添加一个跟随窗口后才能启动同步' }}
            dataSource={store.snapshot.follows}
            renderItem={(window, index) => <List.Item actions={[<Button key="remove" type="text" danger icon={<DeleteOutlined />} disabled={running || picking} aria-label={`移除跟随窗口 ${index + 1}`} onClick={() => void pick(() => store.removeFollowWindow(index))} />]}><List.Item.Meta avatar={<div className="sync-window-icon follow"><BranchesOutlined /></div>} title={formatWindow(window)} description={`${window.processName || '未知进程'} · PID ${window.pid || '—'}`} /></List.Item>}
          />
          {pickingFollow && <div className={`sync-pick-candidate ${pickerTarget ? 'has-candidate' : ''}`} data-testid="sync-follow-window-candidate">
            <div className="sync-window-icon follow is-picking"><AimOutlined /></div>
            <div>
              <Typography.Text strong>{pickerTarget ? `当前候选跟随窗口：${formatWindow(pickerTarget)}` : '正在扫描跟随窗口'}</Typography.Text>
              <Typography.Paragraph className="compact-paragraph" type="secondary">{windowPickDescription('follow', pickerTarget)}</Typography.Paragraph>
            </div>
          </div>}
        </Flex>
      </Card>

      <Card className="workflow-card" title={<Space><span className="step-index">02</span><span>按键规则</span></Space>} extra={<Button type="primary" loading={savingRules} disabled={running || picking || conflict} onClick={() => void saveRules()}>保存规则</Button>}>
        <Typography.Paragraph type="secondary">输入浏览器键码，例如 <Typography.Text code>KeyA</Typography.Text>、<Typography.Text code>Space</Typography.Text>、<Typography.Text code>F1</Typography.Text>，多个按键用逗号分隔。包含集合非空时作为允许列表。</Typography.Paragraph>
        <Flex vertical gap={14}>
          <div><Typography.Text strong>包含按键</Typography.Text><Typography.Text type="secondary" className="hotkey-hint">留空表示允许所有按键（再应用排除集合）</Typography.Text>{ruleInput('include', '例如 KeyA, Space, F1')}</div>
          <div><Typography.Text strong>排除按键</Typography.Text><Typography.Text type="secondary" className="hotkey-hint">排除优先级高于包含；冲突会被拒绝</Typography.Text>{ruleInput('exclude', '例如 Escape, Tab')}</div>
        </Flex>
        {conflict && <Typography.Paragraph type="danger" className="compact-paragraph"><WarningOutlined /> 包含和排除集合存在冲突，请移除重复按键。</Typography.Paragraph>}
      </Card>

      <Card className="workflow-card run-card" title={<Space><span className="step-index">03</span><span>运行控制</span></Space>}>
        <Flex align="center" justify="space-between" wrap gap={16}>
          <div>
            <Typography.Title level={4} className="run-title">{running ? '主窗口正在广播输入' : '准备就绪后开始同步'}</Typography.Title>
            <Typography.Text type="secondary">已捕获 {store.snapshot.capturedCount} 个键盘事件{store.snapshot.lastCode ? ` · 最近 ${store.snapshot.lastCode}` : ''}</Typography.Text>
            {store.snapshot.lastError && <Typography.Paragraph type="danger" className="compact-paragraph"><WarningOutlined /> {store.snapshot.lastError}</Typography.Paragraph>}
          </div>
          <Button className="run-button" type={running ? 'default' : 'primary'} danger={running} size="large" disabled={!running && (!ready || picking)} icon={running ? <PauseCircleOutlined /> : <PlayCircleOutlined />} onClick={() => void pick(running ? store.stop : store.start)}>
            {running ? '停止主窗口同步' : '启动主窗口同步'}
          </Button>
        </Flex>
      </Card>
    </>
  )
}
