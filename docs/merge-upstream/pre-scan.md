# 合并前置扫描报告 · v1.15.1 → v1.23.1

生成日期：2026-04-14
执行者：Claude Code（inline execution）
基线：`main` @ 2c6afad
上游锚点：`v1.23.1` @ 2026-04-13

---

## 关键发现：上游 v1.16 ~ v1.20 无正式 tag

执行 `git fetch upstream --tags` 后发现上游只打了 v1.21.0 / v1.22.0 / v1.23.0 / v1.23.1 这 4 个正式 tag。v1.16 ~ v1.20 仅以 "Release vX.Y.Z" 的 commit messages 存在。

**应对**：使用 Release commit SHA 代替 tag 作为合并里程碑。合并目标如下：

| 里程碑 | 合并目标（传给 `git merge` 的参数） | 类型 |
|--------|-------------------------------------|------|
| v1.16.0 | `5ecaf3de2ec84066218cbff9ff90dbe07a0dac94` | Release commit |
| v1.17.0 | `c3d40e8cb365d74bd9b60ff3f9182f804f837926` | Release commit |
| v1.18.0 | `f7d5043fc24e6ec6503285d1c3860c2c20c84be9` | Release commit |
| v1.19.0 | `d6285e0225ce3672b7e936a3c7a696275189705e` | Release commit |
| v1.20.0 | `a1ab547e2ec9c81610976ef05118d2d87a46a3a4` | Release commit |
| v1.21.0 | `v1.21.0`（或 `5d61206d7e320c8ad56d38d667bfd52d8536683c`） | 官方 tag |
| v1.22.0 | `v1.22.0` | 官方 tag |
| v1.23.0 | `v1.23.0` | 官方 tag |
| v1.23.1 | `v1.23.1` | 官方 tag |

---

## 总览

- **待合并版本**：9 版
- **总 commits**：110（v1.15.1..v1.23.1）
- **含 Wails-only 文件的版本**：v1.17.0 / v1.20.0 / v1.22.0（含 `bridge/tray.go`、`frontend/src/bridge/wailsjs/**`）

---

## 各版本详情

### v1.16.0（基线 v1.15.1）

- **commits**：16
- **file stat**：43 files changed, +1586 / −3375
- **红区文件触碰**：11 个（`bridge/bridge.go`、`App.vue`、`bridge/io.ts`、`constant/*`、`locale/*`、`stores/appSettings/kernelApi/subscribes`、`utils/generator/others`）
- **新增 bridge 方法**：无
- **关键 commits**：
  - `7ab0ede` 新增 `buildSmartRegExp` utility（可能影响 generator.ts）
  - `e75bb9e` 新增默认 FakeIP DNS server 与规则
  - `901e489` 新增 profile/subscribe template 函数
  - `d215897` 新增 profile 迁移逻辑
- **风险标签**：**高**（首版，Wails/Web 桥接层首次撞车 + 红区文件批量触碰）
- **节奏**：**B（强制停下汇报）** —— 已定

### v1.17.0

- **commits**：26
- **file stat**：124 files changed, +1569 / −1482（大批量修改）
- **红区文件触碰**：21 个（含 `main.go`、`bridge/exec.go`、`bridge/tray.go`、`wailsjs/**`、`InboundsConfig.vue`、`GeneralSettings.vue`）
- **新增 bridge 方法**：`ExitApp`、`UpdateTray`、`UpdateTrayAndMenus`（后两者 Wails-only，跳过；`ExitApp` 本地已有等价 HTTP 版 `/api/exit`）
- **关键 commits（高风险）**：
  - `8ab245e` **refactor(core-api): centralize REST and WebSocket logic into api layer** —— 与本地 Core Proxy 强相关
  - `e2c6706` **fix: use correct protocol for core api base url** —— 同上
  - `8313223` refactor(tray): unify tray and menu updates —— Wails-only，本地 stub
  - `d0d51ad` **refactor(settings): extract GeneralSettings into modular subcomponents** —— 红区 GeneralSettings.vue 被大结构重组，本地 `downloadProxy` 字段必须保留
  - `ad74167` feat(theme): add custom color support with ColorPicker
  - `0da1db0` feat: optimize settings UI and introduce v-platform directive
- **预期升级中断风险**：高
  - 规则 2 触发可能：`main.go` 路由被改
  - 规则 5 触发可能：core-api 层统一改动可能影响 Core Proxy 对 API/WS 的代理结构
- **风险标签**：**高**
- **节奏**：依 v1.16 结果由用户决定。建议 **B（停下汇报）**

### v1.18.0

- **commits**：3
- **file stat**：少量（`frontend/src/bridge/server.ts +4`）
- **红区文件触碰**：8 个，多数是 bridge 层
- **新增 bridge 方法**：无
- **关键 commits**：
  - `a782294` feat(IO): 部分文件按 byte-range 读写
  - `dc0539e` feat(server): 静态/上传路由支持 header 与 raw upload
