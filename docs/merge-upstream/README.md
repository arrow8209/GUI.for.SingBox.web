# Merge Upstream v1.23.1 Workspace

本目录记录 v1.15.1 → v1.23.1 分版本合并的决策与验证。

每小版本产出：
- `v1.X.Y-decisions.md`：冲突决策清单
- `v1.X.Y-verification.md`：编译/冒烟/回归验证记录

另含：
- `pre-scan.md`：合并前置扫描报告（执行 Task 3 产出）
- `final-verification.md`：最终全量验证记录（执行 Task 13 产出）

## 重要背景

**上游 v1.16.0 ~ v1.20.0 没有正式 git tag**，只有 Release commits。本次合并以 commit SHA 代替 tag 作为里程碑，详见 `pre-scan.md`。
