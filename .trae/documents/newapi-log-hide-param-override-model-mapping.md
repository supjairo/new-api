# 计划：通用日志接口对普通用户隐藏「参数覆盖」「模型映射」

## Summary

在日志返回层（`model/log_other.go` 的用户视角投影）增加字段过滤：普通用户通过 `/api/log/self`、`/api/log/token` 查询日志时，响应中的 `other` 不再包含 `po`（参数覆盖审计）、`is_model_mapped`、`upstream_model_name`（模型映射）三个键。不改日志写入、不改数据库存储、不改前端；管理员与 Root 查看日志完全不受影响。前端详情弹窗的「模型映射」「参数覆盖」区块均为数据驱动渲染（键不存在即不渲染），后端剥离后自动隐藏，无需改前端代码。

## Current State Analysis

### 数据写入（本次不改）

* [service/log\_info\_generate.go#L115-L118](file:///Users/jairo/Desktop/newapi-custom/service/log_info_generate.go)：`relayInfo.IsModelMapped` 时 `SetPublic("is_model_mapped", true)`、`SetPublic("upstream_model_name", ...)`。

* [service/log\_info\_generate.go#L150-L155](file:///Users/jairo/Desktop/newapi-custom/service/log_info_generate.go)：`appendParamOverrideInfo` 用 `SetPublic("po", relayInfo.ParamOverrideAudit)` 写参数覆盖审计行。

* [service/task\_billing.go#L59-L62](file:///Users/jairo/Desktop/newapi-custom/service/task_billing.go)、[L166-L167](file:///Users/jairo/Desktop/newapi-custom/service/task_billing.go)：任务结算的消耗日志（`is_task=true`，同样显示在通用日志页）也以 `SetPublic` 写入 `is_model_mapped`/`upstream_model_name`。

* 以上均以 `SetPublic` 写入，存储为 `Log.Other` JSON 顶层字段，历史日志已落库。

### 返回链路（本次修改点）

| 端点                   | 鉴权        | 调用链                                                                                                                                                           | 投影视角         |
| -------------------- | --------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------ |
| `GET /api/log/self`  | UserAuth  | `controller.GetUserLogs` → `model.GetUserLogs` → `formatUserLogs`（[model/log.go#L115-L121](file:///Users/jairo/Desktop/newapi-custom/model/log.go#L115-L121)） | User         |
| `GET /api/log/token` | TokenAuth | `controller.GetLogByKey` → `model.GetLogByTokenId` → `formatUserLogs`                                                                                         | User         |
| `GET /api/log/`      | AdminAuth | `controller.GetAllLogs` → `FormatAdminLogs`（非 root）/ `FormatRootLogs`（root）                                                                                   | Admin / Root |

* 用户视角投影收敛在 [model/log\_other.go#L215-L263](file:///Users/jairo/Desktop/newapi-custom/model/log_other.go#L215-L263) `formatLogOtherJSON`：User 视角目前只删 `admin_info`/`root_info`/`audit_info` 和 `legacySensitiveLogOtherKeys`（channel\_id/channel\_name/channel\_type/reject\_reason），**不删** `po`/`is_model_mapped`/`upstream_model_name` —— 这就是泄露点。

* Admin 视角仅删 `root_info`，Root 视角全量保留 → 管理员不受本次改动影响。

### 前端展示（本次不改）

* [details-dialog.tsx#L1136-L1151](file:///Users/jairo/Desktop/newapi-custom/web/src/features/usage-logs/components/dialogs/details-dialog.tsx)：`!response_model && other.is_model_mapped && other.upstream_model_name` 时渲染「模型映射」区块。

* [details-dialog.tsx#L1302-L1330](file:///Users/jairo/Desktop/newapi-custom/web/src/features/usage-logs/components/dialogs/details-dialog.tsx)：`other.po` 非空数组时渲染「参数覆盖」区块。

* 两处均无 `isAdmin` 门控，纯数据驱动：后端键被剥离后区块自动消失。

## Proposed Changes

### 1. `model/log_other.go` — 用户投影增加三个受限键（核心改动）

* 新增包级变量（放在 `legacySensitiveLogOtherKeys` 旁边）：

```go
// userHiddenLogOtherKeys are channel-config-derived fields (model mapping
// result and parameter override audit) that remain in stored logs for
// admin/root review but are stripped from user/token-visible projections.
var userHiddenLogOtherKeys = []string{
	"is_model_mapped",
	"upstream_model_name",
	"po",
}
```

* 在 `formatLogOtherJSON` 的 `logOtherVisibilityUser` 分支中，紧跟 `legacySensitiveLogOtherKeys` 删除循环之后，追加同样的删除循环。

**为什么不动** **`legacySensitiveLogOtherKeys`**：该列表同时被 `SetPublic` 用于拒绝写入（[log\_other.go#L48-L55](file:///Users/jairo/Desktop/newapi-custom/model/log_other.go#L48-L55)）。若把这三个键加进去，新日志写入时 `SetPublic` 会直接拒绝，管理员也看不到了——与「管理员不受影响」矛盾。因此用独立的只读剥离列表。

**为什么选读取时剥离而非改写** **`SetAdmin`**：

* 需求明确要求「直接在接口处添加返回控制」，不改写入路径与已落库数据。

* 读取时剥离对**历史日志同样生效**；改写入口只能保护新日志。

* 单点收口：`formatLogOtherJSON` 是用户视角唯一投影点，普通用户所有日志查询（含任务结算消耗日志）都被覆盖。

### 2. `model/log_format_test.go` — 补充回归测试（扩展现有文件，不新建）

沿用文件内 `TestLegacyLogOtherVisibilityIsRoleSeparated` 的表驱动风格（testify），新增一个测试函数，覆盖：

* User 投影：`po`、`is_model_mapped`、`upstream_model_name` 被剥离；同一条日志的普通公开键（如 `model_ratio`）保留；嵌套 `admin_info` 仍被剥离。

* Admin 投影：三个键保留（管理员不受影响），`root_info` 仍被剥离。

* Root 投影：三个键保留。

## Assumptions & Decisions

1. **「不要修改代码」的理解**：指不改日志写入/落库数据、不改前端展示代码，仅在接口返回处加控制。若理解有误（例如你想要的是连存储都改掉），在评审时指出即可。
2. **`response_model`** **不在本次范围**：`other.response_model`（「响应模型」区块）也含 `upstream_model` 字段，前端标签为「Response Model」而非「模型映射」，属官方的模型不一致标记功能。你只点名了两项，故不动；如需一并隐藏，评审时说明，加进 `userHiddenLogOtherKeys` 即可（一行）。
3. 无数据库行为变更（纯读时 JSON 投影，无 schema/驱动/SQL 改动），不触发三数据库验证矩阵。
4. 无前端改动、无 i18n 改动、`relaykit/` 不涉及。
5. 不需要新增文档，不改 `docs/`。

## Verification

1. `go build ./...`（根模块编译通过）。
2. `go test ./model/ -run 'TestLogOther|TestLegacyLogOther|TestUserLogProjection' -v`（新测试与既有 log\_other/log\_format 测试全绿）。
3. `go test ./service/ -run 'TestQuotaSaturation|TestLogOther' -v`（确认写入口相关既有测试不受影响）。
4. 人工核对（可选）：以普通用户 token 调 `/api/log/self`，确认响应 `other` 无 `po`/`is_model_mapped`/`upstream_model_name`；以管理员调 `/api/log/`，确认三个键仍在。
5. 完成汇报：列出改动文件、测试结果，并明确说明 `response_model` 未包含在本次范围内。

