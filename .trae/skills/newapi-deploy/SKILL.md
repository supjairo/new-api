---
name: newapi-deploy
description: 构建 lucas:last 自定义镜像并部署到宝塔服务器 lucas 编排（部署目标是 nhk:latest 标签），含验证替换与强制收尾清理。用户要求部署、发布、更新服务器时使用；不用于同步分支。
---

# New API 部署（lucas:last → nhk:latest 三步走）

目标：把 `release` 合并线的代码构建为本地镜像 `lucas:last`，上传到用户的服务器，替换宝塔面板 Docker Compose 项目 **lucas** 中 `new-api` 服务使用的镜像，验证通过后**强制执行收尾清理**。全程不得改动 `main`/`custom` 分支，不得动官方镜像 `calciumion/new-api`。

## 服务器事实（已确认，勿再假设）

- 服务器：宝塔面板主机，编排目录 `/www/dk_project/dk_app/newapi/lucas/docker-compose.yml`，编排项目名 `lucas`，服务容器 `lucas-new-api-1`，内部端口 3000。
- 编排 `new-api` 服务的 `image:` 引用的是 **`nhk:latest`**（不是 `lucas:last`，也不是 `calciumion/new-api`）。部署方式：上传 `lucas:last` 后在服务器 `docker tag lucas:last nhk:latest`，编排文件无需改动。仅当服务器上 `image:` 与此处描述不符时才询问用户。
- 服务器本机访问 `http://127.0.0.1:3000` 可验证；外部可能有防火墙/反代，直连 IP:3000 不通不代表失败。最终可用性以服务器本机 curl + 用户浏览器确认为准。
- SSH 密码方式连接需要 `sshpass`（本机已装于 `/usr/local/bin/sshpass`）。凭据由用户消息提供；不回显、不落盘。
- 若实际情况与以上任何一条不符（编排路径变了、image: 变了），先如实告知用户再继续。

## 部署前置检查

1. `git status --short --branch`：工作树必须干净（`.trae/` 等未跟踪文件可忽略），当前在 `release` 分支且与 `origin/release` 一致（`git rev-parse release origin/release` 相同）。不满足时先停下询问。
2. 确认 `git diff custom release` 为空——部署内容必须是"官方最新 + 全部自定义"。
3. 部署会改线上服务：**开始前必须向用户确认**"即将部署 `<当前 release 短哈希>` 到服务器 lucas（镜像 tag nhk:latest）"，得到明确同意才继续。
4. 服务器连接信息由用户以消息提供；不要在输出中回显密码、密钥，不要把凭据写入任何文件。

## 第一步：本地构建镜像（在本机执行）

1. 依赖构建工具：`docker`（本机可用 `docker version` 验证，缺失时报告并停止）。构建用仓库自带 [Dockerfile](Dockerfile)（多阶段：bun 构建前端 → go 构建后端 → debian 运行时），无需本地装 Go/Node。
2. 构建（在仓库根目录）：
   ```bash
   docker build --platform linux/amd64 -t lucas:last .
   ```
   服务器是 amd64；若用户说明服务器为 arm64，则改用 `--platform linux/arm64`。
3. 本地验证镜像：`docker run -d --rm --name lucas-verify -p 127.0.0.1:3100:3000 lucas:last`，等几秒后 `curl -fs http://127.0.0.1:3100/api/status` 应返回 `"success":true`，然后 `docker stop lucas-verify`。验证失败则停止，报告容器日志 `docker logs lucas-verify`，绝不把坏镜像发往服务器。

## 第二步：上传并更新服务器

1. 记录回退点（重建容器之前）：在服务器上执行
   ```bash
   docker inspect lucas-new-api-1 --format '{{.Image}}'   # 当前运行容器的镜像 ID
   docker images --format '{{.Repository}}:{{.Tag}} {{.ID}}' | grep nhk
   ```
   把旧 `nhk:latest` 的镜像 ID 记入部署报告。这个 ID 是验证失败时的回滚依据。
