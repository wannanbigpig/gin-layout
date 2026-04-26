# 代码全量审查与优化报告

- 审查日期：2026-04-26
- 审查范围：当前工作区全部新增/修改代码，重点覆盖迁移、系统参数、系统字典、任务中心、审计日志、登录安全、路由权限、测试与文档。
- 结论：新增业务大体已接入既有框架，包括路由、迁移、默认菜单 API 绑定、审计 diff、测试与文档；但仍存在 1 个当前可复现测试失败，以及若干安全、审计可靠性、任务中心语义一致性问题。建议先修 P0/P1，再处理 P2/P3 优化。

## 已执行验证

| 命令 | 结果 | 说明 |
| --- | --- | --- |
| `go test ./...` | 失败 | `tests/admin_test/system_routes_test.go:238` 的 `TestSystemDictWriteFlow` 失败，字典 options 响应结构与测试预期不一致。 |
| `go run main.go -c ./config.yaml command migrate check` | 通过 | 迁移版本检查通过：2 个版本、4 个文件，版本范围 `20260425000001` 到 `20260425000002`。 |

## 业务融合检查

| 模块 | 融合情况 | 证据 | 结论 |
| --- | --- | --- | --- |
| 系统参数 | 已接入路由、迁移、缓存、审计、运行时敏感字段配置 | `internal/routers/admin_router.go`、`internal/service/sys_config/`、`data/migrations/20260425000001_init_table.up.sql`、`data/migrations/20260425000002_init_data.up.sql` | 功能链路完整，但敏感值展示和审计脱敏不足。 |
| 系统字典 | 已接入路由、迁移、多语言、测试 | `internal/controller/admin_v1/auth_sys_dict.go`、`internal/service/sys_dict/`、`tests/admin_test/system_routes_test.go` | 主链路完整，但 options 响应 contract 与测试断言不一致。 |
| 任务中心 | 已接入任务定义同步、手动触发、重试、取消、运行记录 | `internal/service/taskcenter/`、`internal/cron/registry.go`、`cmd/cron/tasks.go` | 基本可用，但重试/高风险/取消审计语义未闭环。 |
| 审计日志 | 已扩展 request log 查询、导出、敏感字段配置、变更 diff | `internal/middleware/audit_queue.go`、`internal/service/audit/request_log_manage.go`、`internal/pkg/utils/sensitive/fields.go` | 功能接入完整，但 queue 不可用时存在审计丢失风险。 |
| 路由与权限 | 新路由已注册，默认菜单 API 绑定已补充 | `internal/routers/admin_router.go`、`internal/service/access/menu_api_defaults.go`、`data/migrations/20260425000002_init_data.up.sql` | 融合较好，建议增加“路由注册与菜单 API 绑定一致性”测试。 |
| 迁移命令 | 已改为时间戳版本，检查命令可用 | `internal/console/migrate/`、`data/migrations/20260425000001_*`、`data/migrations/20260425000002_*` | 符合项目经验记录，验证通过。 |

## P0：必须先修复

### P0-01 系统字典 options 响应 contract 与测试预期不一致

- 现象：`go test ./...` 在 `TestSystemDictWriteFlow` 失败。
- 失败位置：`tests/admin_test/system_routes_test.go:238`
- 相关代码：
  - `internal/controller/admin_v1/auth_sys_dict.go:135` 到 `internal/controller/admin_v1/auth_sys_dict.go:145`
  - `internal/pkg/response/response.go:108` 到 `internal/pkg/response/response.go:119`
  - `tests/admin_test/system_routes_test.go:233` 到 `tests/admin_test/system_routes_test.go:238`
  - `tests/admin_test/system_routes_test.go:386` 到 `tests/admin_test/system_routes_test.go:400`
