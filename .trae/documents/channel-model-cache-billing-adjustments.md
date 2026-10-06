# 渠道级模型缓存用量折算实施计划

## 一、摘要与已确认决策

在每个渠道内配置“客户端模型名 → 读缓存百分比、写缓存百分比”。只影响该渠道实际执行请求的缓存计费用量和消费日志，不修改全局模型单价、上游响应、原始 usage 或上下文长度。

用户已确认：
- 首版覆盖 Chat/Completions、Responses（含 Compact）、Claude、Gemini 的 HTTP/SSE 文本结算。
- 暂不覆盖独立音频、Realtime、图片生成/编辑、异步任务、Embeddings、Rerank、Moderations、搜索等独立业务计费；文本入口中实际含音频用量的请求也不调整。
- 允许百分比小数；折算后的 token 向下取整，日志与计费使用同一整数结果。例如 100 × 10% = 10，3 × 10% = 0。
- 按客户端 OriginModelName 大小写敏感、完整名称精确匹配；不匹配上游映射名或计费别名，不提供通配符与继承。
- 无规则或缺少某一比例默认 100%；显式 0% 有效。读、写分别配置；写比例同时作用于普通、5分钟、1小时写缓存，各自原有价格不变。
- 被免除的缓存部分不转回普通输入计费；输入、输出其他用量不变。

## 二、现状分析

1. `relaykit/dto/channel_settings.go` 的 ChannelSettings 经 `model/channel.go` 的 setting TEXT JSON 保存，现有创建/更新 API 共用 ValidateSettings。
2. `middleware/distributor.go:588-593` 注入所选渠道设置，`relay/common/relay_info.go:284-286` 放入当前 ChannelMeta。渠道切换后重新初始化，结算读取当前渠道即可，不增加全局规则查询或额外规则缓存。
3. `service/billing_usage.go` 的 effectiveBillingUsage 恢复保留的供应商快照，应保持只处理协议事实的职责。
4. `service/text_quota.go` 的 PostTextConsumeQuota 归一化后进行亲和观测、传统 summary 计算、表达式求值、SettleBilling 和消费日志。其他业务也会调用此入口，因此必须按实际请求模式限制范围，不能仅凭进入函数就调整。
5. OpenAI prompt 通常包含缓存，Claude 普通输入不含缓存。直接缩小 usage 缓存字段后沿用旧计算，会把免除部分重新计入普通输入。
6. `service/tiered_settle.go` 的 BuildTieredTokenParams 按表达式使用变量拆分类别，Len 保持上下文长度。该函数还有音频、Realtime、预览等调用者，不改变其公共默认行为。
7. `web/src/features/channels/lib/channel-form.ts` 的 buildSettingJSON 重建已知 setting 字段，必须同时扩展读取、表单校验和保存，否则再次编辑会丢规则。

## 三、配置契约与界面

### 后端契约

在 `relaykit/dto/channel_settings.go` 增加 `cache_billing_adjustments` map：

```json
{
  "cache_billing_adjustments": {
    "model-a": { "read_percent": 100, "write_percent": 10 },
    "model-b": { "read_percent": 50, "write_percent": 12.5 }
  }
}
```

每个比例用 `*float64` + omitempty 区分缺省和显式零。缺省/null 比例按100；规则本身必须为对象，拒绝错误结构、空白模型名、非数值、非有限值、负数、超过100的值。模型名不自动修剪或改写。保存错误包含模型名及字段。运行时遇到非法历史字段，记录诊断并将该字段按100处理。

`model/channel.go` 的 ValidateSettings 调用新增规则校验；沿用已有渠道API、TEXT列、渠道缓存刷新，不增加列、索引、迁移、配置接口或依赖。

### 首版界面

在渠道抽屉高级设置区域添加“模型缓存用量折算”JSON输入，复用 `web/src/components/json-code-editor.tsx` 和既有 RHF/FormMessage；提供两个模型的简短示例与范围、缺省、取整、仅文本生效的说明。空输入/空对象代表删除全部规则。

组件复用决策：现有 ModelMappingEditor 只支持字符串到字符串映射，不能直接承载两个数值属性；NumericSpinnerInput 只接受整数且空值回落0，不适合百分比小数和缺省100。首版已有 JsonCodeEditor 可完整表达规则，无需新建表格组件或扩展公共组件。保留后续结构化界面的空间，但本次不同时实现两种编辑器。

