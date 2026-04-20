# 公网部署安全加固 · 设计

**日期**：2026-04-20
**触发**：codex 安全审核（4 项严重 + 4 项高/中）
**目标部署场景**：公网直连（A 选项）
**实施范围**：A 全做（9 项一次性完成）

---

## 1. 目标与范围

### 目标

把 GUI.for.SingBox.web 从"内网/本机自用"安全等级提升到"可公网直连"等级，覆盖 codex 审核的全部 4 项严重 + 4 项高/中。

### In-scope（9 项）

| # | 项 | 严重度 |
|---|---|--------|
| 1 | 取消默认弱口令；首次启动生成随机密码；argon2id 哈希；config 文件 0600 | 严重 |
| 2 | token 改 HttpOnly Cookie + CSRF 双提交；coreBearer 服务端持有，不再前端透传 | 严重 |
| 3 | WebSocket Origin 白名单（默认仅 loopback；env 配置） | 高 |
| 4 | 默认监听 `127.0.0.1:22345`（env 显式覆盖才能开 0.0.0.0） | 高 |
| 5 | `GetPath` 路径沙箱：拒绝绝对路径与 `..`；限制在 `data/` 子树 | 高 |
| 6 | 登录限速 + 锁定（每 IP + 每用户名 5 次/分钟，触发后锁 5 分钟） | 中 |
| 7 | HTTP server 完整超时（ReadHeader/Read/Write/Idle） | 中 |
| 8 | 安全响应 header（CSP/X-Content-Type-Options/Referrer-Policy/Frame-Options） | 中 |
| 9 | 文档：公网部署 checklist（HTTPS 反代示例 + env 配置参考） | 中 |

### Out-of-scope

- **操作审计日志**（exec/io/net 写 `data/audit.log`）—— 后续单独项目，不阻塞上线
- **2FA/OAuth/SSO** —— 后续按需
- **请求体流式处理**（中危 #8 "请求体整包读入内存"）—— 当前文件 IO 都有自然大小上限（profile/auth.yaml 几十 KB），暂时不动；只有 `/api/files/write` 大文件场景才需要，可后续做
- **Cookie 体系下的多用户** —— 当前是单管理员，不引入多用户

### 交付物

1. main 分支上的 commit 链（建议拆 9 个独立 commit，每项一个）
2. `data/auth.yaml` schema 升级 + 自动迁移逻辑
3. 公网部署文档：`docs/deployment/public-deployment.md`
4. 升级影响说明：`CHANGES.md` 追加 breaking changes 段

---

## 2. 数据模型变更

### `data/auth.yaml` schema

**旧**：
```yaml
username: admin
password: admin123
```

**新**：
```yaml
username: admin
password_hash: $argon2id$v=19$m=65536,t=3,p=4$<salt>$<hash>
must_change_password: true
created_at: 2026-04-20T10:00:00Z
```

**自动迁移**：
- 启动时检测旧格式（含 `password` 字段而非 `password_hash`）：若是默认 `admin/admin123` → 删文件按"首次启动"处理；若是用户自定义密码 → 自动转换为 hash + 保留用户名 + 标记 `must_change_password: false`（认为用户已知道密码）
- 文件权限统一改为 `0600`，目录 `data/` 改为 `0700`

### CSRF token 存储

- 登录成功后服务端生成 32 字节随机 CSRF token
- 写入响应：`Set-Cookie: csrf_token=<value>; Path=/; SameSite=Strict`（**不**用 HttpOnly，前端 JS 需要读）
- 同时写入响应 body：`{ "csrfToken": "<value>" }`
- 前端把它存到 `sessionStorage`，请求 POST/PUT/DELETE 时加 `X-CSRF-Token` header
- 服务端校验 cookie 与 header 一致

### Session token

- 登录成功后服务端生成 32 字节随机 session token，存内存 `sessions map[string]session`（沿用现有结构）
- 写入响应：`Set-Cookie: session=<value>; Path=/; HttpOnly; SameSite=Strict; Secure`（HTTPS 时 Secure；通过 env 控制）
- WebSocket 升级时浏览器自动带 cookie，服务端校验
- **前端 `auth_token` localStorage 不再使用**，登录返回也不再返回 token 字段（兼容性破坏，但本 fork 是单 client 全替换）

