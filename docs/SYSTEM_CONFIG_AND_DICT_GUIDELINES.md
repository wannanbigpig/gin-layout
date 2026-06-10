# 系统配置与系统字典后端接入规范

本文档从后端视角约定 `sys_config` 与 `sys_dict` 的使用边界。核心目标是避免把权限、状态机、数据结构、安全边界等后端核心规则做成后台可随意修改的配置，同时为运行时策略和展示型枚举提供清晰、可审计、可测试的接入方式。

## 总体原则

- `sys_config` 管运行时策略，后端读取后必须有类型约束、默认值和降级行为。
- `sys_dict` 管展示选项，后端最多用于返回 label、tag、颜色等展示数据。
- model 常量、validator、service 状态机和 migration 管核心规则。

也就是说，后台可以调整“某些策略阈值”和“怎么展示”，但不能绕过代码里的权限判断、状态流转、字段含义、审计语义和安全约束。

## `sys_config` 适用场景

适合由后端接入 `sys_config` 的内容：

- 运行期可调的开关，例如某个低风险功能是否开启。
- 运维阈值，例如登录失败锁定次数、锁定时长、日志保留天数。
- 安全策略配置，例如 request log 脱敏字段。
- 对业务行为有影响，但允许在不发版的情况下调整，且能接受明确降级策略的参数。

新增系统配置时必须同时补齐：

- 配置 key 常量，禁止在业务代码中散落字符串。
- value type、默认值和允许范围。
- 领域化读取 helper，例如 `BoolValue`、`IntValue` 或专门 service 方法。
- 读取失败、配置缺失、类型错误时的降级行为。
- 是否敏感的安全标记。
- 缓存刷新、启动预热或运行时 reload 策略。
- 初始化 migration 数据。
- 单元测试或关键链路测试。

后端接入 `sys_config` 时必须保证：

- service 层仍保留业务兜底，不能把配置值直接当作可信输入。
- validator 仍负责接口入参合法性，不能依赖配置表替代校验。
- 敏感配置不能通过 detail、value、request log、change diff 等通用链路泄露原值。
- protected 配置禁止修改稳定字段，例如 key、value type、group code。
- 配置变更影响运行态时，必须明确是否需要刷新缓存或重启进程。

## `sys_config` 不适用场景

以下内容不建议放入 `sys_config`：

- 权限判断规则。
- 路由定义。
- 数据库结构含义。
- 核心状态机流转规则。
- 任务调度器的 handler/spec 安全边界；定时任务启停这类运营状态应使用 `task_definitions`。
- 会导致“后台一改，系统核心行为立即改变且难以审计”的规则。

这类规则应继续由代码常量、validator、service 状态机、数据库约束和 migration 负责。

## `sys_dict` 适用场景

适合由后端通过 `sys_dict` 提供的内容：

- 对外展示层下拉选项。
- 列表 tag 文案、颜色、tag type。
- 多语言展示 label。
- 变化主要影响展示，不改变后端业务规则的枚举。

当前推荐接入的展示型字典包括：

- `common_status`：通用启用/禁用展示。
- `yes_no`：是否展示。
- `menu_type`：菜单类型展示。
- `api_auth_mode`：接口鉴权模式展示。
- `http_method`：HTTP 方法展示。
- `task_kind`：任务类型展示。
- `task_source`：任务来源展示。
- `task_run_status`：任务执行状态展示。

后端提供字典 options 时只承担展示数据输出职责。即使字典项被禁用、删除或误改，后端核心接口仍必须由 validator、model 常量和 service 逻辑保证合法性。

## `sys_dict` 不适用场景

以下内容不建议把 `sys_dict` 作为后端唯一判断来源：

- validator 合法值。
- 任务执行状态流转。
- 角色、菜单、用户状态的后端启停判断。
- 权限鉴权模式的实际执行逻辑。
- 审计 diff 的核心字段判断。

后端可以复用字典做展示，但不能因为字典被误改就放宽核心业务规则。例如 `status=2` 不能因为字典中新增了一个选项就自动变成合法状态。

## 后端实现要求