修改：
- `web/src/features/channels/types.ts`：渠道设置规则类型。
- `web/src/features/channels/lib/channel-form.ts`：schema/defaults/解析/buildSettingJSON，校验错误并保留显式0。
- `web/src/features/channels/lib/channel-form-errors.ts`：错误归属高级设置区域。
- `web/src/features/channels/components/drawers/channel-mutate-drawer.tsx`：复用JSON控件、字段错误、权限锁定与说明。
- `web/src/i18n/locales/{en,zh,zh-TW,fr,ja,ru,vi}.json`：英文源字符串key及七语翻译。实施前加载 i18n-translate 与 vercel-react-best-practices 技能。

## 四、统一计费调整层

在 `service/text_quota.go` 建立请求局部缓存调整快照，作为稳定业务概念由两套计费与日志复用。不要修改原始 dto.Usage，不把折算塞入 effectiveBillingUsage，不逐适配器实现。

步骤：
1. 取得原始 canonical usage，并按原用量完成现有亲和观测。
2. 检查首版请求范围、最终渠道、OriginModelName，解析规则。未命中或双100直接原路径，不产生审计附加字段。
3. 收集原始缓存读、写、TTL拆分事实。OpenRouter推算先使用原始cost、原始用量与原价格，保留现有适用条件及容量检查，不从调整量或表达式价格反推。
4. 以 decimal 乘比例并 Floor，使用 common 集中安全转换，不裸转int。100%直接保留原整数，避免改变历史值。任何安全夹紧沿用现有审计规则。
5. 传统计算及表达式参数调整读取同一快照；日志在最终收费模式确定后使用对应结果。禁止重复乘比例。

### 传统倍率

扩展 calculateTextQuotaSummary 接收可选调整快照，原路径保持兼容。
- 普通输入拆分始终按原始缓存数量，保留 OpenAI/Claude/legacy/OpenRouter 分支和原有零下限。
- 缓存收费项使用调整后的数量乘原有缓存倍率。
- TTL存在时先划分原始普通写余量 `max(total-5m-1h,0)`，三个收费桶分别Floor，调整后计费总写量由实际收费桶合计，日志与之相同。不对聚合与子桶重复收费。
- 工具附加费、模型/分组倍率、OtherRatios、音频既有逻辑、最终quota转换/最低收费政策不改变。
- 固定按次收费不因token折算降价，继续原日志口径，不标记为token折算结算。

### 表达式

在 `service/tiered_settle.go` 增加文本专用调整逻辑；公共 BuildTieredTokenParams 默认行为不变。
- 先基于原始usage与 usedVars 生成原始参数，保留 Len/C/非缓存图片等事实。
- 对独立计价的CR/CC/CC1h使用快照调整量。
- 若缓存类别留在兜底P，减掉该类别原始贡献与调整贡献的差值，避免免掉的缓存重新按普通输入收费；P保留零下限。
- Anthropic未引用cr时，只把调整后的读缓存贡献留在P。现有未引用写缓存不加回P的行为不顺带修改。
- 图片缓存：沿用现有有效明细验证及img_cr opt-in契约。引用img_cr且明细有效时，先按原始明细互斥拆分，再分别Floor普通读缓存与图片读缓存；调整读缓存总量由两桶合计，Img非缓存部分不变。未引用img_cr但图片缓存明确留在已计价Img内时，扣除该Img中的免除缓存贡献；P只扣实际仍留在P中的缓存贡献，按类别交集排除已拆出部分，不重复扣图片缓存。无效/缺失明细不猜测，不新拆Img，保持现有聚合回退及诊断。
- 同一个缓存交集在原表达式的多个项目中被计价时，不顺带重写历史归一化，调整其各项目原有缓存贡献，100%结果须逐值不变。
- OpenRouter由cost推算的写缓存仅在现有传统路径适用；表达式仍以其原有归一化事实为准，不为本功能补造原本未计价的写缓存。对该路径原来没有的缓存事实不产生免除量。
- 求值成功且实际为token叶子时，记录实际TokenParams到结算结果供既有日志注入使用；失败保留预扣及原日志口径，不伪造实际计价参数。
- 固定价叶子保持按次金额及原日志口径；表达式条件本身若依赖被调整计价变量，仍忠实执行表达式，不另行锁定原始分支。Len保持原值。

