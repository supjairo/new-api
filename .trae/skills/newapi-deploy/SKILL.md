---
name: newapi-deploy
description: 构建 lucas:last 自定义镜像并部署到宝塔服务器 lucas 编排，含验证替换与收尾清理。用户要求部署、发布、更新服务器时使用；不用于同步分支。
---

# New API 部署（lucas:last 三步走）

目标：把 `release` 合并线的代码构建为本地镜像 `lucas:last`，上传到用户的服务器，替换宝塔面板 Docker Compose 项目 **lucas** 中的旧镜像，验证通过后收尾清理。全程不得改动 `main`/`custom` 分支，不得动官方镜像 `calciumion/new-api`。

## 部署前置检查

1. `git status --short --branch`：工作树必须干净，当前在 `release` 分支且与 `origin/release` 一致（`git rev-parse release origin/release` 相同）。不满足时先停下询问。
2. 确认 `git diff custom release` 为空——部署内容必须是"官方最新 + 全部自定义"。
3. 部署会改线上服务：**开始前必须向用户确认**"即将部署 `<当前 release 短哈希>` 到服务器 lucas"，得到明确同意才继续。
4. 服务器连接信息由用户以消息提供（主机/IP、SSH 凭据形态等），不要在输出中回显密码、密钥，不要把凭据写入任何文件。

## 第一步：本地构建镜像（在本机执行）

1. 依赖构建工具：`docker`（本机可用 `docker version` 验证，缺失时报告并停止）。构建用仓库自带 [Dockerfile](Dockerfile)（多阶段：bun 构建前端 → go 构建后端 → debian 运行时），无需本地装 Go/Node。
2. 构建（在仓库根目录）：
   ```bash
   docker build --platform linux/amd64 -t lucas:last .
   ```
   服务器是 amd64；若用户说明服务器为 arm64，则改用 `--platform linux/arm64`。
3. 本地验证镜像：`docker run -d --rm --name lucas-verify -p 127.0.0.1:3100:3000 lucas:last`，等几秒后 `curl -fs http://127.0.0.1:3100/api/status` 应返回 `"success":true`，然后 `docker stop lucas-verify`。验证失败则停止，报告容器日志 `docker logs lucas-verify`，绝不把坏镜像发往服务器。

## 第二步：上传并更新服务器（等用户提供服务器信息后执行）

用户会把服务器连接信息发给你；上传方式届时按用户指定执行，默认路径是 `docker save | ssh docker load`：

1. 导出并传输（一条命令完成，避免落盘 tar 文件）：
   ```bash
   docker save lucas:last | ssh <user>@<host> 'docker load'
   ```
   若用户要求 scp 方式，则 `docker save -o /tmp/lucas-last.tar lucas:last` → `scp` → 服务器 `docker load` → 删除本地与服务器上的 tar。
2. 更新宝塔 lucas 编排：编排文件中 `new-api` 服务的 `image:` 必须引用 `lucas:last`（若仍是 `calciumion/new-api:latest`，先与用户确认改为 `lucas:last`）。然后：
   - 宝塔面板：Docker → Compose 项目 → lucas → 重新构建/重启；或
   - SSH 执行：在编排目录 `docker compose up -d new-api`（先 `docker compose pull 2>/dev/null || true` 无必要可跳过——镜像是本地加载的）。
3. **先验证再替换的替代法**（用户要求过验证环节；若用户希望更稳妥，用此法）：在服务器上先起新容器到临时端口 `docker run -d --name lucas-verify -p 127.0.0.1:3100:3000 --env-file <与正式一致的环境> lucas:last`，`curl http://127.0.0.1:3100/api/status` 通过后再重建正式容器；验证容器随后删除。
4. 正式验证：`curl -fs http://<服务器>:3000/api/status` 返回 success；请用户在浏览器确认关键功能（登录、日志页、渠道页）正常。用户未确认前不算部署完成。

## 第三步：收尾清理（验证通过后才做）

1. 服务器：`docker image prune -f` 清悬空层；确认旧镜像（如 `calciumion/new-api:latest`、旧 `lucas` 悬空镜像）不再被任何容器引用后，用 `docker rmi <旧镜像>` 显式删除。删除前列出将删的镜像名让用户过目。
2. 本机：删除构建中间产物与验证容器——`docker rmi lucas:last`（若用户同意）、清理 `/tmp` 下的 tar（如有）。前端/后端构建产物本来就在容器内完成，本地工作树不应有新增未跟踪文件；有则检查是否为构建残留，征得同意后删除。
3. 记录本次部署：release 短哈希、镜像构建时间、验证结果，写入中文报告。

## 红线

- 只在 `release` 分支基础上构建；绝不从 `main`、`custom` 或脏工作树构建。
- 数据安全：不动服务器上的数据卷（`./data`、postgres/redis 卷）、不清数据库、不改编排里的密码与 DSN。
- 更新编排仅限 `image:` 引用与重建容器；其它字段变更须先问用户。
- 回退预案：重建前记录旧镜像名/ID（`docker inspect lucas-new-api的镜像`），验证失败时能 `docker tag <旧镜像> lucas:last && docker compose up -d` 快速回滚。
- 不泄露凭据；不在报告里写 IP 以外的敏感信息时也先征询。

## 中文完成报告（每次必须）

- **构建**：release 短哈希、平台、镜像 `lucas:last`、本地验证结果。
- **上传与更新**：传输方式、编排更新动作、正式验证结果（api/status + 用户浏览器确认）。
- **清理**：删除了哪些本地/服务器旧镜像与文件，保留了什么。
- **遗留**：未验证项、回退说明（旧镜像 ID）。
