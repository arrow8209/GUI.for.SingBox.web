# 合并上游 GUI.for.SingBox v1.23.1 到 Web fork 的设计

**日期**：2026-04-14
**分支基线**：`main` @ a76614f（fork 最后一个提交）
**上游锚点**：`GUI-for-Cores/GUI.for.SingBox@v1.23.1`（2026-04-13 发布）
**上游最后合并点**：`v1.15.1`（commit 53402d4）
**跨度**：v1.16.0 → v1.17.0 → v1.18.0 → v1.19.0 → v1.20.0 → v1.21.0 → v1.22.0 → v1.23.0 → v1.23.1，共约 8 个中间版本

---

## 1. 目标与范围

### 目标

最终产物：当前 fork 的功能集等同于上游 `GUI-for-Cores/GUI.for.SingBox@v1.23.1`，但仍以 **Go HTTP + Vue Web 形态**运行，保留本地全部业务自定义功能（登录鉴权、Core Proxy、VLESS Reality/Trojan 入站、自定义入站 JSON、Shadowsocks 密码生成、下载代理、订阅 TLS 错误提示、构建 git hash 等）。

### In-scope

- 合并上游 v1.15.1 → v1.23.1 所有业务代码改动（前端 views/stores/utils/components、后端 `main.go`/`pkg/**`、内核交互、订阅/规则/配置解析等）。
- 对前端实际调用到的新 bridge 方法，在本地 `main.go` + `pkg/eventbus` + `frontend/src/bridge/*.ts` 里补齐 HTTP/WS 版实现。

### Out-of-scope

- 上游纯 Wails 桌面特性（托盘、WebView2、窗口 API、桌面快捷键、原生通知中的 Wails 专用部分）—— stub 或跳过。
- 上游在 `bridge/**`、`frontend/src/bridge/wailsjs/**` 里前端 views 并不调用的方法 —— 不必移植。
- Release 流程 / 打包脚本（`.github/workflows/**`）—— 本 fork 另有发布方式，不与上游对齐。
- 超出 v1.23.1 的更新（未来再起新一轮）。

### 交付物

1. 合并后的 `main` 分支（经 `feature/merge-upstream-v1.23.1` 整合后 no-ff 合回）。
2. 每小版本一份《冲突决策清单》：`docs/merge-upstream/v1.X.Y-decisions.md`。
3. 每小版本的验证记录：`docs/merge-upstream/v1.X.Y-verification.md`。
4. 合并前置扫描报告：`docs/merge-upstream/pre-scan.md`（实施第一步产出）。

---

## 2. 工作流与 git 布局

### 一次性初始化

```bash
git remote add upstream https://github.com/GUI-for-Cores/GUI.for.SingBox.git
git fetch upstream --tags
git branch feature/merge-upstream-v1.23.1 main
git worktree add ../GUI.for.SingBox.web-merge feature/merge-upstream-v1.23.1
```

- **Remote 名**：`upstream`
- **Worktree 路径**：`../GUI.for.SingBox.web-merge/`（与主仓库同级）
- **合并分支**：`feature/merge-upstream-v1.23.1`

原目录 `GUI.for.SingBox.web/` 留给日常迭代/热修；合并工作全部在 `GUI.for.SingBox.web-merge/` 进行。

### 每小版本的固定流水线

1. `git merge upstream/v1.X.Y --no-commit --no-ff`
2. 按第 3 节冲突分类策略逐块解决，同步写 `docs/merge-upstream/v1.X.Y-decisions.md`。
3. 扫描该版本是否引入前端新调用的 bridge 方法；如有，按"按需落地"实现其 HTTP/WS 版本。
4. 编译闸：`go build ./...` + `cd frontend && pnpm build`（必须全绿）。
5. 冒烟闸 + 自定义功能回归闸（见第 5 节）；结果记录到 `docs/merge-upstream/v1.X.Y-verification.md`。
6. `git commit -m "merge: upstream v1.X.Y into web fork"`
7. `git tag merge-v1.X.Y-done`（作为回滚锚点）
8. 按节奏规则汇报给用户（见 §4 节奏表）。

### 最终合回 main

- 7+ 版全部过闸后：`git checkout main && git merge feature/merge-upstream-v1.23.1 --no-ff` 产出有意义的归并点；push。
- 清理：`git worktree remove ../GUI.for.SingBox.web-merge`。

### 中途紧急修复 main 的逃生口

若合并期间 main 需要紧急修复，在原目录直接改 main 即可；合并分支不 rebase。合并分支最后合回 main 时自然会把 main 的新 commit 带上（或者合回前先在合并分支 `git merge main` 把 main 的修复并进来）。

---

## 3. 冲突决策策略