---

## 3. 后端改造（Go）

### `main.go`

新增/修改的关键函数与中间件：

```go
// 配置
type SecurityConfig struct {
    BindAddr        string   // env BIND 默认 "127.0.0.1:22345"
    AllowedOrigins  []string // env ALLOWED_ORIGINS 默认 ["http://127.0.0.1:*"]
    SecureCookie    bool     // env SECURE_COOKIE 默认 true（公网必开）
    SessionTTL      time.Duration // 默认 24h
    PasswordResetEnv string  // env ADMIN_PASSWORD 用于初始化或强制覆盖
}

// 限速 middleware
type rateLimiter struct {
    mu       sync.Mutex
    attempts map[string][]time.Time // key: IP 或 username
    window   time.Duration          // 1 min
    max      int                    // 5
    lockout  time.Duration          // 5 min
    locks    map[string]time.Time
}
func (rl *rateLimiter) check(key string) bool { ... }
func (rl *rateLimiter) record(key string) { ... }

// 安全 header middleware
func securityHeaders(next http.Handler) http.Handler { ... }

// CSRF middleware（仅校验 P/U/D 方法）
func csrfMiddleware(next http.Handler) http.Handler { ... }

// argon2id
func hashPassword(plain string) (string, error) { ... } // golang.org/x/crypto/argon2
func verifyPassword(plain, hash string) bool { ... }

// 首次密码生成
func generateInitialPassword() (string, error) { ... } // crypto/rand 24 字节 base64
```

### `bridge/utils.go` GetPath 沙箱

```go
func GetPath(relPath string) string {
    base := filepath.Join(Env.BasePath, "data")
    cleaned := filepath.Clean(relPath)

    // 拒绝绝对路径
    if filepath.IsAbs(cleaned) {
        log.Printf("GetPath rejected absolute path: %s", relPath)
        return ""  // 调用方应检查空串
    }

    // 拒绝 ..
    if strings.Contains(cleaned, "..") {
        log.Printf("GetPath rejected path traversal: %s", relPath)
        return ""
    }

    full := filepath.Join(base, cleaned)
    // 防止 symlink 越狱：用 EvalSymlinks 后再次校验前缀
    resolved, err := filepath.EvalSymlinks(full)
    if err == nil && !strings.HasPrefix(resolved, base) {
        log.Printf("GetPath rejected symlink escape: %s -> %s", relPath, resolved)
        return ""
    }

    return full
}
```

**调用方影响**：现有 `bridge/io.go`、`bridge/exec.go` 等所有用 GetPath 的地方都得加空串检查，返回错误而非继续执行。

### `pkg/eventbus/bus.go` Origin 校验

```go
var allowedOrigins []string  // 启动时从配置注入

upgrader := websocket.Upgrader{
    CheckOrigin: func(r *http.Request) bool {
        origin := r.Header.Get("Origin")
        if origin == "" {
            return false  // 拒绝无 Origin 的连接（curl 等）
        }
        for _, allowed := range allowedOrigins {
            if matchOrigin(allowed, origin) { return true }
        }
        return false
    },
}
```

`matchOrigin` 支持精确匹配 + 端口通配（`http://127.0.0.1:*`）。

### `main.go` Core Proxy 不再透传 bearer

```go
func (s *Server) handleCoreProxy(w http.ResponseWriter, r *http.Request) {
    // 之前：从 X-Core-Bearer header / coreBearer query 读
    // 现在：从当前会话用户的 active profile 读
    profile, err := s.getActiveProfile()  // 服务端从 data/profiles.yaml + appSettings 读
    if err != nil { ... }
    coreBase := profile.Experimental.ClashAPI.ExternalController
    bearer := profile.Experimental.ClashAPI.Secret
    // ... 用 coreBase + bearer 代理 ...
}
```

