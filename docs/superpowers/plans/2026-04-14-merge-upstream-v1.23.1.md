# 合并上游 v1.23.1 到 Web fork · 实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将上游 `GUI-for-Cores/GUI.for.SingBox@v1.23.1` 的业务功能按版本递进合并进本 fork，最终产物功能等同于 v1.23.1，但仍以 Go HTTP + Vue Web 形态运行，保留本地全部业务自定义（鉴权、Core Proxy、Reality/Trojan 入站、自定义入站 JSON、下载代理等）。

**Architecture:** 在独立 worktree `../GUI.for.SingBox.web-merge/` 的 `feature/merge-upstream-v1.23.1` 分支上逐版本 `git merge` 上游 v1.16.0 → v1.23.1。每版固定流水线：扫 diff → 解冲突 + 写决策清单 → bridge 按需补齐 → 编译闸 → 冒烟闸 → 自定义功能回归闸 → commit + tag 作回滚锚点。全部过闸后 no-ff 合回 main。

**Tech Stack:** Git worktree, git merge (递进合并), Go 1.24.0 (chi, cors, gorilla/websocket), Vue 3 + Vite + TypeScript, Pinia, CodeMirror。

**Reference:** 设计文档 `docs/superpowers/specs/2026-04-14-merge-upstream-v1.23.1-design.md`（commit 890e18c）。

---

## 工作路径约定

- **原仓库**：`/home/zhuyb/Documents/1.code/GUI.for.SingBox.web/`（供日常迭代/热修，合并期间不碰）
- **合并 worktree**：`/home/zhuyb/Documents/1.code/GUI.for.SingBox.web-merge/`（所有合并工作在此）
- **合并分支**：`feature/merge-upstream-v1.23.1`
- **决策清单目录**：`docs/merge-upstream/` (worktree 内)
- **用户授权**：全流程自主推进，仅以下场景强制停下汇报（设计文档 §3 升级中断规则）：
  1. 冲突涉及鉴权中间件结构变更
  2. 上游重构本地新增功能所在核心文件（如 `main.go` 路由组织）
  3. 上游删除本地业务依赖的方法/字段
  4. 冲突分类无法归入表 ①–⑧
  5. 上游引入新协议/内核能力类型且与 Reality/Trojan/自定义入站代码路径交叉

---

## Phase 1：初始化 worktree 与 upstream remote

### Task 1: 添加 upstream remote 并 fetch

**Files:**
- Modify: git 配置（本地仓库 `.git/config`）

- [ ] **Step 1: 检查当前 remote 状态**

Run:
```bash
cd /home/zhuyb/Documents/1.code/GUI.for.SingBox.web
git remote -v
```

Expected: 只显示 `arrow` 和 `origin`，均指向 `https://github.com/arrow8209/GUI.for.SingBox.web.git`

- [ ] **Step 2: 添加 upstream remote**

Run:
```bash
git remote add upstream https://github.com/GUI-for-Cores/GUI.for.SingBox.git
git remote -v
```

Expected: 新增 `upstream  https://github.com/GUI-for-Cores/GUI.for.SingBox.git (fetch/push)`

- [ ] **Step 3: fetch upstream tags**

Run:
```bash
git fetch upstream --tags
```

Expected: 下载上游所有分支和 tag，无错误。可能因网络慢耗时较长。

- [ ] **Step 4: 验证关键 tag 存在**

Run:
```bash
git tag -l | grep -E "^v1\.(1[6-9]|2[0-3])" | sort
```

Expected: 至少显示 `v1.16.0, v1.17.0, v1.18.0, v1.19.0, v1.20.0, v1.21.0, v1.22.0, v1.23.0, v1.23.1`（实际可能包含小 patch 版本）

- [ ] **Step 5: 记录 tag 清单**

Run:
```bash
git tag -l | grep -E "^v1\.(1[5-9]|2[0-3])" | sort -V > /tmp/upstream-tags.txt
cat /tmp/upstream-tags.txt
```

保存清单供 Task 4 前置扫描使用。

---

### Task 2: 创建合并分支与 worktree

- [ ] **Step 1: 确认当前 main HEAD**

Run:
```bash
cd /home/zhuyb/Documents/1.code/GUI.for.SingBox.web
git rev-parse main
git log --oneline -1 main
```

Expected: HEAD 为 `a76614f fix: handle expired auth and clarify subscription tls errors`（或此之后的 commit，记录实际 SHA）

- [ ] **Step 2: 创建合并分支（基于 main）**