前提：用户已选"逐冲突人工判断 + 文档化分类"（Question 4 = C）。

### 冲突分类表（按文件区域）

| # | 区域 | 文件/路径 | 默认策略 |
|---|------|-----------|---------|
| ① | 已被 Web 重写的桥接层 | `bridge/*.go`（除 `app.go` 业务部分）、`frontend/src/bridge/*.ts` | 以本地为准；上游改动抽出"前端实际调用到的新方法"单独补 HTTP/WS 版 |
| ② | 已删除的 Wails 产物 | `frontend/src/bridge/wailsjs/**`、`wails.json`、`main.go` 的 Wails 入口 | 一律拒绝上游变更 |
| ③ | 前端业务 views/stores/utils | `frontend/src/views/**`、`frontend/src/stores/**`、`frontend/src/utils/**`（未进入红区者） | 以上游为准融合，保留本地业务新增 |
| ④ | 前端本地修改文件（红区） | 见下方红区清单 | 逐行判断，每块冲突写进决策清单 |
| ⑤ | 后端服务层 | `main.go`、`pkg/eventbus/**` | 本地为主；上游 `bridge/app.go`、`bridge/server.go` 中涉及的新协议/订阅/解析逻辑迁移到对应 HTTP handler；鉴权中间件、Core Proxy 必保留 |
| ⑥ | 依赖与构建 | `go.mod`、`go.sum`、`frontend/package.json`、`pnpm-lock.yaml`、`vite.config.ts`、`tsconfig*.json` | 以上游为准 + 保留本地必须（`chi`、`cors`、`gorilla/websocket`、`VITE_API_BASE` 等）；版本号冲突取较高 |
| ⑦ | 元数据/文档 | `README.md`、`CHANGES.md`、`LICENSE`、`.github/**` | 本地为准 |
| ⑧ | 静态资源/图标 | `build/**`、`public/**` | 以上游为准；本地替换的品牌资源（如 favicon）保留 |

> 合并途中若发现没覆盖到的文件，就地扩展分类表并记入当版决策清单。
>
> **优先级**：若同一文件同时出现在某行（①–③、⑤–⑧）与红区 ④ 清单，**红区策略（逐行融合 + 进决策清单）优先**。例如 `main.go` 既属 ⑤ 后端服务层、又是红区文件，按红区处理。

### 红区清单（类 ④，逐行融合）

- `frontend/src/views/LoginView.vue`
- `frontend/src/views/ProfilesView/components/InboundsConfig.vue`
- `frontend/src/views/SettingsView/components/GeneralSettings.vue`
- `frontend/src/views/SplashView.vue`
- `frontend/src/utils/request.ts`
- `frontend/src/utils/others.ts`
- `frontend/src/utils/restorer.ts`
- `frontend/src/utils/tray.ts`
- `frontend/src/utils/websockets.ts`
- `frontend/src/bridge/events.ts`
- `frontend/src/bridge/notification.ts`
- `frontend/src/bridge/browser.ts`
- `frontend/src/bridge/http.ts`
- `frontend/src/bridge/app.ts`
- `frontend/src/bridge/exec.ts`
- `frontend/src/bridge/index.ts`
- `frontend/src/bridge/io.ts`
- `frontend/src/bridge/mmdb.ts`
- `frontend/src/bridge/net.ts`
- `frontend/src/bridge/server.ts`
- `frontend/src/bridge/window.ts`
- `frontend/src/stores/auth.ts`（全新文件，无冲突但需确保保留）
- `frontend/src/stores/appSettings.ts`、`frontend/src/stores/kernelApi.ts`、`frontend/src/stores/subscribes.ts`（本地改过下载代理/core bearer 等）
- `frontend/src/api/kernel.ts`
- `frontend/src/types/profile.d.ts`、`frontend/src/types/app.d.ts`
- `frontend/src/lang/locale/en.ts`、`frontend/src/lang/locale/zh.ts`
- `frontend/src/utils/generator.ts`
- `frontend/src/utils/env.ts`
- `frontend/src/constant/kernel.ts`、`frontend/src/constant/profile.ts`
- `frontend/src/enums/kernel.ts`
- `frontend/src/router/index.ts`、`frontend/src/router/routes.ts`
- `frontend/src/App.vue`
- `frontend/src/assets/styles/custom.less`
- `frontend/index.html`
- `frontend/env.d.ts`
- `frontend/tsconfig.app.json`
- `frontend/vite.config.ts`
- `main.go`
- `bridge/app.go`、`bridge/app_control.go`、`bridge/exec.go`、`bridge/net.go`、`bridge/notification.go`、`bridge/server.go`、`bridge/types.go`

> 清单是起点；合并中遇到上游改动命中的其他本地修改文件，即时入红区。