> **注意**：这里需要服务端能 parse profile YAML。可以读 `data/profiles.yaml` 并复用 `bridge.App.GetEnv()`-级别的 helper。或者前端只传 profile id，服务端按 id 查。**简化方案**：前端登录后通过新 API `POST /api/core/select-profile { profileId }` 显式告诉服务端"当前激活哪个 profile"，服务端缓存到会话。

### HTTP server 超时

```go
srv := &http.Server{
    Addr:              cfg.BindAddr,
    Handler:           router,
    ReadHeaderTimeout: 10 * time.Second,
    ReadTimeout:       60 * time.Second,
    WriteTimeout:      60 * time.Second,
    IdleTimeout:       120 * time.Second,
}
```

WS handler 注意：在 hijack 之前不会被 WriteTimeout 干扰；hijack 之后是裸 net.Conn，不受 http.Server 超时控制（gorilla/websocket 自己有 ping/pong）。

### 启动逻辑

```go
func main() {
    cfg := loadSecurityConfig()  // env

    // 首次启动：data/auth.yaml 不存在
    if _, err := os.Stat(authPath); errors.Is(err, fs.ErrNotExist) {
        var pwd string
        if cfg.PasswordResetEnv != "" {
            pwd = cfg.PasswordResetEnv
        } else {
            pwd, _ = generateInitialPassword()
            // 写到 stdout
            fmt.Fprintf(os.Stderr, "\n========================================\n")
            fmt.Fprintf(os.Stderr, "Initial admin password: %s\n", pwd)
            fmt.Fprintf(os.Stderr, "Stored in: %s\n", initialPasswordCache)
            fmt.Fprintf(os.Stderr, "Login at /login and change immediately.\n")
            fmt.Fprintf(os.Stderr, "========================================\n\n")
            // 写到 data/.cache/initial-password.txt (0600)
            os.WriteFile(initialPasswordCache, []byte(pwd), 0600)
        }
        hash, _ := hashPassword(pwd)
        writeAuthConfig(authPath, &AuthConfig{
            Username:           "admin",
            PasswordHash:       hash,
            MustChangePassword: true,
            CreatedAt:          time.Now().UTC(),
        })
    } else {
        // 检测是否旧格式 + 自动迁移
        migrateAuthIfNeeded(authPath)
    }
    // ...
}
```

---

## 4. 前端改造（Vue）

### `frontend/src/stores/auth.ts`

- `token` 字段废弃；改用 `csrfToken` + `mustChangePassword` 状态
- `login()` 不再 setItem(localStorage)；改为 `fetch(..., { credentials: 'include' })`
- `logout()` 调 `/api/logout` 后清 sessionStorage
- 新增：`changePassword(oldPwd, newPwd)`

### `frontend/src/api/request.ts` 与 `bridge/http.ts`

```typescript
const init: RequestInit = {
  ...,
  credentials: 'include',  // 必须，否则 cookie 不会带
  headers: {
    ...(method !== 'GET' ? { 'X-CSRF-Token': csrfToken } : {}),
    ...
  }
}
```

### `frontend/src/api/websocket.ts` 与 `bridge/events.ts`

- 删掉 `token` query 参数（cookie 自动带）
- 删掉 `coreBearer` query 参数（服务端持有）
- WebSocket 直接 `new WebSocket(url)`，浏览器自动带 cookie

### `frontend/src/api/kernel.ts`

- 删掉 `setupKernelApi`/`setupKernelWs` 里设置 `X-Core-Base/X-Core-Bearer` 的代码
- 改为：登录后 store 选 profile 时调 `POST /api/core/select-profile { profileId }`
- request/websocket 不再设 base = Core Proxy URL；直接走 `/api/core/...`

### 强制改密 UI

- 登录后若 `mustChangePassword`，路由强制跳 `/change-password`
- 改密成功后，服务端清除 `must_change_password` 字段并删除 `data/.cache/initial-password.txt`

---

## 5. 配置与文档

### env 变量清单