Run:
```bash
git branch feature/merge-upstream-v1.23.1 main
git branch --list feature/merge-upstream-v1.23.1
```

Expected: 显示 `feature/merge-upstream-v1.23.1`

- [ ] **Step 3: 创建 worktree**

Run:
```bash
git worktree add ../GUI.for.SingBox.web-merge feature/merge-upstream-v1.23.1
```

Expected: 新建 `../GUI.for.SingBox.web-merge/` 目录，输出 "Preparing worktree..." 成功信息。

- [ ] **Step 4: 验证 worktree 状态**

Run:
```bash
git worktree list
cd ../GUI.for.SingBox.web-merge
git status
git log --oneline -1
```

Expected: worktree list 显示两条记录；`git status` 为 clean；HEAD 与 main 一致。

- [ ] **Step 5: 在 worktree 中创建决策文档目录**

Run:
```bash
cd /home/zhuyb/Documents/1.code/GUI.for.SingBox.web-merge
mkdir -p docs/merge-upstream
ls docs/merge-upstream/
```

Expected: 目录存在且为空。

- [ ] **Step 6: 提交 worktree 初始化 marker commit**

Run:
```bash
cd /home/zhuyb/Documents/1.code/GUI.for.SingBox.web-merge
echo "# Merge Upstream v1.23.1 Workspace" > docs/merge-upstream/README.md
cat >> docs/merge-upstream/README.md <<'EOF'

本目录记录 v1.15.1 → v1.23.1 分版本合并的决策与验证。

每小版本产出：
- `v1.X.Y-decisions.md`：冲突决策清单
- `v1.X.Y-verification.md`：编译/冒烟/回归验证记录

另含：
- `pre-scan.md`：合并前置扫描报告（Task 4 产出）
EOF
git add docs/merge-upstream/README.md
git commit -m "chore(merge): initialize merge-upstream workspace"
```

Expected: commit 成功，一行 README 入 git。

---

## Phase 2：合并前置扫描

### Task 3: 生成《合并前置扫描报告》

**Files:**
- Create: `docs/merge-upstream/pre-scan.md`

- [ ] **Step 1: 统计各小版本 commit 数量**

Run:
```bash
cd /home/zhuyb/Documents/1.code/GUI.for.SingBox.web-merge
for tag in v1.16.0 v1.17.0 v1.18.0 v1.19.0 v1.20.0 v1.21.0 v1.22.0 v1.23.0 v1.23.1; do
  prev=$(git tag -l | grep -E "^v1\." | sort -V | grep -B1 "^${tag}$" | head -1)
  count=$(git log --oneline upstream/${prev}..upstream/${tag} 2>/dev/null | wc -l)
  echo "${prev} -> ${tag}: ${count} commits"
done
```

记录输出用于报告。

- [ ] **Step 2: 提取每小版本的 commit 列表与 stat**

Run:
```bash
for tag in v1.16.0 v1.17.0 v1.18.0 v1.19.0 v1.20.0 v1.21.0 v1.22.0 v1.23.0 v1.23.1; do
  prev=$(git tag -l | grep -E "^v1\." | sort -V | grep -B1 "^${tag}$" | head -1)
  echo "===== ${prev} -> ${tag} ====="
  git log --oneline upstream/${prev}..upstream/${tag} 2>/dev/null
  echo "--- file stat ---"
  git diff --stat upstream/${prev}..upstream/${tag} 2>/dev/null | tail -10
done > /tmp/upstream-version-diffs.txt
wc -l /tmp/upstream-version-diffs.txt
```

Expected: 生成完整版本 diff 概览文件。

- [ ] **Step 3: 识别各版本的 Wails 桌面专属改动（可忽略）**

Run:
```bash
for tag in v1.16.0 v1.17.0 v1.18.0 v1.19.0 v1.20.0 v1.21.0 v1.22.0 v1.23.0 v1.23.1; do
  prev=$(git tag -l | grep -E "^v1\." | sort -V | grep -B1 "^${tag}$" | head -1)
  echo "===== ${tag} Wails-only touch ====="
  git diff --name-only upstream/${prev}..upstream/${tag} 2>/dev/null | grep -E "(wailsjs|bridge/tray|bridge/bridge\.go|wails\.json|\.github/workflows)" | head -20
done > /tmp/upstream-wails-only.txt
```

- [ ] **Step 4: 识别各版本对"红区文件"的触碰**