### 《冲突决策清单》模板

路径：`docs/merge-upstream/v1.X.Y-decisions.md`

```markdown
# v1.X.Y 合并冲突决策清单

## 概览
- 冲突文件数：N
- 决策分布：保本地 X / 采上游 Y / 手动融合 Z

## 逐块决策
### <path/to/file>:<hunk 范围>
- **区域分类**：④ 前端本地修改文件
- **冲突性质**：上游重构了 InboundsConfig 的 tab 结构，本地加了 Reality 字段
- **决策**：手动融合——保留本地 Reality 字段，采纳上游新增 Hysteria2 支持
- **备注**：上游 `showAdvanced()` 逻辑本地无等价，需确认是否要引入
```

### 升级中断规则（必须停下汇报的场景）

遇到以下情况必须停下、不得自行决策：

1. 冲突涉及**鉴权中间件结构**变更（`authMiddleware`、token 生成/校验、`/api/login`、`/api/logout`）。
2. 上游重构了本地新增功能所在的**核心文件**（例如 `main.go` 的路由组织被改写）。
3. 上游**删除**了本地业务依赖的某个方法/字段。
4. 冲突分类**无法归入**表 ①–⑧ 之一。
5. 上游引入**新协议/内核能力类型**（Hysteria2、AnyTLS、WireGuard 入站等）且与本地自定义入站/Reality/Trojan 代码路径交叉时——由用户拍板是否采纳及如何并存。

---

## 4. 逐版本推进计划

按第 2 节的固定流水线，每版独立过闸。下表"预估关注点"是未看 diff 的先验判断，实际 diff 抓到后（实施计划的第一步"合并前置扫描"）再更新为该版的具体清单。

| 版本 | 预估关注点 | 预估风险 | 检查点节奏 |
|------|-----------|---------|------------|
| v1.16.0 | 首次面对 Wails bridge 演进 + 前端普通更新交织，冲突密度最高 | **高** | B：停下汇报 |
| v1.17.0 | 沿用 v1.16 同类冲突模式；应开始进入稳态 | 中 | 看 v1.16 结果决定 |
| v1.18.0 | 预期纯功能增强 | 中 | C：无异常直推 |
| v1.19.0 | 预期纯功能增强 | 中 | C |
| v1.20.0 | 关键节点：跨大段版本后一次阶段冒烟 + 回归 | 中 | B：停下汇报 |
| v1.21.0 | 预期 UI/功能增强 | 中 | C |
| v1.22.0 | 预期 UI/功能增强 | 中 | C |
| v1.23.0 | 预期较大新功能 | 中-高 | B：停下汇报 |
| v1.23.1 | 紧跟 v1.23.0 的修复版，通常冲突极少 | 低 | 直推 + 收尾 |

### 节奏规则

- 默认 **B（每版停下汇报）** 从 v1.16.0 起步。
- 过 v1.16.0 后由用户决定是否切到 **C（关键节点才停）**：v1.20.0、v1.23.0、v1.23.1 仍必停。
- 升级中断规则（§3）触发时，无视节奏规则强制停下。

### 工时粗估（不承诺，仅作预算）

- v1.16：0.5 ~ 1 天
- v1.17 ~ v1.22（6 版）：合计 1 ~ 2 天
- v1.23.0：0.5 天
- v1.23.1：< 0.5 天
- **总计 ≈ 2.5 ~ 4 天**有效推进时间（未含用户 review 等候）。

### 失败退路

- 每版 tag `merge-v1.X.Y-done`：任何一版出现无法解决的冲突或验证失败，`git reset --hard merge-v1.(X-1).Y-done` 退回上一版稳定点，重新规划该版策略。
- 整条合并失败：`git worktree remove ../GUI.for.SingBox.web-merge` + `git branch -D feature/merge-upstream-v1.23.1`，main 毫发无损。
- tag 会独立保留，删分支不会删 tag；随时可以 `git checkout -b retry merge-v1.(X-1).Y-done` 从某版重开。

---

## 5. 验证与验收

### 编译闸（必须全绿）

```bash
go build ./...
cd frontend && pnpm install && pnpm build
cd frontend && pnpm lint   # 警告容忍，错误必改
```

### 冒烟闸

1. `./gui-singbox` 正常启动，`:22345` 可访问。
2. WebSocket `/ws` 能连接；`/api/**` 可用。
3. 首页、订阅页、配置页、规则集页、插件页、日志页、设置页逐一打开，无控制台报错、无明显渲染异常。
4. 启停 sing-box 内核：下载/启动/停止/查看 PID 正常。

### 自定义功能回归闸（12 项必须全绿）

