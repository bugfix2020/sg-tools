# SG Tools

Windows 优先的桌面工具 monorepo，当前包含一个使用 Go + Wails + React + Ant Design 实现的「键盘连点器」功能。

## 当前功能

- 最多 8 个独立配置实例，每个实例固定 12 个按键槽位。
- 每个实例绑定不同目标窗口，并为每个按键设置独立间隔。
- 使用 Win32 `PostMessageW` 向光标下实际控件发送 `WM_KEYDOWN` / `WM_KEYUP`。
- 当前实例与全部实例的启动、停止，以及可即时修改的四组全局热键。
- 配置保存到 `%APPDATA%\\sg-tools\\config.json`，窗口句柄和运行状态不会持久化。
- 仅 Windows 提供可用的平台适配；其他平台返回明确的 `unsupported_platform` 错误。

当前版本暂不包含“主窗口带动多个跟随窗口”的广播模式。该模式会在后续功能中复用 `pkg/clicker` 与 Windows PostMessage 抽象单独实现。

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
pkg/clicker/                 独立可测试的调度与 profile 核心
```