- 新增配置或字典默认数据优先放初始化 migration。
- 未发布开发阶段不额外加常驻同步逻辑，避免把 seed 数据做成启动副作用。
- 已发布或共享环境需要演进时，通过新增 migration 或明确的幂等同步入口处理。
- 配置读取应集中在 service/helper，controller 不直接拼装业务规则。
- 核心枚举必须在 form validator 和 model 常量中显式表达。
- 审计 diff 可以使用 label 映射提升可读性，但 diff 字段集合和安全脱敏规则必须由代码控制。
- 敏感配置详情和列表只返回脱敏占位符；更新时收到未改动的脱敏占位符应保留原值，不能把占位符写成真实配置值。
- 菜单标题 i18n 更新采用 Replace 语义：同一 `menu_id` 下，本次请求未提交的 locale 会被删除，仅保留本次提交的 locale。
- 系统配置名称、字典类型名称、字典项 label 的 i18n 更新采用 PATCH 语义：只新增/更新请求中出现的 locale，未传 locale 保留旧值。
- 接口文档必须显式说明当前实体使用的是 Replace 还是 PATCH，不能笼统写成“支持 i18n 更新”。

## i18n 更新语义差异

为了避免调用方误删多语言，后端把不同实体的 i18n 更新分成两类语义。下表是当前代码事实，对应实现位于 `internal/model/menu_i18n.go` 与 `internal/model/sys_i18n.go`，相关单元测试见 `internal/model/i18n_test.go`。

| 实体表 | 更新方法 | 语义 | 行为 |
| --- | --- | --- | --- |
| `menu_i18n` | `MenuI18n.UpsertMenuTitles` | Replace | 同一 `menu_id` 下，本次请求未提交的 locale 行会在事务内被删除，仅保留本次提交的 locale。 |
| `sys_config_i18n` | `SysConfigI18n.UpsertConfigNames` | PATCH | 仅 `ON CONFLICT DO UPDATE` 提交的 `(config_id, locale)`，未提交的 locale 行保持不变。 |
| `sys_dict_type_i18n` | `SysDictTypeI18n.UpsertTypeNames` | PATCH | 同上，仅更新提交的 `(dict_type_id, locale)`。 |
| `sys_dict_item_i18n` | `SysDictItemI18n.UpsertLabels` | PATCH | 同上，仅更新提交的 `(dict_item_id, locale)`。 |

四个方法在写入前都会执行 `strings.TrimSpace`：

- 如果 `locale` 或对应文本（`title` / `config_name` / `type_name` / `label`）经 TrimSpace 后为空字符串，该行会被静默跳过，既不会被插入，也不会用空字符串覆盖已有行。
- 整批 locale 全部为空时，方法早返回，对数据库零写入；菜单 Replace 语义下也不会因为传入空 map 而误删现有数据。

调用方接入建议：

- 菜单编辑表单必须回填完整的多语言。前端提交时未带的 locale 等价于"删除"，需要在 UI 层显式提示用户当前已有哪些语言、本次会保留哪些。
- 系统配置 / 字典类型 / 字典项编辑表单可以做"按需更新某个 locale"，不需要每次提交全量多语言；想清空某个 locale 应该走 `DeleteByXxxIDs` 这样的删除入口，而不是传空字符串。
- 服务端如果需要在 PATCH 语义之上对齐 Replace 语义，应该在业务层显式调用对应模型的 `DeleteByXxxIDs` 后再 Upsert，禁止把责任下放给前端。

接口测试与调用方验收时，至少要覆盖这三条事实：

- 菜单更新时省略某个 locale，后续 detail / 数据库中该 locale 应消失。
- 系统配置、字典类型、字典项更新时省略某个 locale，后续 detail / 数据库中该 locale 必须保留旧值。
- 传空字符串不是“清空翻译”，而是“本行跳过不写”；想删除 locale 只能走显式删除链路。

## 对外展示接口边界

- `dict/options` 只输出展示选项，不输出后端业务规则。
- `status`、`is_*`、任务状态等字段可以通过字典返回展示 label，但合法值仍由后端 validator 和 model 常量决定。
- 新增字典 type code 时，后端必须同步默认数据、接口文档和接口测试；具体展示层 fallback 由调用方项目文档维护。
- 字典接口失败、字典项误改或字典项缺失时，后端核心业务语义不能改变。

## 决策表

| 需求 | 建议 |
| --- | --- |
| 页面下拉、tag、颜色、多语言 label | 放 `sys_dict` |
| 登录锁定次数、审计脱敏字段、可运营阈值 | 放 `sys_config` |
| validator 枚举、权限模式、状态机流转 | 放代码常量与校验 |
| 定时任务是否真实启停 | 放 `task_definitions.status`，`cron` 每次触发前读取；handler/spec 仍以代码内置定义为准 |
| 后端需要运行时读取策略 | 通过领域化 helper 读取 `sys_config`，并保留默认值 |
| 展示层需要减少硬编码枚举 | 后端提供 `dict/options`；调用方 fallback 由调用方项目维护 |