## 五、日志与不变量

修改 `service/text_quota.go` 的缓存日志生成：
- 成功token折算时，cache_tokens、cache_creation_tokens、5m/1h、cache_write_tokens显示实际用于本次计算的整数数量；折算为0显式保存0。
- 同步实际billing_tokens、适用的image_cache_tokens，与表达式真实输入一致，复用现有 InjectTieredBillingInfo。
- 在 `other.admin_info.cache_billing_adjustment` 保存客户端模型名、百分比、原始缓存、调整缓存、写缓存来源；普通用户不暴露原始量。
- prompt_tokens、completion_tokens、input_tokens_total、Len、性能输出统计保持原始口径，不伪造总输入。后台明细用于解释总输入与折算缓存的差异。
- 缺省/100%、排除范围、固定价、求值失败不增加折算标记。
- 保留原有预扣估算、失败退款、余额/token/渠道累计更新；不为了百分比改变预扣或最低quota规则。0%只承诺对应缓存收费量为0，不承诺传统整单免最低扣费。
- 配置在当前请求ChannelMeta中读取，不结算时重新读数据库；成功重试使用最终执行渠道设置。

## 六、集中回归验证

### 后端

扩展现有 `service/text_quota_test.go`，不跨层散建测试文件。使用require/assert、确定性表格fixture，覆盖：
- 配置缺省、null、显式0、小数、边界100、非法值/结构；JSON保存读取。
- 渠道A/B同模型规则隔离、客户端名精确匹配、映射名不匹配、最终渠道重试。
- 原读缓存100配10%：普通输入不增加、收费读缓存10、日志10、钱包/token/渠道累计与最终quota一致。
- 写缓存普通/5m/1h分别Floor，聚合等于收费桶合计；3配10%为0且日志显式0。
- OpenAI/Claude、OpenRouter原始cost推算、读写重叠输入零下限。
- 表达式引用/不引用缓存变量，Len原值、图片缓存有效/缺失/无效明细及不同usedVars交叠。
- 原始usage与亲和观测不变；缺省/100%逐值保持。
- 固定价、表达式失败、无usage、排除的audio/image/realtime路径不折算。

### 前端

扩展 `web/src/features/channels/components/__tests__/channel-configuration.test.tsx`：加载回显、JSON编辑、显式0/小数、非法规则错误、删除全部规则、权限禁用、API保存失败草稿保留、再次保存不丢配置。必要的纯表单转换断言也集中于这次规则职责，不新增无关测试。

### 数据库要求

新增JSON设置的持久化往返须在真实SQLite、MySQL、PostgreSQL验证，复用现有测试DSN入口 `TEST_FIXED_MYSQL_DSN`/`TEST_FIXED_POSTGRES_DSN` 及对应日志库fixture。无列/索引/迁移变更，不引入额外迁移工程。覆盖旧渠道无配置、更新/删除规则、0及小数保存；日志JSON读写亦验证。记录实际版本、命令与结果；缺少服务/DSN导致skip是验证阻塞，不宣称三库兼容或完成。

### 实施时执行的命令（当前未运行）

从仓库根目录执行：
- gofmt 修改的Go文件。
- `go test ./service -run 'TestCacheBillingAdjustment' -count=1`（新增测试统一此业务前缀）。
- 带真实三库DSN运行对应集中fixture，并确认未skip。
- `go test ./service ./model -count=1`。
- `go build ./...`。

从relaykit执行：`GOWORK=off go build ./...`，确认模块独立。

从web执行：
- `bun run test src/features/channels/components/__tests__/channel-configuration.test.tsx`。
- `bun run typecheck`。
- `bun run lint`。
- `bun run format:check`。
- `bun run build`。

## 七、实施顺序与交付

1. 建立集中配置/费用/日志回归fixture，明确100%和排除范围的原行为。
2. 实现渠道DTO、保存校验与前端读写契约。
3. 实现一次性缓存快照、传统输入原始拆分与调整收费。
4. 实现文本表达式调整、原始Len与交叠保护、实际参数日志及管理员审计。
5. 接入复用JSON编辑器、国际化、权限和字段错误。
6. 执行三库与相关测试、独立relaykit及前后端构建；汇报实际结果与阻塞。

只修改上述功能必需文件；不改全局价格、不提交、不推送、不部署、不新增仓库业务文档。当前仅生成此计划，等待批准后实施。
