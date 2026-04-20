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

## 公网部署安全加固（2026-04-20）

### Breaking changes

1. **API**：登录响应不再返回 `token` 字段；改用 HttpOnly Cookie 自动管理 session。前端 `localStorage.auth_token` 已废弃。任何外部脚本调 API 都需要改为 cookie + CSRF header 模式。
2. **数据存储**：`data/auth.yaml` 字段从 `password`（明文）改为 `password_hash`（argon2id）；首次启动自动迁移；旧默认 `admin/admin123` 会被删除并重新生成随机初始密码。
3. **默认行为**：服务监听地址默认从 `:22345` 改为 `127.0.0.1:22345`；公网部署需配反代或显式 `BIND=0.0.0.0:22345`。
4. **路径**：`bridge/utils.go` 的 `GetPath` 增加沙箱限制，仅允许 `data/` 子树内的相对路径；插件如有依赖宿主机其他路径会失效。
5. **WebSocket**：缺失或非白名单 `Origin` 的 WS 连接被拒（不影响浏览器，但 CLI 工具需带 Origin header）。
6. **Core Proxy**：`X-Core-Base` / `X-Core-Bearer` header 与 `coreBase` / `coreBearer` query 不再支持；前端必须先 `POST /api/core/select-profile { profileId }`，由服务端从 profile 读取 bearer。
7. **CSRF 防御**：所有状态变更请求（POST/PUT/PATCH/DELETE）必须带 `X-CSRF-Token` header，值与 `csrf_token` cookie 一致；缺失或不匹配返回 403。

### env 变量新增

| env | 默认 | 说明 |
|-----|------|------|
| `BIND` | `127.0.0.1:22345` | 监听地址 |
| `ALLOWED_ORIGINS` | `http://127.0.0.1:*,http://localhost:*` | WS Origin 白名单（多个用逗号分隔） |
| `SECURE_COOKIE` | `true` | Cookie Secure 标志（HTTPS 时必开） |
| `SESSION_TTL` | `24h` | 会话有效期 |
| `ADMIN_PASSWORD` | （空） | 强制设/重设管理员密码 |

### 首次启动

1. 运行二进制时 stderr 会打印初始随机密码（24 字符 base64url）
2. 同时写入 `data/.cache/initial-password.txt`（权限 0600）
3. 用此密码登录后立即被引导到 `/change-password` 强制改密
4. 改密后 `initial-password.txt` 自动删除