Run:
```bash
RED_ZONE='frontend/src/views/LoginView.vue|frontend/src/views/ProfilesView/components/InboundsConfig.vue|frontend/src/views/SettingsView/components/GeneralSettings.vue|frontend/src/views/SplashView.vue|frontend/src/utils/(request|others|restorer|tray|websockets|generator|env)\.ts|frontend/src/bridge/|frontend/src/stores/(auth|appSettings|kernelApi|subscribes)\.ts|frontend/src/api/kernel\.ts|frontend/src/types/(profile|app)\.d\.ts|frontend/src/lang/locale/(en|zh)\.ts|frontend/src/constant/(kernel|profile)\.ts|frontend/src/enums/kernel\.ts|frontend/src/router/|frontend/src/App\.vue|frontend/index\.html|frontend/vite\.config\.ts|^main\.go$|^bridge/'

for tag in v1.16.0 v1.17.0 v1.18.0 v1.19.0 v1.20.0 v1.21.0 v1.22.0 v1.23.0 v1.23.1; do
  prev=$(git tag -l | grep -E "^v1\." | sort -V | grep -B1 "^${tag}$" | head -1)
  echo "===== ${tag} Red zone touch ====="
  git diff --name-only upstream/${prev}..upstream/${tag} 2>/dev/null | grep -E "${RED_ZONE}" | sort
done > /tmp/upstream-redzone-touch.txt
wc -l /tmp/upstream-redzone-touch.txt
```

- [ ] **Step 5: 识别各版本新增的 bridge 方法（前端是否会用）**

Run:
```bash
for tag in v1.16.0 v1.17.0 v1.18.0 v1.19.0 v1.20.0 v1.21.0 v1.22.0 v1.23.0 v1.23.1; do
  prev=$(git tag -l | grep -E "^v1\." | sort -V | grep -B1 "^${tag}$" | head -1)
  echo "===== ${tag} new bridge methods ====="
  git diff upstream/${prev}..upstream/${tag} -- 'bridge/*.go' 2>/dev/null | grep -E "^\+func \(.*\*App\)" | sort -u
done > /tmp/upstream-new-bridge-methods.txt
```

- [ ] **Step 6: 抓取各版本 release notes**

Run:
```bash
for tag in v1.16.0 v1.17.0 v1.18.0 v1.19.0 v1.20.0 v1.21.0 v1.22.0 v1.23.0 v1.23.1; do
  echo "===== ${tag} ====="
  git tag -l -n99 "${tag}" | head -30
done > /tmp/upstream-release-notes.txt
```

若 git tag annotation 为空，在 Step 7 用 WebFetch 补：`https://github.com/GUI-for-Cores/GUI.for.SingBox/releases/tag/v1.X.Y`

- [ ] **Step 7: 写入《合并前置扫描报告》**

把 /tmp/ 中的四份临时产物整合为 `docs/merge-upstream/pre-scan.md`，结构如下：

```markdown
# 合并前置扫描报告 · v1.15.1 → v1.23.1

生成日期：2026-04-14

## 总览
- 待合并版本：v1.16.0, v1.17.0, v1.18.0, v1.19.0, v1.20.0, v1.21.0, v1.22.0, v1.23.0, v1.23.1
- 实际发现的中间 patch 版本（如 v1.16.1 等）：[列出]
- 总 commit 数：[合计]

## 各版本详情

### v1.16.0（基线 v1.15.1）
- commit 数：N
- Wails-only 文件：M（将忽略）
- 红区文件触碰：K 个
- 新增 bridge 方法：J 个
- Release notes 要点：[摘要]
- **风险标签**：[高/中/低] + 原因
- **预期行动**：[概述 v1.16 合并需重点处理的红区冲突]

### v1.17.0
... (同上格式)

[... 每版一节 ...]

## 关键发现（Heads-up）

1. **上游与本地重复实现的功能**（避免双份逻辑）：
   - [若发现，列出]
2. **上游重构本地新增功能所在文件**（触发升级中断规则）：
   - [若发现，列出]
3. **上游新增协议/内核能力**（需用户决策）：
   - [若发现，列出]

## 节奏计划确认

根据扫描结果，各版本预期节奏（B=停下汇报 / C=直推）：
- v1.16.0: [B 或 C，附理由]
- v1.17.0: ...
...
```

Write file 命令：用 Write 工具写入完整内容。

- [ ] **Step 8: 提交前置扫描报告**

Run:
```bash
cd /home/zhuyb/Documents/1.code/GUI.for.SingBox.web-merge
git add docs/merge-upstream/pre-scan.md
git commit -m "docs(merge): add pre-scan report for v1.15.1..v1.23.1"
```

