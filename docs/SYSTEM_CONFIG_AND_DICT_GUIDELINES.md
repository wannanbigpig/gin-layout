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
- 任务调度器的真实 source-of-truth，除非已专门改造 scheduler 从 DB 读取。
- 会导致“后台一改，系统核心行为立即改变且难以审计”的规则。

这类规则应继续由代码常量、validator、service 状态机、数据库约束和 migration 负责。

## `sys_dict` 适用场景

适合由后端通过 `sys_dict` 提供的内容：

- 前端下拉选项。
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
- 接口文档必须说明配置/字典字段的业务语义、枚举含义和 PATCH/replace 更新语义。

## 前端协作约束

- 列表筛选、表格 tag、普通 select 可以优先使用 `dict/options`。
- 页面必须提供本地 fallback，保持离线或字典接口失败时的可用性。
- 对 `status`、`is_*` 这类字段，前端可用字典展示；后端仍以 validator 和 model 常量约束。
- 前端新增依赖字典 type code 时，应同步后端默认数据、接口文档和 fallback 约定。

## 决策表

| 需求 | 建议 |
| --- | --- |
| 页面下拉、tag、颜色、多语言 label | 放 `sys_dict` |
| 登录锁定次数、审计脱敏字段、可运营阈值 | 放 `sys_config` |
| validator 枚举、权限模式、状态机流转 | 放代码常量与校验 |
| 定时任务是否真实启停 | 先明确 scheduler source-of-truth，再决定是否进 DB 配置 |
| 后端需要运行时读取策略 | 通过领域化 helper 读取 `sys_config`，并保留默认值 |
| 前端想减少硬编码枚举 | 接 `dict/options`，同时保留 fallback |
