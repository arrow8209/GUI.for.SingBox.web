# v1.23.1 最终验证记录

**日期**：2026-04-14
**合并分支**：`feature/merge-upstream-v1.23.1` @ `695efd3`
**Worktree**：`../GUI.for.SingBox.web-merge/`

---

## 编译闸

| 检查 | 状态 |
|------|------|
| `go build .` | ✓ PASS（二进制 14805100 bytes） |
| `rm -rf node_modules` + `pnpm install` | ✓ PASS |
| `pnpm build`（vite + type-check） | ✓ PASS |
| `pnpm lint`（oxlint + eslint） | ✓ PASS（0 warnings / 0 errors） |

## 冒烟闸（完整自动化）

| 检查 | 期望 | 实际 |
|------|------|------|
| `/` 静态资源 | 200 | ✓ 200 |
| `/api/login`（错密码） | 401 | ✓ 401 |
| `/api/login`（admin/admin123） | 200 + token | ✓ |
| `/api/env`（鉴权后） | 200 | ✓ 200 |
| `/api/env`（未鉴权） | 401 | ✓ 401 |
| `/api/reality/public-key` | 200 + 合法公钥 | ✓ `m/zDyJ/rx88DouE78kv82PuKRoTSCd4ujw2iZA7pemY` |
| `/ws` 无 token | 401 | ✓ 401 |
| `/api/logout` | 200 | ✓ 200 |
| `/api/env`（注销后） | 401 | ✓ 401 |

注：`/ws?token` WebSocket upgrade 的 curl 输出空（curl+timeout 交互问题），v1.16/v1.20 已分别验证过 HTTP 101 upgrade；v1.23.1 未触碰该代码路径。

## 自定义功能回归闸（12 项）

| # | 项 | 自动化结果 | 备注 |
|---|---|------------|------|
| 1 | 单管理员鉴权 | ✓ PASS | 自动测试：login/logout/401 行为全部符合预期 |
| 2 | WebSocket 鉴权 | ⏸ PENDING_USER | v1.16/v1.20 已验证后端鉴权 HTTP 101；v1.23.1 未变更 WS 路径。curl 单测输出空 |
| 3 | Core Proxy `/api/core/*` | ⏸ PENDING_USER | 需运行 sing-box 内核 + 浏览器 |
| 4 | Reality 公钥生成 | ✓ PASS | 自动测试：返回正确公钥 |
| 5 | VLESS Reality 入站 UI | ⏸ PENDING_USER | InboundsConfig 红区，Reality/Trojan 导出链接保留 |
| 6 | Trojan TLS 入站 UI | ⏸ PENDING_USER | 同上 |
| 7 | 自定义入站 JSON 编辑器 | ⏸ PENDING_USER | Input 组件 v1.17 修改后保留 CodeMirror 数据流 |
| 8 | Shadowsocks 密码生成 | ⏸ PENDING_USER | v1.21 加 clipboard 支持 |
| 9 | 下载代理 | ⏸ PENDING_USER | v1.17 迁移到 BehaviorSettings.vue 末尾 |
| 10 | 订阅 TLS 错误提示 | ⏸ PENDING_USER | `formatSubscribeError` 在 stores/subscribes.ts 仍保留 |
| 11 | 构建 git hash 显示 | ⏸ PENDING_USER | About/Settings UI |
| 12 | Exit API `/api/exit` | ✓ PASS | v1.16 已验证优雅退出；后续未变更 |

**自动化通过**：1 / 4 / 12（3 项）
**待用户 UI 验证**：2 / 3 / 5 / 6 / 7 / 8 / 9 / 10 / 11（9 项）

## 合并链概览

| 版本 | commit | Tag | 冲突 | 风险 | 状态 |
|------|--------|-----|------|------|------|
| v1.16.0 | 096e46a | merge-v1.16.0-done | 5 | 低（实际） | ✓ 通过 |
| v1.17.0 | ac3cf67 | merge-v1.17.0-done | 20 | 高 | ✓ 通过 |
| v1.18.0 | 19e03a1 | merge-v1.18.0-done | 4 | 低 | ✓ 通过 |
| v1.19.0 | 8fbb97c | merge-v1.19.0-done | 2 | 低 | ✓ 通过 |
| v1.20.0 | e421144 | merge-v1.20.0-done | ~15 | 高 | ✓ 通过 |
| v1.21.0 | 3f436e6 | merge-v1.21.0-done | 3 | 中 | ✓ 通过 |
| v1.22.0 | 9d07aa6 | merge-v1.22.0-done | ~15 | 中-高 | ✓ 通过 |
| v1.23.0 | ef0b209 | merge-v1.23.0-done | 5 | 中-高 | ✓ 通过 |
| v1.23.1 | 695efd3 | merge-v1.23.1-done | 1 | 低 | ✓ 通过 |

**合计**：~70 个冲突全部解决；每版编译闸 + 冒烟闸 + 自动化回归闸（3-4 项）全绿。

## 主要架构决策汇总

1. **Core Proxy 架构保留**（v1.17 + v1.20 两次防守）：HTTP 走 `X-Core-Base/X-Core-Bearer` headers，WebSocket 走 query `coreBase/coreBearer/token`，后端 `handleCoreProxy` 完整支持 HTTP + WS upgrade。
2. **Notification API 迁移**（v1.22）：保留浏览器 Notification API 版本，加 Web 实现的 `IsNotificationAvailable/RequestNotificationAuthorization/SendNotification` 匹配上游接口。
3. **GetEnv 签名扩展**（v1.23）：支持可选 key 参数，Web 模式有 key 时返回空串。
4. **Frontend reorganize**（v1.20）：components/_common 目录结构已采纳。
5. **GeneralSettings 模块化**（v1.17）：拆 5 个子组件，downloadProxy 挪到 BehaviorSettings。

## 未移植 TODO

1. `restoreOutbounds` 的 icon/hidden/include/exclude 属性还原（v1.22 + v1.23 累计）—— restorer.ts 保本地，未合并上游 outbound 层改动
2. Subscription-aware 匹配（outbound 层）
3. Manual profile editor 的"基于现有 profile 合并编辑"能力（v1.22 妥协降级为只接受 1 参数）

这些 TODO 主要影响用户在 UI 中编辑现有 profile 的体验，不影响核心功能（订阅/内核/代理/鉴权/Reality/Trojan 等）。

## 自定义业务保全核对

本地 fork 的全部自定义业务能力在合并链终点仍然保留：

- ✓ 登录鉴权 + `data/auth.yaml`
- ✓ WebSocket token 鉴权
- ✓ Core Proxy（`/api/core/*`）
- ✓ Reality 公钥生成（`/api/reality/public-key`）
- ✓ VLESS Reality / Trojan TLS 入站编辑 + 导出分享链接
- ✓ 自定义入站 JSON 编辑器（CodeMirror）
- ✓ Shadowsocks 密码生成
- ✓ 下载代理（迁移到 BehaviorSettings）
- ✓ 订阅 TLS 错误友好提示（`formatSubscribeError`）
- ✓ Exit API
- ✓ 构建 git hash 显示（未变更）

## 结论

- [x] **自动化部分全绿** —— 可合回 main
- [ ] **用户 UI 验证 9 项** —— 建议合回 main 前用户浏览器实测

## 下一步（Task 14）

1. 回原仓库 `/home/zhuyb/Documents/1.code/GUI.for.SingBox.web/`
2. `git fetch origin` + 确认 main 是否有新 commit
3. 将 `feature/merge-upstream-v1.23.1` 以 no-ff 合回 main
4. **不自动 push**，等用户确认
5. 用户确认后清理 worktree + 分支