Expected: commit 成功。

---

## Phase 3：逐版本合并

### Per-Version Merge Procedure（通用流水线）

以下步骤对每小版本都要完整执行。Phase 3 下每个 Task 是一个版本的实例，把 `<TAG>`、`<PREV>` 替换为实际 tag。

**Files per version:**
- Create: `docs/merge-upstream/<TAG>-decisions.md`
- Create: `docs/merge-upstream/<TAG>-verification.md`

**通用模板步骤（每版的 Task 下复用）：**

**A. 扫 diff 确认规模**
```bash
cd /home/zhuyb/Documents/1.code/GUI.for.SingBox.web-merge
git log --oneline upstream/<PREV>..upstream/<TAG>
git diff --stat upstream/<PREV>..upstream/<TAG> | tail -10
```
记录 commit 数、变更文件数、+/- 行数。

**B. 启动合并**
```bash
git merge upstream/<TAG> --no-commit --no-ff -m "merge: upstream <TAG> into web fork"
```
Expected: 很可能报冲突。`git status` 查看 `Unmerged paths` 清单。

**C. 冲突分类与解决**

按设计文档 §3 分类表逐块处理。开一个临时文件 `/tmp/<TAG>-conflicts.txt` 记录每块：

```text
<path>:<hunk>
分类：①/②/③/④/⑤/⑥/⑦/⑧
决策：保本地 / 采上游 / 手动融合
备注：...
```

命中升级中断规则时立即停下汇报用户。

**D. 手动融合红区文件**

对 ④ 类文件用 `git mergetool` 或直接编辑 `<<<<<<<`/`=======`/`>>>>>>>` 标记段，逐行判断。

**E. bridge 按需落地**

```bash
git diff upstream/<PREV>..upstream/<TAG> -- 'bridge/*.go' | grep -E "^\+func \(.*\*App\)" | sort -u > /tmp/<TAG>-new-bridge-methods.txt
```

逐个方法查 `frontend/src/views/` 和 `frontend/src/stores/` 是否调用；调用则在 `main.go` + `pkg/eventbus` + `frontend/src/bridge/*.ts` 补齐 HTTP/WS 实现。未调用则跳过（记入决策清单）。

**F. 清理忽略文件**

```bash
git reset HEAD frontend/dist/ guiforcores 2>/dev/null
git checkout -- frontend/dist/ guiforcores 2>/dev/null
git status | grep -E "(frontend/dist|guiforcores)" && echo "WARNING: build artifacts in diff"
```

若发现构建产物进入 diff，统一移除。

**G. 编译闸**

```bash
go build ./...
cd frontend && pnpm install && pnpm build
cd ..
```
Expected: 全绿。若失败，返回 C/D/E 排查。

**H. 冒烟闸**

```bash
# 启动服务
./gui-singbox &
SERVER_PID=$!
sleep 3
curl -sf http://127.0.0.1:22345/api/login -X POST -H 'Content-Type: application/json' -d '{"username":"admin","password":"admin123"}'
# 获取 token 后打每个 API + 手动打开浏览器点各页
```
手动在浏览器 `http://127.0.0.1:22345/` 过一遍：首页、订阅、配置、规则集、插件、日志、设置。

停服：`kill $SERVER_PID`。

**I. 自定义功能回归闸（12 项，见设计 §5）**

逐项验证，记入 `<TAG>-verification.md`。

**J. 写决策清单**

`docs/merge-upstream/<TAG>-decisions.md`，结构见设计 §3。

**K. 写验证记录**

`docs/merge-upstream/<TAG>-verification.md`：

```markdown
# <TAG> 验证记录

## 编译闸
- go build: [PASS/FAIL]
- pnpm build: [PASS/FAIL]
- pnpm lint: [warn 数/error 数]

## 冒烟闸
- [ ] 服务启动 :22345
- [ ] WebSocket /ws 可连
- [ ] 首页无错
- [ ] 订阅页
- [ ] 配置页
- [ ] 规则集页
- [ ] 插件页
- [ ] 日志页
- [ ] 设置页
- [ ] 内核启停

## 自定义功能回归（12 项）
1. 单管理员鉴权：[PASS/FAIL] <备注>
2. WebSocket 鉴权：[PASS/FAIL]
3. Core Proxy：[PASS/FAIL]
4. Reality 公钥生成：[PASS/FAIL]
5. VLESS Reality 入站：[PASS/FAIL]
6. Trojan TLS 入站：[PASS/FAIL]
7. 自定义入站 JSON 编辑器：[PASS/FAIL]
8. Shadowsocks 密码生成：[PASS/FAIL]
9. 下载代理：[PASS/FAIL]
10. 订阅 TLS 错误提示：[PASS/FAIL]
11. 构建 git hash 显示：[PASS/FAIL]
12. Exit API：[PASS/FAIL]

## TODO（非阻塞瑕疵）
- ...

## 结论
[放行 / 不放行 + 原因]
```

