import { useState } from 'react'
import { ConfigProvider, Layout, Menu, Tabs, Typography } from 'antd'
import { AppstoreOutlined, BranchesOutlined, ToolOutlined } from '@ant-design/icons'
import { ClickerFeature } from '../features/clicker/ClickerFeature'
import { FollowSyncFeature } from '../features/follow-sync/FollowSyncFeature'
import '../styles.css'

export function AppShell() {
  const [activeKey, setActiveKey] = useState('clicker')
  const features = [
    { key: 'clicker', icon: <AppstoreOutlined />, label: '键盘连点器' },
    { key: 'follow-sync', icon: <BranchesOutlined />, label: '主窗口同步' },
  ]
  return (
    <ConfigProvider theme={{ token: { colorPrimary: '#2563eb', colorBgLayout: '#f5f7fb', borderRadius: 10, fontFamily: 'Inter, "Segoe UI", system-ui, sans-serif' }, components: { Card: { boxShadowTertiary: '0 10px 28px rgba(15, 23, 42, 0.05)' }, Tabs: { itemSelectedColor: '#2563eb' } } }}>
      <Layout className="app-layout">
        <Layout.Sider className="app-sider" width={72} breakpoint="md" collapsedWidth={0} trigger={null}>
          <div className="brand-mark"><ToolOutlined /></div>
          <Menu mode="inline" selectedKeys={[activeKey]} onClick={({ key }) => setActiveKey(key)} items={features.map((feature) => ({ key: feature.key, icon: feature.icon, label: feature.label }))} />
        </Layout.Sider>
        <Layout>
          <Layout.Header className="app-header">
            <div><Typography.Text className="brand-name">SG Tools</Typography.Text><Typography.Text type="secondary" className="brand-subtitle">Windows utility workspace</Typography.Text></div>
            <Typography.Text type="secondary">桌面工具箱</Typography.Text>
          </Layout.Header>
          <Layout.Content className="app-content">
            <Tabs className="feature-tabs" activeKey={activeKey} onChange={setActiveKey} items={features.map((feature) => ({ key: feature.key, label: feature.label, children: feature.key === 'clicker' ? <ClickerFeature /> : <FollowSyncFeature /> }))} />
          </Layout.Content>
        </Layout>
      </Layout>
    </ConfigProvider>
  )
}
