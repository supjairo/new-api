# 计划：错误请求审计（独立表 + 使用日志审计列，管理侧专属）

## 需求摘要（三轮反馈汇总）

1. **独立数据表**：错误请求记录单独建表（不塞 `logs.other`，不建新页面）。
2. **无开关**：不需要任何启用/关闭控制，始终记录。
3. **使用日志增加「审计」列**：在使用日志（usage-logs）错误行上增加管理侧专属的审计入口，权限设计参考现有 `admin_info` 角色分层；用户侧不可见。
4. **全错误类型 + 显示一致**：上游/渠道错误和本地服务产生的错误（参数校验失败、额度不足、令牌无效等）都要记录；用户端看到什么错误，记录里就是什么错误（含渠道复写 override 与 request id 后缀）。

## 现状问题

1. 错误日志（`logs` 表 type=5）在 `ProcessChannelError` 逐渠道尝试时写入（[service/relay\_error.go#L76-L104](../../service/relay_error.go#L76-L104)），content 为**原始上游错误**；用户实际收到的是 `Relay()` defer 中复写后 + request id 的最终消息（[controller/relay.go#L97-L113](../../controller/relay.go#L97-L113)）→ 显示不一致。
2. 本地拒绝错误（`ErrOptionWithNoRecordErrorLog`）、任务错误（`respondTaskError`）、MJ 错误、渠道选择失败等不产生错误日志 → 用户看到错误但日志缺失。
3. `ERROR_LOG_ENABLED` 默认关闭；错误日志不记录请求体。

## 设计决策

| 决策点     | 结论                                                                                                                               |
| ------- | -------------------------------------------------------------------------------------------------------------------------------- |
| 独立表     | LOG\_DB 新表 `error_requests`（与 logs 同库，含 ClickHouse DDL 分支）；随现有日志清理任务过期删除                                                         |
| 记录时机    | **最终响应点**（Relay defer / respondTaskError / RelayMidjourney），每失败请求至多一条；无任何开关                                                      |
| 显示一致    | `error_message` = 用户实际收到的最终消息（`ApplyErrorOverride` 复写后 + request id）；状态码/错误类型/错误码均取最终值                                           |
| 请求体快照   | 截断（默认 4KB，env `ERROR_REQUEST_BODY_LIMIT_KB` 可调）；multipart 记占位符；超长标记 `body_truncated`；`MaskSensitiveInfo` 脱敏                      |
| 渠道原始错误  | `ProcessChannelError` 中逐渠道脱敏后暂存 context，最终点并入 `channel_errors` 字段（管理侧对比原始 vs 最终）                                                 |
| 现有错误日志行 | 记录点同步迁移到最终点，content 同为用户所见消息（消除不一致）；`constant.ErrorLogEnabled` 开关随之删除                                                            |
| 使用日志 UI | 管理端错误行增加「审计」列，点击按 `request_id` 从独立表拉取详情弹窗                                                                                        |
| 不在范围    | 管理控制台自身 API 错误、`RelayNotFound`/`RelayNotImplemented`、task plugin 自渲染错误；relaykit 的 `IsRecordErrorLog` API 保留但不再消费（不动 relaykit 模块） |

## 实施步骤

### 后端

1. **新模型** — 新建 `model/error_request.go`

   * `ErrorRequest` 字段：`Id, CreatedAt, UserId, Username, TokenId, TokenName, ChannelId, ModelName, RequestPath, Method, StatusCode, ErrorType, ErrorCode, ErrorMessage, RequestBody, BodyTruncated, ChannelErrors(JSON), Group, IsStream, RetryCount, RequestId`；索引：created\_at、user\_id、username、model\_name、status\_code、request\_id。

   * `MigrateErrorRequests()`：仿 `MigrateAuditLogs`（[model/audit\_log.go#L243-L255](../../model/audit_log.go#L243-L255)）——SQL 库 `LOG_DB.AutoMigrate`；ClickHouse `CREATE TABLE IF NOT EXISTS`（MergeTree，`ORDER BY (created_at, id)`）。

   * `RecordErrorRequest`、`GetErrorRequestByRequestId(requestId)`（管理侧详情用）。

   * 清理：`CountOldErrorRequests` / `DeleteOldErrorRequestsBatch`（仿 [model/log.go#L697-L739](../../model/log.go#L697-L739)，含 ClickHouse 单次 mutation 分支）。

2. **注册迁移** — [model/main.go](../../model/main.go#L395-L403) `migrateLOGDB()` 中 `MigrateAuditLogs()` 后调用 `MigrateErrorRequests()`。

3. **配置** — [constant/env.go](../../constant/env.go)、[common/init.go](../../common/init.go)：新增 `ErrorRequestBodyLimitKB`（`ERROR_REQUEST_BODY_LIMIT_KB`，默认 4）；**删除** **`ErrorLogEnabled`**（[common/init.go#L199](../../common/init.go#L199) 及其全部引用）。

4. **请求体快照** — 新建 `service/error_request.go`

   * `snapshotErrorRequestBody(c)`：`common.GetBodyStorage(c)` 失败 → `"[request body unavailable: ...]"`；multipart → `"[multipart body, N bytes]"`；其余 Seek 0 后 `io.LimitReader` 读 limit+1 字节，超限置 truncated 并补省略标记，读后 Seek 复位；结果 `MaskSensitiveInfo` 脱敏。

5. **渠道错误暂存** — [service/relay\_error.go](../../service/relay_error.go) `ProcessChannelError`

   * 保留自动禁用（`DisableChannel`）与日志输出；移除 `model.RecordErrorLog` 写入段。

   * 追加 `{channel_id, status_code, message(脱敏截断)}` 到新 `constant.ContextKeyChannelErrorTries`（复用 `common.SetContextKey`）。

6. **最终记录** — `service/error_request.go` 新增 `RecordFinalErrorLog(c *gin.Context, apiErr *types.NewAPIError, relayInfo *relaycommon.RelayInfo)`

   * 调用点保证已过 `ApplyErrorOverride` + `SetMessage`；`c == nil` 容错，`relayInfo` 可为 nil（从 c 取 model/token 等，失败时留空）。

   * 写独立表一条记录；同时写一条 `logs` type=5 错误日志行（复用 `model.RecordErrorLog`，other 复用现有 `AppendRelayLogAdminInfo`/`AppendResponseModelLogInfo`/`AppendTaskPluginContextAuditInfo` 与 `error_override` 审计），content = 同一最终消息，保证使用日志行显示一致。

   * `gopool.Go` 异步插入，goroutine 只使用已拷贝普通值，不引用 `gin.Context`。

7. **接入最终响应点** — [controller/relay.go](../../controller/relay.go)

   * `Relay()` defer：`newAPIError.SetMessage(...)`（L98）后调用（relayInfo 可能未生成，传 nil 容错）。

   * `respondTaskError`（L781）：429 文案改写后调用。

   * `RelayMidjourney`（L341 分支）：记录最终 description。

   * 检查 `controller/responses_websocket.go`：若有独立的客户端错误写出点则同样接入，已覆盖则不动。

8. **查询接口** — 新建 `controller/error_request.go`：`GetErrorRequestByRequestId`；[router/api-router.go](../../router/api-router.go#L313-L320) 附近新增 `GET /api/error_request/:requestId`（`middleware.AdminAuth()`）。

9. **清理集成** — [service/system\_task.go](../../service/system_task.go#L359) 日志清理任务：同 `TargetTimestamp` 对 `error_requests` 执行计数+分批删除，计入任务统计。

10. **后端测试** — 新建 `service/error_request_test.go`（唯一新测试文件，testify）

    * 快照逻辑表测试：截断与标记、multipart 占位符、脱敏、GetBodyStorage 失败占位。

    * 接入点回归（仿 [controller/relay\_error\_log\_test.go](../../controller/relay_error_log_test.go)）：复写命中 → 独立表与 logs 行的 message 均等于复写后消息（含 request id）；本地拒绝错误也产生记录；channel\_errors 含原始错误。

    * 修正因删除 `ErrorLogEnabled` 与迁移记录点受影响的既有测试（[service/relay\_error\_test.go](../../service/relay_error_test.go)、[service/error\_test.go](../../service/error_test.go)、[controller/relay\_task\_plugin\_test.go](../../controller/relay_task_plugin_test.go) 相关用例）。

### 前端

1. **使用日志「审计」列** — `web/src/features/usage-logs/`

   * 实现前按 web/AGENTS.md 检索 `components/columns/`、`details-dialog.tsx` 现有列与权限渲染模式（如渠道列的 admin-only 处理）。

   * `common-logs-columns.tsx` 增加一列：仅当前登录者为管理员且行 type=5 时渲染审计按钮；用户侧不渲染。

   * 点击弹窗按行 `request_id` 调 `GET /api/error_request/:requestId`：展示用户可见最终错误（与行内消息一致）、逐渠道原始错误对比、复写审计（error\_override）、请求体快照（可复制、JSON 尽力格式化）；复用 `@/components/dialog` 与 `@/components/copy-button`。

   * 管理端列表 API 已按角色投影 `other`（用户侧剥离 admin\_info），前端只需按登录角色控制渲染。

2. **i18n** — 新增 key 后 `bun run i18n:sync` 同步 en/zh/zh-TW/fr/ja/ru/vi。

### 验证

* 后端：`go build ./...`；`go test ./service/... ./controller/... ./model/...`；`gofmt` 修改文件。

* **三库迁移验证（新表，强制）**：SQLite、MySQL、PostgreSQL 分别启动验证 `error_requests` 自动创建；同库连续启动两次验证幂等；各库插入+查询一次；以最近发布版本库升级一遍。ClickHouse 无本地实例则明确报告阻塞项。

* 前端：`bun run typecheck`、涉及文件 `bun run lint`、相关 vitest、`bun run build`。

* 手工链路（核心验收 = 显示一致）：

  1. 上游错误 + 命中渠道复写 → 用户收到消息 ≡ 使用日志错误行 content ≡ 独立表记录（含 request id）。
  2. 本地拒绝（无效参数、额度不足）→ 同样有记录且消息一致。
  3. 多渠道重试后失败 → 使用日志一条错误行；审计弹窗可见逐渠道原始错误与请求体。
  4. 普通用户登录 → 错误行无审计列，接口返回不含请求体等管理侧信息。

## 风险与说明

* 行为变更：错误日志从「逐渠道每尝试一条」变为「每失败请求一条」，中间渠道失败不再单独成行（原始错误保留在审计记录 channel\_errors）；`ERROR_LOG_ENABLED` 开关删除，错误日志/错误请求始终记录。

* 表增长：请求体上限 4KB、随日志清理任务过期删除；无开关按用户要求。

* 响应延迟：快照为上限 4KB 的内存读取，插入异步化。