- **风险标签**：**低**
- **节奏**：C（直推）

### v1.19.0

- **commits**：4
- **红区文件触碰**：6 个（全在 `bridge/` 下的 exec 相关）
- **新增 bridge 方法**：无
- **关键 commits**：
  - `2ee23ea` feat: add privileged mode detection and execution support
  - `9b65d13` fix: update class bindings for Tag component styles
  - `655fd5b` refactor: icon component
- **风险标签**：**低-中**（privileged mode 涉及 exec 逻辑，本地已改过 `exec.go` 为 HTTP 形式）
- **节奏**：C（直推）

### v1.20.0

- **commits**：23
- **file stat**：126 files changed, +2052 / −3159（大重构版本）
- **红区文件触碰**：17 个（含 `main.go`、`vite.config.ts`、`appSettings`、`utils/tray`、`utils/others`）
- **新增 bridge 方法**：`ExitApp`（签名改变，本地需同步）
- **关键 commits（高风险）**：
  - `7a70c8a` **refactor(websocket): simplify WS lifecycle and per-channel management** —— 与本地 /ws 路径业务化撞车
  - `93640ff` **refactor(websocket): improve WebSocket management and cleanup logic**
  - `82d7153` **refactor(bridge): apply cache-control headers globally in RollingRelease middleware**
  - `90ebe69` refactor: use http.ServeFile in RollingRelease with optimized caching
  - `b298a2f` **refactor(frontend): reorganize common components and views** —— 大量文件移动
  - `cbbdb87` / `c7c0908` refactor(tray) —— Wails-only
  - `f4637ff` Refactor code structure for improved readability and maintainability（即 `rolling-release` tag 指向的 commit）
- **预期升级中断风险**：高
  - 规则 2 触发高概率：frontend 大规模文件移动可能命中本地红区
  - 规则 5 触发可能：WebSocket 重构涉及本地已扩展的 `/ws?token=` 鉴权
- **风险标签**：**高**
- **节奏**：**B（强制停下）** —— 已定（设计文档关键节点）

### v1.21.0

- **commits**：9
- **file stat**：25 files changed, +404 / −329
- **红区文件触碰**：13 个
- **新增 bridge 方法**：无
- **关键 commits**：
  - `919a50c` feat: add WorkingDirectory option to ExecOptions —— 本地 exec.go 已重写，需要对齐
  - `f6c8f36` feat: clipboard pasting + Input 验证增强
  - `8fc0a40` feat: 延迟测试自定义 timeout
  - `c1be4cf` feat: normalizeBase64 utility（可能 conflict generator.ts）
  - `7a57e59` feat: Relax isValidBase64 to accept non-standard base64 —— 订阅解析改动
- **风险标签**：**中**
- **节奏**：C（直推）

### v1.22.0

- **commits**：18
- **file stat**：64 files changed, +2145 / −1298
- **红区文件触碰**：25 个（数量较多）
- **新增 bridge 方法**：`GetEnv(key string) any`（签名变更：从无参变为带 key 参数；本地 `bridge.App.GetEnv()` 需同步）
- **关键 commits（高风险）**：
  - `0714535` **feat: add manual profile editor and enhance config restoration** —— 可能与本地自定义入站 JSON 编辑器功能重叠
  - `1cb1bd0` **refactor: migrate to Wails native notifications and upgrade to v2.12.0** —— Wails 通知大改，本地已完全替换为浏览器 Notification
  - `224d9d3` feat(auto-start): macOS auto-start —— Wails 桌面特性，本地跳过
  - `03a650e` feat: enhance app restart logic for macOS —— 桌面特性
  - `d1d2ad6` feat(system proxy): clear app-managed system proxy when core stops
  - `64d02ae` feat: enhance config generation options and restore logic
  - `982ff42` feat: enhance profile restore and rule parsing
  - `12fb2c2` / `dbed9d5` feat(outbounds): add icon/hidden properties
- **预期升级中断风险**：中-高
  - 规则 5 触发可能：manual profile editor 与本地自定义入站 JSON 编辑器交叉
- **风险标签**：**中-高**
- **节奏**：C（直推，除非 manual profile editor 触发升级中断）

### v1.23.0

- **commits**：9
- **file stat**：29 files changed, +1234 / −942
- **红区文件触碰**：7 个
- **新增 bridge 方法**：无
- **关键 commits**：
  - `4329fb5` **refactor: migrate plugin execution to ESM modules and streamline lifecycle handling** —— 插件系统架构重构
  - `0b0f2bd` feat: adapt deprecated config fields for the latest core
  - `d30143a` feat: enhance download progress tracking and UI updates in core settings
  - `acd5143` refactor: switch macOS auto-start to LaunchAgents —— 桌面特性
  - `558265a` feat(auto-start): implement auto-start support for Linux —— 桌面特性
- **风险标签**：**中-高**（插件 ESM 迁移是架构级变更）
- **节奏**：**B（强制停下）** —— 已定

### v1.23.1