2. 导出并传输（一条命令完成，避免落盘 tar 文件）：
   ```bash
   docker save lucas:last | sshpass -p '<密码>' ssh -o StrictHostKeyChecking=accept-new root@<host> 'docker load'
   ```
   若用户要求 scp 方式，则 `docker save -o /tmp/lucas-last.tar lucas:last` → `scp` → 服务器 `docker load` → 删除本地与服务器上的 tar。
3. 打标签并重建（镜像已加载为 lucas:last 后）：
   ```bash
   ssh root@<host> 'docker tag lucas:last nhk:latest && cd /www/dk_project/dk_app/newapi/lucas && docker compose up -d new-api'
   ```
   若服务器编排的 `image:` 引用与"服务器事实"一节不符，先与用户确认再改。
4. 正式验证：等容器 healthy 后在服务器本机执行
   ```bash
   curl -fs http://127.0.0.1:3000/api/status
   ```
   应返回 `"success":true`；再 `docker ps` 确认 `lucas-new-api-1` 为 healthy。外部直连 IP:3000 不通时不要误判失败（防火墙/反代），改问用户实际访问入口。请用户在浏览器确认关键功能（登录、日志页、渠道页）正常。用户未确认前不算部署完成。

## 第三步：收尾清理（验证通过后强制执行，不可跳过）

清理目标：服务器和本机不留任何本次部署产生的临时内容；旧版本镜像**不再保留**（用户已明确：验收完毕即删除，不做服务器留档回滚）。回退手段 = 重新检出旧 release 哈希构建镜像，因此报告中不要写"旧镜像 ID 可回滚"这种与实际不符的说法。

1. 服务器（按顺序）：
   ```bash
   docker rmi lucas:last            # 删除上传用标签（nhk:latest 指向同一镜像，不受影响）
   docker image prune -af           # 清理悬空镜像；旧版本镜像在此步被彻底删除
   ```
   然后核对：`docker images -a` 只剩在用镜像（nhk:latest、redis、mysql、其它业务镜像）；`docker system df` 的 Images RECLAIMABLE 应为 0B。检查 `/tmp/` 无部署 tar；无 `lucas-verify` 之类的临时容器（有则 `docker rm -f`）。
2. 本机：`docker rmi lucas:last`；删除 `/tmp` 下的部署 tar（如有）；确认无遗留验证容器。前端/后端构建产物都在容器内完成，本地工作树不应有新增未跟踪文件；有则检查是否为构建残留，征得同意后删除。
3. 记录本次部署：release 短哈希、镜像 ID、验证结果、清理结果（删了什么、服务器现存哪些镜像），写入中文报告。

## 红线

- 只在 `release` 分支基础上构建；绝不从 `main`、`custom` 或脏工作树构建。
- 数据安全：不动服务器上的数据卷（`./data`、mysql/redis 卷）、不清数据库、不改编排里的密码与 DSN。
- 更新编排仅限 `image:` 引用与重建容器；其它字段变更须先问用户。
- 清理是部署的强制收尾步骤：验证通过后必须执行第三步并核对结果，不得只删标签不删镜像，也不得跳过核对。
- 收尾清理中不做服务器留档回滚（用户明确要求验收后删旧镜像）；回滚 = 用旧 release 哈希重新构建。
- 不泄露凭据；不在报告里写 IP 以外的敏感信息时也先征询。

## 中文完成报告（每次必须）

- **构建**：release 短哈希、平台、镜像 `lucas:last` 及其 ID、本地验证结果。
- **上传与更新**：传输方式、tag 动作（lucas:last → nhk:latest）、旧镜像 ID（回退点记录）、正式验证结果（容器 healthy + 服务器本机 api/status + 用户浏览器确认）。
- **清理**：服务器删除了哪些镜像/容器/文件（附 `docker system df` 或镜像列表核对结果）；本机删除了什么；服务器现存镜像清单。
- **遗留**：未验证项、待用户决定事项；没有则不写。