**L. 提交与打 tag**

```bash
git add -A docs/merge-upstream/<TAG>-*.md
# 注意: git add 已在 B 步合并时把业务文件暂存过；这里只补文档
git status   # 确认无遗漏
git commit -m "merge: upstream <TAG> into web fork"
git tag merge-<TAG>-done
```

**M. 按节奏汇报或推进**

根据前置扫描的节奏表（见 pre-scan.md）：
- 若该版标记 B：停下汇报用户，附 decisions.md + verification.md 摘要，等明确指令再推进下一版。
- 若该版标记 C：直接推进下一 Task。

---

### Task 4: 合并 v1.16.0（首版，B 节奏强制停下）

**版本替换**：`<TAG>` = `v1.16.0`，`<PREV>` = `v1.15.1`

**预期**：冲突密度最高（Wails bridge 演进 + 前端普通更新交织）。设计文档标记为"高风险"。

- [ ] **Step 1: 执行 Per-Version Procedure 步骤 A** —— 扫 diff（见上方通用模板）
- [ ] **Step 2: 执行步骤 B** —— `git merge upstream/v1.16.0 --no-commit --no-ff`
- [ ] **Step 3: 执行步骤 C** —— 冲突分类（生成 `/tmp/v1.16.0-conflicts.txt`）

触发升级中断规则时立即停下。特别注意：
- 若 `main.go` 的路由组织被上游改写（规则 2）→ 停
- 若 `authMiddleware` 结构变更（规则 1）→ 停
- 若上游引入 Hysteria2/AnyTLS/WireGuard 入站且涉及 Reality/Trojan 代码路径（规则 5）→ 停

- [ ] **Step 4: 执行步骤 D** —— 手动融合红区文件
- [ ] **Step 5: 执行步骤 E** —— bridge 按需落地
- [ ] **Step 6: 执行步骤 F** —— 清理构建产物
- [ ] **Step 7: 执行步骤 G** —— 编译闸（go build + pnpm build + pnpm lint）
- [ ] **Step 8: 执行步骤 H** —— 冒烟闸（浏览器手动过页）
- [ ] **Step 9: 执行步骤 I** —— 自定义功能回归闸（12 项）
- [ ] **Step 10: 执行步骤 J** —— 写 `v1.16.0-decisions.md`
- [ ] **Step 11: 执行步骤 K** —— 写 `v1.16.0-verification.md`
- [ ] **Step 12: 执行步骤 L** —— commit + tag `merge-v1.16.0-done`

- [ ] **Step 13: 强制汇报用户**

格式：

```
【v1.16.0 合并完成】
- 冲突文件数：N
- 决策分布：保本地 X / 采上游 Y / 手动融合 Z
- 新增 bridge 方法处理：J 个中 K 个落地，N-K 个跳过
- 编译闸：PASS
- 冒烟闸：PASS / FAIL <哪页>
- 回归闸 12 项：M/12 通过；失败项：...
- 结论：放行 / 不放行
- 决策清单：docs/merge-upstream/v1.16.0-decisions.md
- 验证记录：docs/merge-upstream/v1.16.0-verification.md
- 建议后续节奏：B / C（附理由）
```

等用户指令再进 Task 5。

---

### Task 5: 合并 v1.17.0

**版本替换**：`<TAG>` = `v1.17.0`，`<PREV>` = `v1.16.0`

**节奏**：依赖 v1.16.0 结果。用户指令为 B 则完成后停；为 C 则直推 Task 6。

- [ ] **Step 1: 执行 Per-Version Procedure 步骤 A**
- [ ] **Step 2: 执行步骤 B**
- [ ] **Step 3: 执行步骤 C**（注意升级中断规则）
- [ ] **Step 4: 执行步骤 D**
- [ ] **Step 5: 执行步骤 E**
- [ ] **Step 6: 执行步骤 F**
- [ ] **Step 7: 执行步骤 G**
- [ ] **Step 8: 执行步骤 H**
- [ ] **Step 9: 执行步骤 I**
- [ ] **Step 10: 执行步骤 J** —— 写 `v1.17.0-decisions.md`
- [ ] **Step 11: 执行步骤 K** —— 写 `v1.17.0-verification.md`
- [ ] **Step 12: 执行步骤 L** —— commit + tag `merge-v1.17.0-done`
- [ ] **Step 13: 按节奏汇报或推进**（B 则停；C 则进 Task 6）