- **commits**：2
- **file stat**：6 files changed, +77 / −22
- **红区文件触碰**：2 个（`bridge/bridge.go`, `bridge/exec.go`）
- **关键 commits**：
  - `fc5c848` fix: add macOS fallbacks for process info and memory lookup
  - `be28e95` fix: remove rolling-release directory after app update on macOS
- **风险标签**：**低**（纯 macOS 修复 patch 版）
- **节奏**：收尾直推

---

## 关键发现（Heads-up）

### 1. 上游与本地可能重复实现的功能

- **v1.22.0 的 "manual profile editor"** (commit `0714535`) —— 可能与本地的"自定义入站 JSON 编辑器"（CodeMirror + 非法 JSON 提示）功能交叉。合并时需判断是取上游版本、保本地版本，还是两者并存。建议升级中断规则 5 触发时停下讨论。

### 2. 上游 core-api 重构与本地 Core Proxy 的关系

- **v1.17.0 的 `refactor(core-api): centralize REST and WebSocket logic`** 可能改变前端调用 sing-box HTTP API 的路径/方式。
- 本地 Core Proxy（`/api/core/*` + `X-Core-Base` + `X-Core-Bearer`）是 fork 的关键基础设施，不能被上游重构冲掉。
- 合并 v1.17.0 时必须仔细检查 `frontend/src/api/kernel.ts` 与 `frontend/src/utils/websockets.ts` 的改动，保证 Core Proxy 调用路径不被绕过。

### 3. v1.20.0 的 WebSocket 生命周期重构

- `refactor(websocket): simplify WS lifecycle` 和 `refactor(websocket): improve WebSocket management` 会改 WS 客户端管理。
- 本地 `/ws` 已扩展支持 `?token=` query 参数鉴权；上游重构不能删掉此能力。
- 本地 `pkg/eventbus/` 的 WS 事件总线架构可能与上游重构思路不一致，需保留本地架构。

### 4. v1.22.0 的 Wails 原生通知迁移

- 上游 `1cb1bd0 refactor: migrate to Wails native notifications` 大概率删掉旧通知代码。
- 本地已完全替换为浏览器 Notification API（`frontend/src/bridge/notification.ts`）。
- 合并时 `bridge/notification.go` 的改动一律以本地为准（按 ① 桥接层规则）。

### 5. v1.23.0 的插件 ESM 迁移

- `4329fb5 refactor: migrate plugin execution to ESM modules` 是插件系统架构级变更。
- 本 fork 的插件系统是否需要跟进 ESM 化？若用户重度依赖插件，这一版的合并价值最高；若插件使用轻度，可简化处理。
- 预期会在该版合并时停下讨论取舍。

### 6. Wails-only 改动的处理

以下类型 commit 在合并时一律以本地为准（拒绝上游）或跳过：
- 所有 `bridge/tray.go` 改动
- 所有 `frontend/src/bridge/wailsjs/**` 改动
- 所有 `refactor(tray)`、`feat(auto-start)`、macOS/Windows/Linux 桌面特性
- `bridge/bridge.go` 中涉及 Wails runtime 注册的部分

---

## 节奏计划（基于扫描更新）

| 版本 | 风险 | 计划节奏 |
|------|------|---------|
| v1.16.0 | 高 | **B（强制停下）** |
| v1.17.0 | 高 | **B（建议停下）** |
| v1.18.0 | 低 | C（直推） |
| v1.19.0 | 低-中 | C（直推） |
| v1.20.0 | 高 | **B（强制停下）** |
| v1.21.0 | 中 | C（直推） |
| v1.22.0 | 中-高 | C（直推，除非 manual editor 触发升级中断） |
| v1.23.0 | 中-高 | **B（强制停下）** |
| v1.23.1 | 低 | 收尾直推 |

**B 节奏停下次数**：最少 4 次（v1.16.0 / v1.17.0 / v1.20.0 / v1.23.0），加最终验收。

---

## 工时预估（修正）

- v1.16.0：0.5 ~ 1 天（首版冲突集中爆发）
- v1.17.0：1 ~ 2 天（124 files 大量改动 + core-api 重构 + GeneralSettings 模块化）
- v1.18.0：< 0.5 天
- v1.19.0：< 0.5 天
- v1.20.0：1 ~ 2 天（WebSocket 重构 + 大规模文件移动）
- v1.21.0：< 0.5 天
- v1.22.0：0.5 ~ 1 天（manual editor 决策 + Wails 通知大改处理）
- v1.23.0：0.5 ~ 1 天（插件 ESM 迁移决策）
- v1.23.1：< 0.5 天
- **总计修正**：4 ~ 8 天有效推进时间（设计文档原估 2.5 ~ 4 天偏低）

修正原因：上游 v1.17.0 和 v1.20.0 都是 120+ 文件的大改版本，比设计阶段假设的"中等重构"重得多。

---

## 下一步

执行计划 Task 4：合并 v1.16.0（B 节奏）。