1. **单管理员鉴权**：首次启动自动生成 `data/auth.yaml`（默认 `admin/admin123`）；未登录受保护路由返回 401；正确凭据登录拿到 token；过期 token 前端触发 `forceLogout`（见 a76614f）。
2. **WebSocket 鉴权**：`/ws?token=xxx` 能用 query 参数通过鉴权。
3. **Core Proxy `/api/core/*`**：前端经 X-Core-Base + X-Core-Bearer 代理访问运行中 sing-box HTTP API；hop-by-hop header 过滤正确，可用。
4. **Reality 公钥生成 `/api/reality/public-key`**：给私钥返回对应公钥。
5. **VLESS Reality 入站**：面板能编辑 Reality 字段并持久化；生成的 sing-box config 含正确 `tls.reality` 段；"导出分享链接"产出合法 `vless://...?security=reality&...`。
6. **Trojan TLS 入站**：面板能编辑 Trojan + TLS；导出合法 `trojan://...` 分享链接。
7. **自定义入站 JSON 编辑器**：CodeMirror 可打开，非法 JSON 有红色提示，合法则能保存。
8. **Shadowsocks 密码生成**：按按钮生成合法长度/字符集密码。
9. **下载代理（configurable download proxy）**：设置页能配 `downloadProxy`；订阅/内核下载经该代理。
10. **订阅 TLS 错误友好提示**：https 错证书 URL 订阅时 UI 提示可读（见 a76614f）。
11. **构建 git hash 显示**：about/settings 区显示当前构建 commit hash（见 81fec33）。
12. **Exit API**：`/api/exit` 能优雅退出进程。

### 放行标准

- 编译闸：100% 通过，任何一项失败不放行。
- 冒烟闸：核心导航页面全部可访问即可；非核心 UI 瑕疵可记入 TODO 但放行。
- 回归闸：12 项必须全部通过；任一失败必须定位到是哪次合并引入并修复后才放行。

### 最终验收（合回 main 前）

- 全部 7+ 版的《决策清单》+《验证记录》文档齐全。
- 最终 HEAD 上执行一次完整编译闸 + 冒烟闸 + 回归闸。
- 用户 review 合并分支的 `git log` 与决策文档，确认无遗漏。

---

## 6. 风险、未知与逃生

### 主要风险

1. **上游重构订阅/配置生成核心模块**：7 版跨度里 `frontend/src/utils/generator.ts`、`frontend/src/stores/subscribes.ts`、`frontend/src/types/profile.d.ts` 若被上游大幅改写，本地基于 v1.15.1 假设的 VLESS Reality/Trojan/自定义入站 JSON/SS 密码生成等逻辑会大面积冲突。
   - **预案**：归入红区，逐行融合；v1.16 合并时若发现此类重构，立即升级为 B 节奏停下汇报。
2. **Wails 特有的内核交互 API 被上游替换为新 IPC/桥接机制**：可能与本地 Core Proxy 架构冲突。
   - **预案**：Core Proxy 保留本地实现；上游新加的 kernel 交互方法按"按需落地"转成 HTTP/WS 版。
3. **上游 go.mod 升级 Go 版本或引入与 chi/gorilla 冲突的依赖**：本地 `go 1.24.0`。
   - **预案**：go.mod 依赖冲突以上游为基础 + 强制保留 chi/cors/gorilla/websocket。
4. **i18n 文案键名撞车**：本地加了 `settings.downloadProxy` 等键。
   - **预案**：文案合并取并集，冲突键优先本地。
5. **构建产物意外进 diff**：本 fork 历史里有 `guiforcores`（14M 二进制）、`frontend/dist/` 等。
   - **预案**：合并时忽略上游对应内容；合并后扫描 `.gitignore` 是否完备并清理误提交。

### 未知信息（实施第一步要揭示）

- 上游 v1.15.1..v1.23.1 总计多少 commit？哪些是功能/bug 修复/纯 Wails 桌面改动？
- 上游 7+ 版里是否新增了本地已实现的功能（Reality 生成、自定义入站、下载代理等）？若有，冲突解决要特别小心避免双份逻辑共存。
- 上游 `bridge/*.go` 里新增了前端实际调用的方法多少个？

**揭示方式**：实施计划第 1 步 —— 执行 `git log upstream/v1.15.1..upstream/v1.23.1 --stat` + 读各版 release notes，产出《合并前置扫描报告》（`docs/merge-upstream/pre-scan.md`），把未知变成已知后再开 v1.16 的合并。

### 整条退路

- 任何阶段：`git worktree remove ../GUI.for.SingBox.web-merge` + `git branch -D feature/merge-upstream-v1.23.1`，main 无影响。
- 部分进度保留：tag `merge-v1.X.Y-done` 持续存在，随时可以 `git checkout -b retry merge-v1.(X-1).Y-done` 从某版重开。