- 根因：`SysDictController.Options` 直接调用 `api.Success(c, options)` 返回 slice；全局 `response.WithData` 对非 object 数据会包装为 `data.result`，但测试 helper `plainListContainsFieldValue` 只接受 `data` 本身是数组。
- 建议修改：
  - 优先选择一个全局一致 contract，不要只为该接口特殊绕过 `response.WithData`。
  - 如果项目约定“列表型非分页数据统一放在 `data.result`”，则修改 `tests/admin_test/system_routes_test.go:386` 到 `tests/admin_test/system_routes_test.go:400`，让 helper 支持 `data.result`。
  - 如果前端需要 `data.options` 这类稳定 object，建议把 `internal/controller/admin_v1/auth_sys_dict.go:145` 改为返回 object，例如 `api.Success(c, gin.H{"options": options})`，并同步更新测试、接口文档和前端调用。
  - 不建议修改 `internal/pkg/response/response.go:108` 到 `internal/pkg/response/response.go:119` 的全局行为；这会影响所有非 object 响应，范围过大。
- 验证命令：
  - `go test ./tests/admin_test -run TestSystemDictWriteFlow -count=1`
  - `go test ./...`

## P1：高优先级风险

### P1-01 敏感系统参数在 detail/value/audit diff/request log 中可能泄露

- 相关代码：
  - `internal/service/sys_config/sys_config.go:30` 到 `internal/service/sys_config/sys_config.go:41`：仅 list 调用了 `maskSensitiveValues`。
  - `internal/service/sys_config/sys_config.go:44` 到 `internal/service/sys_config/sys_config.go:55`：detail 返回原始 `ConfigValue`。
  - `internal/service/sys_config/sys_config.go:73` 到 `internal/service/sys_config/sys_config.go:87`：value 接口从缓存返回原始值。
  - `internal/service/sys_config/sys_config.go:197` 到 `internal/service/sys_config/sys_config.go:206`：脱敏逻辑只在 list 被使用。
  - `internal/service/sys_config/audit_diff.go:15` 到 `internal/service/sys_config/audit_diff.go:18`：`config_value` 被纳入 diff。
  - `internal/service/sys_config/audit_diff.go:136` 到 `internal/service/sys_config/audit_diff.go:154`：snapshot 使用原始 `config.ConfigValue`。
  - `internal/resources/request_log.go:85` 到 `internal/resources/request_log.go:101`：request log detail 暴露 `ChangeDiff`、`RequestBody`、`ResponseBody`。
- 根因：系统参数 list 做了展示脱敏，但 detail、value、审计 diff 和审计日志详情没有沿用同一安全策略；`config_value` 进入 `SetAuditChangeDiffRaw` 后会被 request log 详情读出。
- 建议修改：
  - 在 `SysConfigService.Detail` 返回前，如果 `IsSensitive == 1`，将 `ConfigValue` 替换为 `maskedConfigValue`。
  - 对 `/system/config/value` 增加敏感 key 访问策略：默认拒绝敏感参数读取，或只允许后端内部调用，不直接暴露给管理端接口。
  - 在 `snapshotConfig` 中对 `IsSensitive == 1` 的 `config_value` 使用脱敏值，避免 change diff 落库泄露。
  - 如果确实存在“查看真实值”业务需求，应新建单独 reveal 接口，加入二次确认、权限点和审计记录，而不是复用 detail/value。
- 验证命令：
  - `go test ./internal/service/sys_config -count=1`
  - 增加并运行系统参数敏感值 detail/value/audit diff 的测试。

### P1-02 系统内置参数可被修改稳定字段，可能破坏运行时配置

- 相关代码：
  - `internal/service/sys_config/sys_config.go:109` 到 `internal/service/sys_config/sys_config.go:160`
  - `internal/service/sys_config/sys_config.go:137` 到 `internal/service/sys_config/sys_config.go:145`
  - `internal/service/sys_config/audit_diff.go:96` 到 `internal/service/sys_config/audit_diff.go:118`
- 根因：删除逻辑对 system/protected 配置做了保护，但 update 时仍允许修改 `ConfigKey`、`ValueType`、`GroupCode`、`IsSensitive` 等稳定字段。对 `auth.login_max_failures`、`audit.sensitive_fields` 这类运行时 key 来说，误改 key 或类型会让已有业务读不到配置。
- 建议修改：
  - 在 `applyMutation` 读取已有配置后，如果 `config.IsProtected()` 且 `id > 0`，禁止修改 `ConfigKey`、`ValueType`、`GroupCode`。
  - `IsSensitive` 是否允许修改需要明确策略；安全默认值是 protected 配置禁止降级敏感标记。
  - 保留允许修改的字段：`ConfigValue`、`Status`、`Sort`、`Remark`、`ConfigNameI18n`，前提是 value type 校验通过。
