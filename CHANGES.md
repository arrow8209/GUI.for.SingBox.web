# 项目总结

## 核心改造
1. 摆脱 Wails：删除原 Wails 入口（`bridge/bridge.go`、`bridge/tray.go`、`frontend/src/bridge/wailsjs/**` 等），
   由新的 Go HTTP 服务（`main.go`）统一托管静态资源、REST API（`/api/**`）和 WebSocket 事件总线（`/ws`）。
2. 事件总线：新增 `pkg/eventbus`，实现 Go ↔︎ WebSocket 的消息转发，供 `exec`、`net`、`server` 等模块替代
   `runtime.Events*` 功能。
3. 环境初始化：`bridge/app.go` 自动定位工作目录、加载 `data` 配置，缺失目录时自动创建 `data/.cache`。
4. 网络栈：`bridge/exec.go`、`bridge/net.go`、`bridge/server.go`、`bridge/notification.go` 等全部改为 HTTP+WS
   模式，并将核心 PID 写入 `data/.cache/core-process` 供前端读取。
5. 依赖精简：`go.mod` 移除 Wails 相关依赖，引入 `chi`、`cors`、`gorilla/websocket` 等。

## 前端调整
1. 重写 bridge：`frontend/src/bridge/*.ts`（app/exec/io/net/mmdb/notification/browser/events/window/http）全部改为
   fetch/WebSocket 实现，
   - `events.ts` 自动基于 `VITE_API_BASE` 计算 WS 地址。
   - `notification.ts` 改用浏览器 Notification + fallback。
   - `browser.ts` 提供 `BrowserOpenURL`、`ClipboardSetText`。
2. UI 适配：删除 Wails 特有的 `--wails-draggable`、`window` 调用；`index.html` 复原为普通 Vite 模板，去掉
   WebView2 提示。
3. 配置/工具链：`tsconfig.app.json`、`vite.config.ts` 移除 `@wails` alias；`README.md` 更新为“前端打包 + Go
   服务”的安装方式。
4. 运行体验：`utils/others.ts` 默认 UA 改为浏览器标识；`exec.ts` 在发起 HTTP 请求前即注册事件监听，防止错过
   sing-box 的启动日志。

## 构建 & 运行
- 开发：`go run .` + `VITE_API_BASE=http://127.0.0.1:22345/api pnpm dev -- --host`
- 生产：`cd frontend && pnpm build && cd .. && go build -o gui-singbox && ./gui-singbox`

## 目录说明
- `data`：运行期配置、订阅、核心二进制，含敏感信息，默认 gitignore。
- `tmp`：调试日志目录，默认 gitignore。
