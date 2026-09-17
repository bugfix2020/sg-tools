# SG Tools

Windows 优先的桌面工具 monorepo，当前包含一个使用 Go + Wails + React + Ant Design 实现的「键盘连点器」功能。

## 当前功能

- 最多 8 个独立配置实例，每个实例固定 12 个按键槽位。
- 每个实例绑定不同目标窗口，并为每个按键设置独立间隔。
- 使用 Win32 `PostMessageW` 向光标下实际控件发送 `WM_KEYDOWN` / `WM_KEYUP`。
- 当前实例与全部实例的启动、停止，以及可即时修改的四组全局热键。
- 配置保存到 `%APPDATA%\\sg-tools\\config.json`，窗口句柄和运行状态不会持久化。
- 仅 Windows 提供可用的平台适配；其他平台返回明确的 `unsupported_platform` 错误。
- 「主窗口同步」选择一个主窗口和多个跟随窗口，仅由主窗口启动或停止；跟随窗口只接收广播。
- 同步规则支持包含集合和排除集合，排除优先且冲突/重复规则会被拒绝；窗口句柄只在当前运行期有效，重启后需重新绑定。

主窗口同步的发送端复用 Win32 `PostMessageW` 和现有按键 down/up 生命周期。`PostMessageW` 本身只能发送消息，不能观察真实输入；Windows 适配使用 `WH_KEYBOARD_LL` 捕获经过系统低级键盘 hook 的键盘边沿，并限定到主窗口的前台/root 窗口，同时排除 SG Tools 自身进程。完全停留在应用内部、没有进入该系统 hook 边界的输入不保证可被观察；本版本也不包含鼠标同步。

## 开发环境

- Go 1.26+
- Node.js 24+
- pnpm 11+
- Wails v2.15.0
- Windows WebView2 Runtime

```powershell
go install github.com/wailsapp/wails/v2/cmd/wails@v2.15.0
wails doctor
pnpm install
wails dev
```

## 验证与构建

```powershell
go test ./...
pnpm -r test
pnpm -r typecheck
pnpm --filter @sg-tools/desktop-frontend build
wails build -clean
```

生成的 Windows 程序位于 `build/bin/sg-tools.exe`。

## 目录

```text
app/                         Wails facade、配置和事件
frontend/                    React + Vite + Ant Design UI
internal/platform/windows/   Win32 PostMessage、窗口选择、热键适配
packages/ui/                 可复用的前端主题 token
pkg/clicker/                 独立可测试的调度与 profile 核心，以及共享按键 transition
pkg/windowsync/              独立可测试的输入捕获、过滤、广播和运行状态核心
```