- 验证命令：
  - 增加测试：更新内置参数值成功，重命名内置 key 失败，修改内置 value type 失败。
  - `go test ./internal/service/sys_config -count=1`

### P1-03 审计队列不可用时会直接丢弃审计日志

- 相关代码：
  - `internal/middleware/audit_queue.go:61` 到 `internal/middleware/audit_queue.go:99`
  - `internal/middleware/audit_queue.go:79` 到 `internal/middleware/audit_queue.go:88`
- 根因：当 `queue.enable=true` 且 publisher 不可用时，代码只记录一次 warn 后 `return`，没有降级调用 `persistAuditLogFn`。这会导致开启队列但 Redis/Asynq 暂不可用时，审计日志直接丢失。
- 建议修改：
  - 在 `queue.ErrPublisherUnavailable` 和普通 enqueue error 分支中，降级调用同步持久化：`persistAuditLogFn(context.Background(), kind, snapshot)`。
  - 如果产品定义“审计必须强一致”，则请求应返回错误；当前实现既不阻塞请求也不落库，是最危险的中间状态。
  - 为降级路径补充一次性告警和指标，便于发现队列不可用。
- 验证命令：
  - 增加 middleware 单测：queue enabled + publisher unavailable 时仍写入审计或返回预期错误。
  - `go test ./internal/middleware -count=1`

### P1-04 任务重试未校验当前任务定义的 `status` 和 `allow_retry`

- 相关代码：
  - `internal/service/taskcenter/action.go:190` 到 `internal/service/taskcenter/action.go:262`
  - `internal/model/task_center.go` 中 `TaskDefinition` 的 `Status`、`AllowRetry` 字段。
- 根因：`RetryTask` 只校验原 run 是否 failed、task code 是否为空，然后直接复用旧 run 的信息发布新任务；没有读取当前 `task_definitions`。如果任务已停用或当前定义不允许重试，仍然可以重试历史失败任务。
- 建议修改：
  - 在 `RetryTask` 中通过 `runModel.TaskCode` 加载当前 `TaskDefinition`。
  - 校验 `definition.Status == model.TaskStatusEnabled`。
  - 校验 `definition.AllowRetry == 1`。
  - 校验 `definition.Kind == runModel.Kind`，避免历史数据和当前定义类型漂移。
- 验证命令：
  - 增加测试：failed run 可重试；disabled definition 不可重试；`allow_retry=0` 不可重试。
  - `go test ./internal/service/taskcenter -count=1`

### P1-05 高风险手动任务缺少二次确认或原因字段

- 相关代码：
  - `internal/service/taskcenter/action.go:43` 到 `internal/service/taskcenter/action.go:72`
  - `internal/validator/form/task_center.go` 中 `TaskTriggerForm`
  - `internal/cron/registry.go` 中 `IsHighRisk` 定义与同步。
- 根因：任务定义已有 `IsHighRisk` 字段，cron 启动时也会 warn，但手动触发只校验 `status` 和 `allow_manual`，没有对高风险任务做额外确认、原因记录或更强审计。
- 建议修改：
  - 在 `TaskTriggerForm` 增加 `Confirm` 或 `ConfirmText` 字段。
  - 当 `definition.IsHighRisk == model.TaskHighRisk` 时，要求确认字段满足约定。
  - 将触发原因、确认信息写入 audit diff 或 task event meta。
- 验证命令：
  - 增加测试：普通任务无需确认，高风险任务无确认失败，有确认成功。
  - `go test ./internal/service/taskcenter -count=1`

## P2：中优先级优化

### P2-01 `TaskRunRecorder.Enqueue` 创建 run 与 event 未放在同一事务

