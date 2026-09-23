---
name: newapi-fork-sync
description: 同步 New API fork 三线分支：官方更新进 main，rebase custom 自定义线，再从两者刷新 release 合并线。用户要求更新上游或同步 fork 时使用；不用于部署和功能开发。
---

# New API Fork 同步（三线制）

仓库有三条分支线，这是用户的明确设计意图，不得增删或合并角色：

- **`main`**：官方原版镜像。只通过 fast-forward 跟随 `upstream/main`，绝不提交任何内容。
- **`custom`**：**纯自定义线**。承载全部用户自定义提交（不含官方内容之外的东西混在历史里），每次同步 rebase 到最新 `main` 之上。
- **`release`**：**合并线 = 部署线**。内容始终等于"官方最新 + 全部自定义"（即 `main` + `custom` 的线性叠加），构建和部署只使用它。它没有任何自己的提交，每次同步从 `custom` 快进或重置生成。

禁止创建其它 `custom/*`、`release/*` 或长期特性分支；新的自定义工作一律直接提交到 `custom`。

## 基本事实

- 工作目录 `/Users/jairo/Desktop/newapi-custom`；`upstream` = `QuantumNous/new-api`（只读），`origin` = `supjairo/new-api`。
- 开始前运行 `git status --short --branch`、`git remote -v`、`git branch -vv` 并 `git fetch upstream`、`git fetch origin`。存在未提交改动时先停下询问，绝不覆盖。
- 自定义提交清单 = `git log --oneline main..custom`；自定义改动范围 = `git diff main...custom --stat`。
- 健康检查：`release` 必须与 `custom` 内容一致（`git rev-parse custom^{tree} release^{tree}` 相同，或 `git diff custom release` 为空）。不一致说明上次同步中断，先修复。

## 同步流程（官方有更新时）

1. 官方线：`git switch main && git merge --ff-only upstream/main && git push origin main`。无法 fast-forward 时停下报告，绝不 reset/force-push `main`。
2. 自定义线：`git switch custom && git rebase main`。冲突仅在解法明确时解决（保持官方改动、叠加自定义意图）；否则 `git rebase --abort` 并报告，绝不猜测。
3. 合并线：`git switch release && git rebase custom`（等价于把 release 快进到新 custom）。若因历史分叉无法 rebase，用 `git reset --hard custom` 重建——release 没有自己的提交，这是安全的。
4. 验证（全部通过才算完成，在 release 上执行）：
   - `go build ./...`（`go` 不在 PATH 时找 `~/sdk/go*/bin/go`）
   - `go test ./model/ ./service/` 及受影响包的针对性测试
   - 涉及 `relaykit/` 时：`cd relaykit && GOWORK=off go build ./...`
   - 涉及 `web/` 时：在 `web/` 先 `bun install`（如缺 node_modules）再 `bun run build`，相关组件用 `bunx vitest run <测试文件>` 验证
5. 推送：`git push --force-with-lease origin custom` 和 `git push --force-with-lease origin release`（rebase 改写历史必须带 lease，绝不裸 force）。用户未要求推送时不推。
6. 报告前核对：`git log --oneline main..custom` 自定义提交一个不少；`git diff custom release` 为空；`git diff main...release --stat` 与自定义范围一致。

## 日常自定义开发

1. `git switch custom`，直接修改、提交，`git push origin custom`。
2. 需要部署/构建时（或让 release 保持最新）：`git switch release && git rebase custom && git push --force-with-lease origin release`。
3. 新功能不加新分支；官方更新后按上面同步流程刷新三线。

## 各场景执行规则（必须按场景对号入座）

用户说"同步/更新/拉官方"时，按"同步流程"整段执行三线刷新，结束后按报告模板输出中文报告。

用户在 custom 上开发后要求部署或刷新 release 时，只执行：`git switch release && git rebase custom` → 验证 → `git push --force-with-lease origin release`，不动 main。

任何时候发现分支结构偏离三线定义（多了分支、release 落后于 custom、main 混入提交、release 被手工提交过），先修复结构再继续任务，并向用户说明修复了什么。

- 用户要求改代码/加功能：切到 `custom` 修改提交；若手头在 release 上有未提交改动，提醒用户改动应落在 custom。
- 修复结构与清理分支必须先征得用户确认，除非偏离是本技能同步流程自身造成的中间态。
- 用户未说"推送"时，本地完成验证即停，报告里注明未推送。

## 红线

- 绝不向 `main` 提交；绝不 reset --hard / force-push `main`。
- 绝不向 `release` 手工提交任何代码；它只能从 `custom` 快进/重置生成。
- 绝不覆盖未提交改动；绝不未经用户明确授权删除或改写分支。
- 不泄露密钥、token、环境值。
- 不要凭提交数、分支名或"分支旧"判断功能已覆盖或可丢弃；以实际代码行为为准。

## 中文完成报告（每次必须）

- **官方原版**：旧/新 `upstream/main` 哈希与摘要、`main` 是否与 `upstream/main` 一致、推送结果、偏差说明。
- **自定义线 `custom`**：rebase 目标基线、自定义提交数与清单、冲突及解决方式、推送结果。
- **合并线 `release`**：与 `custom` 的内容一致性核对结果、推送结果、验证（构建/测试）结果。
- **遗留**：未验证项、待用户决定事项；没有则不写。
