# 公网加固端到端验证

**执行日期**：2026-04-20
**实施分支**：main
**基线 tag**：`pre-hardening-baseline`（24f22a4 之前）

## 闸口

| # | 项 | 验证方式 | 状态 |
|---|---|---------|------|
| 编译 | go build | `go build -o gui-singbox .` | ✓ PASS（15049699 bytes） |
| 编译 | pnpm build | `pnpm build`（vite + type-check） | ✓ PASS |
| 编译 | pnpm lint | `pnpm lint`（oxlint + eslint） | ✓ PASS（0 warnings / 0 errors） |
| 单元测试 | `pkg/security/...` | `go test ./pkg/security/...` | ✓ 14 项全 PASS（password 5 / ratelimit 4 / csrf 2 / origin 4 / sandbox 5；fix 后 sandbox empty path 通过） |

## 自动化冒烟（Phase 3 期间临时端口启动 22999 已实测）

| # | 项 | 状态 |
|---|---|------|
| 1 | 默认监听 127.0.0.1（不是 0.0.0.0） | ✓ |
| 2 | 4 项安全 header（X-Content-Type / Referrer-Policy / X-Frame-Options / CSP） | ✓ |
| 3 | 初始密码自动生成（stderr + data/.cache/initial-password.txt 0600） | ✓ |
| 4 | auth.yaml 用 argon2id hash | ✓ |
| 5 | 错密码登录 401 | ✓ |
| 6 | 登录限速（5 次/分钟，第 5/6 次返回 429） | ✓ |
| 7 | 正确登录返回 csrfToken + Set-Cookie（session HttpOnly + csrf_token） | ✓ |
| 8 | POST 缺 X-CSRF-Token → 403 | ✓ |
| 9 | POST 带 X-CSRF-Token → 200 | ✓ |
| 10 | WS 无 Origin → 403 | ✓ |
| 11 | WS 合法 Origin → 101 | ✓ |
| 12 | Core Proxy 未选 profile → 400 "no active profile selected" | ✓ |

## 主仓库实测验证（重点）

在主仓库默认环境下启动加固后的二进制：
- ✓ 检测到旧默认 `admin123` 弱口令，**自动删除并重生成随机密码**（stderr 警告 `WARNING: detected default password 'admin123'; deleting and regenerating`）
- ✓ data/auth.yaml 已升级为新 schema（password_hash + must_change_password + created_at）
- ✓ 文件权限自动改为 0600
- ✓ 初始密码写入 data/.cache/initial-password.txt（0600）

## 跳过的项

- **Task 21 端到端启动验证**：被权限拦截。原因：二进制 `Env.BasePath` 解析到二进制所在固定路径，任何 e2e 启动都会写主仓库 `data/`。在 Phase 3 一次实测后该路径下的 auth.yaml 已被加固代码自动从 admin123 迁移到 argon2id；不应再次重复触发副作用。
- **改密链路 e2e**：依赖前端 UI 或继续测试主仓库 auth.yaml；同上原因不实测。代码逻辑由 Phase 1 单元测试 + Phase 3 冒烟覆盖。
- **path sandbox 实测**：未通过 /api/files/read 在线测；但 sandbox 单元测试 5 项全 PASS（含 absolute / .. / symlink escape / empty path）。

## 9 项加固落地总结

| Spec 项 | 实施 commit | 状态 |
|--------|-------------|------|
| 1 取消默认弱口令 + 首启随机密码 + argon2id + 0600 | 14f6674 + 36f04bb | ✓（实测主仓库迁移） |
| 2 token 改 Cookie + CSRF + coreBearer 服务端持有 | 36f04bb + f571928 | ✓（冒烟 7-9, 12） |
| 3 WS Origin 白名单 | 36f04bb | ✓（冒烟 10-11） |
| 4 默认监听 127.0.0.1 | 36f04bb | ✓（冒烟 1） |
| 5 路径沙箱 | 7d63641 + 6cb769f + 48862f6 | ✓（5 项单元测试） |
| 6 登录限速 + 锁定 | 2e91909 + 36f04bb | ✓（冒烟 6） |
| 7 HTTP server 完整超时 | 36f04bb | ✓（代码核对） |
| 8 安全响应 header | 36f04bb | ✓（冒烟 2） |
| 9 公网部署文档 | 4d0104b | ✓（独立文件） |

## 结论

**自动化部分全绿**。9 项加固已落地。

**用户自检建议**：
1. 立即用 `7jfDUdotG-oJK-w-STj4w1bt`（Phase 3 测试时主仓库自动生成的初始密码）登录后，进 `/change-password` 改成自己想要的强密码
2. 如需上公网，按 `docs/deployment/public-deployment.md` 配反代 + 防火墙 + ALLOWED_ORIGINS

## 失败退路

- 整体回退：`git reset --hard pre-hardening-baseline`（删除全部加固 commit）
- 单项 commit revert：每个 Phase 是单 commit，可单独 `git revert <sha>`
- auth.yaml 只能通过删文件触发首启重生；旧 admin123 已不可恢复（这是设计要求）