- 相关代码：
  - `internal/service/taskcenter/recorder.go:186` 到 `internal/service/taskcenter/recorder.go:222`
- 根因：先 `db.Create(run)`，再 `db.Create(event)`；如果 event 写入失败，会留下没有 enqueue event 的 run。`Start` 和 `Finish` 已使用事务，`Enqueue` 应保持同样一致性。
- 建议修改：
  - 使用 `db.Transaction` 包裹 run/event 两次写入。
  - 事务内创建 run 后再创建 enqueue event，失败整体回滚。
- 验证命令：
  - `go test ./internal/service/taskcenter -count=1`

### P2-02 取消任务丢弃操作者信息

- 相关代码：
  - `internal/service/taskcenter/action.go:264` 到 `internal/service/taskcenter/action.go:319`
  - `internal/service/taskcenter/action.go:266` 到 `internal/service/taskcenter/action.go:267`
- 根因：`CancelTask` 接收了 `triggerUserID` 和 `triggerAccount`，但直接 `_ =` 忽略。取消动作虽然会进入 request log，但 task event/run 层面无法追踪是谁取消。
- 建议修改：
  - 在取消成功后写入 task event，例如 `TaskEventCancel`，meta 包含 `trigger_user_id`、`trigger_account`、`cancel_reason`。
  - 或在 run 上增加取消人字段；如果不想改表，优先写 event meta。
- 验证命令：
  - 增加测试：取消成功后存在 cancel event，meta 包含操作者。
  - `go test ./internal/service/taskcenter -count=1`

### P2-03 i18n upsert 语义只能新增/更新，不能删除被移除的 locale

- 相关代码：
  - `internal/model/sys_i18n.go:56` 到 `internal/model/sys_i18n.go:87`
  - `internal/model/sys_i18n.go:89` 到 `internal/model/sys_i18n.go:120`
  - `internal/model/sys_i18n.go:122` 到 `internal/model/sys_i18n.go:153`
  - `internal/service/sys_config/sys_config.go:109` 到 `internal/service/sys_config/sys_config.go:160`
  - `internal/service/sys_dict/sys_dict.go` 的 type/item mutation 流程。
- 根因：`UpsertConfigNames`、`UpsertTypeNames`、`UpsertLabels` 只 upsert 请求中出现的 locale；如果编辑表单去掉某个 locale，旧 locale 仍保留。当前行为更像 PATCH，但后台编辑表单通常更像 replace。
- 建议修改：
  - 明确接口语义：如果是 PATCH，文档和测试应明确“未传 locale 保留旧值”。
  - 如果是完整编辑，upsert 后删除同一实体下不在请求 map 中的 locale。
  - 当前已有测试若依赖“未传保留”，则保留 PATCH 语义，并在接口文档写明，避免前端误解。
- 验证命令：
  - `go test ./internal/service/sys_config ./internal/service/sys_dict -count=1`

### P2-04 任务定义 DB 与 scheduler source-of-truth 分裂

- 相关代码：
  - `cmd/cron/tasks.go:15` 到 `cmd/cron/tasks.go:39`
  - `internal/cron/registry.go:120` 到 `internal/cron/registry.go:165`
- 根因：启动时会将内置任务 upsert 到 `task_definitions`，但 scheduler 实际仍从 `BuiltinTaskDefinitions(cfg)` 读取，而不是从 DB 读取。若后续任务中心允许编辑任务状态、cron spec、是否高风险等字段，scheduler 不会尊重 DB 修改；同时 `upsertTaskDefinition` 每次启动会覆盖 `status`、`allow_manual`、`allow_retry`、`remark` 等字段。
- 建议修改：
  - 如果 `task_definitions` 只是只读展示镜像，应在接口和 UI 中禁止编辑这些字段。
  - 如果任务中心要成为管理入口，应让 scheduler 从 DB 读取 enabled cron task，并把内置同步策略改为“只初始化缺失记录”或只覆盖不可编辑字段。
- 验证命令：
  - 增加测试：DB 中禁用 cron task 后 scheduler 不注册该任务，或明确只读策略下不提供禁用入口。