---

### Task 6: 合并 v1.18.0

**版本替换**：`<TAG>` = `v1.18.0`，`<PREV>` = `v1.17.0`

**节奏**：默认 C（直推下一版），除非升级中断规则触发。

- [ ] **Step 1: 执行步骤 A**
- [ ] **Step 2: 执行步骤 B**
- [ ] **Step 3: 执行步骤 C**
- [ ] **Step 4: 执行步骤 D**
- [ ] **Step 5: 执行步骤 E**
- [ ] **Step 6: 执行步骤 F**
- [ ] **Step 7: 执行步骤 G**
- [ ] **Step 8: 执行步骤 H**
- [ ] **Step 9: 执行步骤 I**
- [ ] **Step 10: 写 `v1.18.0-decisions.md`**
- [ ] **Step 11: 写 `v1.18.0-verification.md`**
- [ ] **Step 12: commit + tag `merge-v1.18.0-done`**
- [ ] **Step 13: 直推 Task 7**（C 节奏）

---

### Task 7: 合并 v1.19.0

**版本替换**：`<TAG>` = `v1.19.0`，`<PREV>` = `v1.18.0`

**节奏**：C。

- [ ] **Step 1: 执行步骤 A**
- [ ] **Step 2: 执行步骤 B**
- [ ] **Step 3: 执行步骤 C**
- [ ] **Step 4: 执行步骤 D**
- [ ] **Step 5: 执行步骤 E**
- [ ] **Step 6: 执行步骤 F**
- [ ] **Step 7: 执行步骤 G**
- [ ] **Step 8: 执行步骤 H**
- [ ] **Step 9: 执行步骤 I**
- [ ] **Step 10: 写 `v1.19.0-decisions.md`**
- [ ] **Step 11: 写 `v1.19.0-verification.md`**
- [ ] **Step 12: commit + tag `merge-v1.19.0-done`**
- [ ] **Step 13: 直推 Task 8**

---

### Task 8: 合并 v1.20.0（关键节点，B 节奏强制停下）

**版本替换**：`<TAG>` = `v1.20.0`，`<PREV>` = `v1.19.0`

**节奏**：B（强制停下）。这是设计文档标记的阶段性检查点，用于跨大段版本后做一次全量回归。

- [ ] **Step 1: 执行步骤 A**
- [ ] **Step 2: 执行步骤 B**
- [ ] **Step 3: 执行步骤 C**
- [ ] **Step 4: 执行步骤 D**
- [ ] **Step 5: 执行步骤 E**
- [ ] **Step 6: 执行步骤 F**
- [ ] **Step 7: 执行步骤 G**
- [ ] **Step 8: 执行步骤 H**
- [ ] **Step 9: 执行步骤 I（12 项全量回归，本版要格外严格）**
- [ ] **Step 10: 写 `v1.20.0-decisions.md`**
- [ ] **Step 11: 写 `v1.20.0-verification.md`**
- [ ] **Step 12: commit + tag `merge-v1.20.0-done`**
- [ ] **Step 13: 强制汇报用户（同 Task 4 Step 13 格式），等指令再进 Task 9**

---

### Task 9: 合并 v1.21.0

**版本替换**：`<TAG>` = `v1.21.0`，`<PREV>` = `v1.20.0`

**节奏**：C（默认）。

- [ ] **Step 1: 执行步骤 A**
- [ ] **Step 2: 执行步骤 B**
- [ ] **Step 3: 执行步骤 C**
- [ ] **Step 4: 执行步骤 D**
- [ ] **Step 5: 执行步骤 E**
- [ ] **Step 6: 执行步骤 F**
- [ ] **Step 7: 执行步骤 G**
- [ ] **Step 8: 执行步骤 H**
- [ ] **Step 9: 执行步骤 I**
- [ ] **Step 10: 写 `v1.21.0-decisions.md`**
- [ ] **Step 11: 写 `v1.21.0-verification.md`**
- [ ] **Step 12: commit + tag `merge-v1.21.0-done`**
- [ ] **Step 13: 直推 Task 10**

---

### Task 10: 合并 v1.22.0

**版本替换**：`<TAG>` = `v1.22.0`，`<PREV>` = `v1.21.0`