| env | 默认 | 说明 |
|-----|------|------|
| `BIND` | `127.0.0.1:22345` | 监听地址；公网部署改 `127.0.0.1:22345` 反代，**禁止**直接 0.0.0.0 |
| `ALLOWED_ORIGINS` | `http://127.0.0.1:*,http://localhost:*` | WS Origin 白名单，多个用 `,` 分隔 |
| `SECURE_COOKIE` | `true` | Cookie Secure 标志；只 HTTP 时设 `false` 调试 |
| `SESSION_TTL` | `24h` | 会话有效期 |
| `ADMIN_PASSWORD` | （空） | 强制设管理员密码（覆盖文件值或首启用） |
| `PORT` | （沿用） | 仅当 BIND 未设时使用，向前兼容 |

### `docs/deployment/public-deployment.md`（新增）

包含：
1. Nginx/Caddy 反代配置（HTTPS + WS upgrade + 透传 cookie）
2. 防火墙建议（只开 80/443，22345 不暴露）
3. 启动命令 systemd unit 示例
4. 首次密码获取 + 立即改密流程
5. ALLOWED_ORIGINS 配置示例

---

## 6. 兼容性与迁移

### Breaking changes

1. **API**：登录响应不再含 `token` 字段；前端必须用 cookie。任何外部脚本调 API 都要改
2. **存储**：`data/auth.yaml` 字段改名 `password` → `password_hash`，自动迁移但首次启动会重写
3. **行为**：默认监听从 `0.0.0.0` 改为 `127.0.0.1`；公网用户必须配反代或显式 `BIND=0.0.0.0:...`
4. **路径**：`data/` 之外的所有读写都被拒绝；如有插件依赖此能力会失效
5. **WS**：无 Origin 的 WebSocket 连接被拒（不影响浏览器，但 CLI 工具/老脚本受影响）

### 自动迁移逻辑

启动时若发现：
- 旧 `password: admin123` → 删 auth.yaml 走"首次启动"流程，stderr 警告
- 旧 `password: <非默认>` → 自动 hash 后写入新格式，`must_change_password: false`，stderr 提示
- 已是新格式 → 无操作

---

## 7. 风险与未知

### 风险

1. **前端 cookie 跨子域问题**：如果后端在 `api.example.com`、前端在 `panel.example.com`，cookie 默认不跨子域。需要 `Domain=.example.com` 配置。**预案**：文档明确推荐"前后端同源（反代到一个域名下）"。
2. **Core Proxy 改造的服务端 profile 选择**：若用户在 UI 中频繁切换 profile，前端必须及时调 `select-profile` 通知后端。可能引入"profile 选择状态不一致"的 bug。**预案**：`select-profile` 每次切换 profile 都同步调用，UI 切换 profile 的入口是少数几个明确点。
3. **WS upgrade 后超时**：HTTP server 的超时不应影响已建立的 WS。**预案**：手测确认；fallback 是把 WS 路由从主 mux 摘出走独立 ListenAndServe。
4. **argon2id 内存占用**：64 MB × 并发登录数 = 可能被打（DoS via login flood）。**预案**：登录限速已经 cover；64 MB 可调小到 32 MB。

### 未知信息

1. 实际反代部署是 nginx 还是 caddy？（影响文档示例）
2. 是否需要 Basic Auth 第二层（IP 白名单 + Basic Auth + 应用登录）？
3. 是否要支持反代后的 `X-Forwarded-For` 解析（影响限速准确性）

这些不影响主体设计，可在实施时由我做合理默认。

### 退路

每项独立 commit，任一项出问题可单独 revert。完整 9 项失败：`git reset --hard <pre-hardening>` 即可。

---

## 8. 与未合并工作的关系

当前 `feature/merge-upstream-v1.23.1` 分支已 push 到 origin 等审查；本设计针对 main 分支基线。两条线互不冲突——本加固完成后，合并分支可以正常合回 main，加固代码会自然带入。

但有一个交叉点：**Core Proxy `X-Core-Bearer` 改造**会动到合并分支里我特意保留并加固的代码（v1.17 / v1.20 决策）。合并完成后再做加固，相当于把那部分代码再改一次。这是预期的，不算返工。