### P2-05 角色审计 diff 中状态 label 存在重复/陈旧值

- 相关代码：
  - `internal/service/role/audit_diff.go:18` 到 `internal/service/role/audit_diff.go:25`
- 根因：`ValueLabels` 中同时存在 `0 -> 禁用` 和 `2 -> 禁用`。如果角色状态只有 `0/1`，`2` 是陈旧枚举；如果确实存在 `2`，名称也应和业务含义区分。
- 建议修改：
  - 对齐角色状态模型和表单校验，只保留有效枚举。
  - 如果 `2` 已废弃，从 `roleDiffRules` 删除。
- 验证命令：
  - `go test ./internal/service/role -count=1`

## P3：项目级优化建议

### P3-01 固化响应结构约定

- 相关代码：
  - `internal/pkg/response/response.go:108` 到 `internal/pkg/response/response.go:119`
  - `tests/admin_test/system_routes_test.go:386` 到 `tests/admin_test/system_routes_test.go:400`
- 问题：非 object 响应统一包装为 `data.result`，但部分测试和潜在前端调用可能按 `data` 是数组来理解。
- 建议：新增测试 helper 或文档明确 response contract；新增接口优先返回 object，例如 `data.items`、`data.options`、`data.detail`，避免裸 slice/string 引发歧义。

### P3-02 增加路由与默认菜单 API 绑定一致性测试

- 相关代码：
  - `internal/routers/admin_router.go`
  - `internal/service/access/menu_api_defaults.go`
  - `data/migrations/20260425000002_init_data.up.sql`
- 问题：新增路由和菜单 API 默认绑定目前靠人工维护，后续容易漏绑定导致“接口存在但角色无权限”。
- 建议：新增测试扫描新路由组，校验关键管理端路由存在默认绑定；至少覆盖 `system/config`、`system/dict`、`task`、`log/request-log`。

### P3-03 为安全能力补齐专项测试

- 相关代码：
  - `internal/service/sys_config/`
  - `internal/service/audit/request_log_manage.go`
  - `internal/pkg/utils/sensitive/fields.go`
- 问题：敏感配置、审计 diff、request log 展示之间是高风险链路，当前缺少端到端断言。
- 建议：增加测试覆盖“敏感参数创建/更新后，list/detail/audit/request-log 都不泄露原值”。

### P3-04 明确任务定义的管理边界

- 相关代码：
  - `internal/cron/registry.go:120` 到 `internal/cron/registry.go:165`
  - `cmd/cron/tasks.go:15` 到 `cmd/cron/tasks.go:39`
  - `internal/service/taskcenter/action.go`
- 问题：任务中心已具备管理入口的形态，但调度端仍按代码内置定义运行。
- 建议：在文档中明确 `task_definitions` 是只读镜像还是调度配置源；如果是配置源，需要调整 scheduler 读取 DB。

## 建议修改顺序

1. 修复 `TestSystemDictWriteFlow` 中 options 响应 contract 问题，恢复 `go test ./...` 基线。
2. 修复系统参数敏感值泄露链路：detail、value、audit diff、request log。
3. 修复审计 queue 不可用时丢日志问题。
4. 补齐任务中心 retry/high-risk/cancel actor 语义。
5. 统一 i18n update 语义和 task definition source-of-truth。
6. 补充路由权限绑定、敏感值、任务中心专项测试。

## 推荐验证命令

```bash
go test ./tests/admin_test -run TestSystemDictWriteFlow -count=1
go test ./internal/service/sys_config ./internal/service/sys_dict ./internal/service/taskcenter ./internal/middleware -count=1
go test ./...
go run main.go -c ./config.yaml command migrate check
```

## 未验证与遗留风险

- 本次只做审查和文档沉淀，未修改业务代码，因此 `go test ./...` 中已发现的失败仍然存在。
- 未运行前端或真实管理端页面，前端兼容性结论仅基于后端 contract 推断。
- 工作区存在大量未提交改动，本报告按当前文件内容审查；若后续代码继续变动，需重新跑测试并复核高风险链路。