**节奏**：C（默认）。

- [ ] **Step 1: 执行 Per-Version Procedure 步骤 A** —— 扫 diff
- [ ] **Step 2: 执行步骤 B** —— `git merge upstream/v1.22.0 --no-commit --no-ff`
- [ ] **Step 3: 执行步骤 C** —— 冲突分类（升级中断规则）
- [ ] **Step 4: 执行步骤 D** —— 手动融合红区文件
- [ ] **Step 5: 执行步骤 E** —— bridge 按需落地
- [ ] **Step 6: 执行步骤 F** —— 清理构建产物
- [ ] **Step 7: 执行步骤 G** —— 编译闸
- [ ] **Step 8: 执行步骤 H** —— 冒烟闸
- [ ] **Step 9: 执行步骤 I** —— 回归闸 12 项
- [ ] **Step 10: 写 `v1.22.0-decisions.md`**
- [ ] **Step 11: 写 `v1.22.0-verification.md`**
- [ ] **Step 12: commit + tag `merge-v1.22.0-done`**
- [ ] **Step 13: 直推 Task 11**（C 节奏）

---

### Task 11: 合并 v1.23.0（B 节奏强制停下）

**版本替换**：`<TAG>` = `v1.23.0`，`<PREV>` = `v1.22.0`

**节奏**：B（强制停下）。v1.23 是大版本，预期较多新功能。

- [ ] **Step 1: 执行步骤 A**
- [ ] **Step 2: 执行步骤 B**
- [ ] **Step 3: 执行步骤 C**
- [ ] **Step 4: 执行步骤 D**
- [ ] **Step 5: 执行步骤 E**
- [ ] **Step 6: 执行步骤 F**
- [ ] **Step 7: 执行步骤 G**
- [ ] **Step 8: 执行步骤 H**
- [ ] **Step 9: 执行步骤 I**
- [ ] **Step 10: 写 `v1.23.0-decisions.md`**
- [ ] **Step 11: 写 `v1.23.0-verification.md`**
- [ ] **Step 12: commit + tag `merge-v1.23.0-done`**
- [ ] **Step 13: 强制汇报用户，等指令再进 Task 12**

---

### Task 12: 合并 v1.23.1（收尾版）

**版本替换**：`<TAG>` = `v1.23.1`，`<PREV>` = `v1.23.0`

**节奏**：直推至 Phase 4（最终验收）。通常 patch 版本冲突极少。

- [ ] **Step 1: 执行步骤 A**
- [ ] **Step 2: 执行步骤 B**
- [ ] **Step 3: 执行步骤 C**
- [ ] **Step 4: 执行步骤 D**
- [ ] **Step 5: 执行步骤 E**
- [ ] **Step 6: 执行步骤 F**
- [ ] **Step 7: 执行步骤 G**
- [ ] **Step 8: 执行步骤 H**
- [ ] **Step 9: 执行步骤 I**
- [ ] **Step 10: 写 `v1.23.1-decisions.md`**
- [ ] **Step 11: 写 `v1.23.1-verification.md`**
- [ ] **Step 12: commit + tag `merge-v1.23.1-done`**
- [ ] **Step 13: 直推 Phase 4**

---

## Phase 4：最终验收与合回 main

### Task 13: 最终全量验证

**Files:**
- Create: `docs/merge-upstream/final-verification.md`

- [ ] **Step 1: 确认所有版本 tag 存在**

Run:
```bash
cd /home/zhuyb/Documents/1.code/GUI.for.SingBox.web-merge
git tag -l | grep '^merge-v' | sort -V
```

Expected: 显示 v1.16.0 ~ v1.23.1 的全部 9 个 `merge-<TAG>-done` tag。

- [ ] **Step 2: 确认所有决策与验证文档齐全**

Run:
```bash
ls -la docs/merge-upstream/
```

Expected: `README.md`, `pre-scan.md`, 加 9 对 `v1.X.Y-decisions.md` + `v1.X.Y-verification.md`，共 20 个文件。

- [ ] **Step 3: 最终编译闸**

```bash
go build ./...
cd frontend && rm -rf node_modules && pnpm install && pnpm build
cd .. && pnpm lint 2>&1 | tail -5
```

Expected: 全绿。

- [ ] **Step 4: 最终冒烟闸**

同 Per-Version 步骤 H，但更严格——每页都要点过一次。

- [ ] **Step 5: 最终自定义功能回归闸（12 项全量）**

