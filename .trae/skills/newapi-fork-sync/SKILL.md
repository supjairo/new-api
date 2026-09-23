---
name: newapi-fork-sync
description: 同步 New API fork：官方更新进 main，rebase custom 合并线到最新官方之上。用户要求更新上游或同步 fork 时使用；不用于部署和功能开发。
---

# New API Fork 同步（双线制）

仓库只有两条分支线，这是用户的明确设计意图，不得偏离：

- **`main`**：官方原版镜像，只通过 fast-forward 跟随 `upstream/main`，绝不提交任何内容。
- **`custom`**：**唯一的**自定义分支，承载全部自定义提交，rebase 在最新 `main` 之上——**它本身就是"官方最新 + 全部自定义"的合并线**，构建和部署都使用它。

禁止创建任何其它 `custom/*` 或长期特性分支；新的自定义工作一律直接提交到 `custom`。

## 基本事实

- 工作目录 `/Users/jairo/Desktop/newapi-custom`；`upstream` = `QuantumNous/new-api`（只读），`origin` = `supjairo/new-api`。
- 开始前运行 `git status --short --branch`、`git remote -v`、`git branch -vv` 并 `git fetch upstream`、`git fetch origin`。存在未提交改动时先停下询问，绝不覆盖。
- 自定义提交清单 = `git log --oneline main..custom`。自定义改动范围 = `git diff main...custom --stat`。

## 同步流程（官方有更新时）

1. 官方轨道：`git switch main && git merge --ff-only upstream/main`，然后 `git push origin main`。无法 fast-forward 时停下报告，绝不 reset/force-push `main`。
2. 合并线：`git switch custom && git rebase main`。冲突仅在解法明确时解决（保持官方改动、叠加自定义意图）；否则 `git rebase --abort` 并报告，绝不猜测。
3. 验证（全部通过才算完成）：
   - `go build ./...`（`go` 不在 PATH 时找 `~/sdk/go*/bin/go`）
   - `go test ./model/ ./service/` 及受影响包的针对性测试
   - 涉及 `relaykit/` 时：`cd relaykit && GOWORK=off go build ./...`
   - 涉及 `web/` 时：在 `web/` 先 `bun install`（如缺 node_modules）再 `bun run build`，相关组件用 `bunx vitest run <测试文件>` 验证
4. 推送合并线：`git push --force-with-lease origin custom`（rebase 改写历史必须带 lease，绝不裸 force）。用户未要求推送时不推。
5. 报告前核对 `git log --oneline main..custom` 与 `git diff main...custom --stat`：自定义提交一个不少、无多余内容。

## 日常自定义开发

1. `git switch custom`，直接修改、提交。
2. 推送：`git push origin custom`（首次 `git push -u origin custom`）。
3. 新功能不加新分支；若官方随后更新，按上面同步流程 rebase。

## 红线

- 绝不向 `main` 提交；绝不 reset --hard / force-push `main`。
- 绝不覆盖未提交改动；绝不未经用户明确授权删除或改写分支。
- 不泄露密钥、token、环境值。
- 不要凭提交数、分支名或"分支旧"判断功能已覆盖或可丢弃；以实际代码行为为准。

## 中文完成报告（每次必须）

- **官方原版**：旧/新 `upstream/main` 哈希与摘要、`main` 与 `upstream/main` 是否一致、推送结果、偏差说明。
- **合并线 `custom`**：rebase 目标基线、自定义提交数与清单、冲突及解决方式、推送结果（force 模式与新哈希）、构建与测试结果。
- **遗留**：未验证项、待用户决定事项；没有则不写。