同 Per-Version 步骤 I，但全部 12 项都需 PASS。若任一项 FAIL，定位到是哪次合并引入（`git bisect` 在 `merge-v1.X.Y-done` tag 之间），回到那版修复后从该点重新推进。

- [ ] **Step 6: 写最终验证记录**

`docs/merge-upstream/final-verification.md`：

```markdown
# v1.23.1 最终验证记录

## 编译闸
- go build: PASS
- pnpm build: PASS
- pnpm lint: [warn N]

## 冒烟闸（完整导航）
- [x] 所有 8 个主页

## 自定义功能回归（12 项）
1. 单管理员鉴权：PASS
2. WebSocket 鉴权：PASS
...

## 合并链概览
| 版本 | 冲突文件数 | 新增 bridge | 回归闸 | Tag |
|------|-----------|-------------|--------|-----|
| v1.16.0 | ... | ... | 12/12 | merge-v1.16.0-done |
...

## 结论
- [ ] 可合回 main
```

- [ ] **Step 7: 提交最终验证记录**

```bash
git add docs/merge-upstream/final-verification.md
git commit -m "docs(merge): final verification for v1.23.1"
git tag merge-upstream-v1.23.1-complete
```

---

### Task 14: 合回 main

- [ ] **Step 1: 回原仓库**

```bash
cd /home/zhuyb/Documents/1.code/GUI.for.SingBox.web
git status
```

Expected: clean。

- [ ] **Step 2: 先 pull main（以防有紧急修复）**

```bash
git fetch origin
git log --oneline main..origin/main
```

若有新 commit：`git merge origin/main`。若 main 在合并期间有修复，需要先把它拉进合并分支：

```bash
cd ../GUI.for.SingBox.web-merge
git merge main  # 解冲突后 commit
go build ./... && cd frontend && pnpm build && cd ..   # 再过一次编译闸
cd ../GUI.for.SingBox.web
```

- [ ] **Step 3: no-ff 合回 main**

```bash
git merge feature/merge-upstream-v1.23.1 --no-ff -m "merge: upstream v1.23.1 into web fork

详见 docs/merge-upstream/final-verification.md 与各版决策清单。"
```

Expected: 因有 tag 锚点链，冲突极少。若仍有冲突：停下汇报。

- [ ] **Step 4: 再跑一次编译闸**

```bash
go build ./...
cd frontend && pnpm build && cd ..
```

Expected: 全绿。

- [ ] **Step 5: 推送（**暂不执行**，等用户确认）**

不自动 push。产出以下信息给用户：

```
【合并完成，待 push】
- 合并分支 feature/merge-upstream-v1.23.1 已 no-ff 合回 main
- main 新 commit：<SHA>
- 待 push 命令：git push origin main
- 请确认后手动 push，或授权后我执行。
```

等用户指令。

- [ ] **Step 6: 清理 worktree（用户确认 push 后执行）**

```bash
cd /home/zhuyb/Documents/1.code/GUI.for.SingBox.web
git worktree remove ../GUI.for.SingBox.web-merge
git branch -d feature/merge-upstream-v1.23.1   # 保留 merge-v1.X.Y-done tag 作历史锚点
git worktree list
```

Expected: worktree 清理，仅剩主仓库；分支删除；tag 保留。

- [ ] **Step 7: 清理 /tmp 中的临时分析文件**

```bash
rm -f /tmp/upstream-tags.txt /tmp/upstream-version-diffs.txt /tmp/upstream-wails-only.txt /tmp/upstream-redzone-touch.txt /tmp/upstream-new-bridge-methods.txt /tmp/upstream-release-notes.txt /tmp/v1.*-conflicts.txt /tmp/v1.*-new-bridge-methods.txt
```

---

## 失败退路

- **单版失败**：`git reset --hard merge-v1.(X-1).Y-done` → 回到上一稳定版，重新规划该版策略。
- **中途放弃**：
  ```bash
  cd /home/zhuyb/Documents/1.code/GUI.for.SingBox.web
  git worktree remove -f ../GUI.for.SingBox.web-merge
  git branch -D feature/merge-upstream-v1.23.1
  # tag 会保留，随时可从任一 merge-v1.X.Y-done 重开
  ```
  main 毫发无损。
- **从某版重开**：`git checkout -b retry merge-v1.(X-1).Y-done` 从该点切新分支。

---

## 汇报规范

每次按 B 节奏汇报时使用统一格式（见 Task 4 Step 13）。每次汇报后等用户明确指令（"继续 / 停 / 回退 / 改策略"）再推进。
