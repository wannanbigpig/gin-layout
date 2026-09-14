# MEMORY

这份文档写给接手 `gin-layout` 的 AI Agent。它不是项目营销介绍，而是为了让下一位 AI 尽快知道：

- 项目当前真实状态是什么
- 主要入口、核心链路和高风险区域在哪
- 关键约束和禁止事项是什么
- 写代码时如何保持当前项目风格

## 0. 快速结论

先记住这些，不确定时再往下查细节：

- 本文已于 `2026-09-12` 按当前工作区（`HEAD=115db37`，包含未提交修改）复核；以后接手仍需先执行 `git status --short`，不能把本文当成冻结快照。
- 代码是真相，文档只是辅助；文档和代码冲突时，以代码为准并顺手修正文档。
- 新功能优先沿用现有 `service / model / resources / validator` 分层。
- 保持现有工程范式，不引入重框架化、函数式大重写、新 DI 容器或 repository 全量替换。
- Casbin 使用 `github.com/casbin/casbin/v3`。
- 鉴权链路采用 claims-first：请求上下文保存 `AuthPrincipal`，不是完整数据库 `AdminUser`。
- 错误码和 message key 会按请求语言国际化；显式传入 `NewBusinessError(..., message)` 或 `api.Fail(..., message)` 的文本会原样返回，当前仍有这类历史调用。
- 请求审计是“审计快照 + 采样 + 队列优先 + 异步缓冲降级”：队列关闭或投递失败时进入内存缓冲批量落库，仅在缓冲满或批量写失败时逐条同步兜底。
- CORS 各字段的 `["*"]` / 空数组语义不同，不能笼统视为同一种通配行为；详见 3.4。
- 菜单列表/树只返回当前语言 `title`；菜单详情返回 `title_i18n`，不返回 `title`。
- 路由树是 Gin 注册和 API 权限元数据的共同来源；权限关系以数据库为真相，Casbin 做最终接口判定。
- 当前已包含双 Token 会话、任务中心、系统配置/字典、系统通知与 WebSocket、存储驱动、配置热更新、健康检查、限流和安全响应头等能力，不能只按早期 CRUD 脚手架理解。
- 继续改代码时保持小步修改、局部重组、命名稳定。
- 改完代码必须自己验证；新增或修改 HTTP 行为默认要补 `tests/` 接口级测试。

## 1. 禁止事项

当前代码库禁止这样做：

- 引入 `casbin/v2` 或混用不同主版本的 Casbin adapter。
- 在认证中间件里每个请求都回表查询完整 `AdminUser`；账号状态变更应通过服务层撤销 token 或更新会话状态生效。
- 把 `AuthPrincipal.AdminUser()` 当成数据库实时状态。
- 把同步 INSERT 重新变成审计日志的常规主路径；当前只允许在降级缓冲满或批量写失败时同步兜底。
- 把请求日志设计成“每个请求同步写文件 + 异步审计”的双写链路。
- 把 `cors_origins` 空数组当成允许跨域；该字段只有显式配置来源或 `["*"]` 才放行。其他 CORS 字段的空数组有各自默认值，不能套用这条规则。
- 让菜单列表或用户菜单树返回 `title_i18n`。
- 让菜单详情返回 `title`。
- 让菜单写接口接收 `title`；写接口使用 `title_i18n`。
- 把业务逻辑、事务或批量数据修复塞进 controller。
- 把所有逻辑塞回单个超大 service 文件。
- 绕过 `PermissionSyncCoordinator` 分散写入 Casbin 权限。
- 在文件上传 complete 阶段信任客户端回传的对象 ID、对象键、哈希、大小、MIME、bucket 或可见性；这些值必须由服务端 upload session / `complete_token` 约束。
- 把同一个周期任务同时注册到 `cron` 和 worker fallback，造成重复执行。
- 只改代码不跑测试，或把验证责任留给用户或下一个 AI。

## 2. 当前定位

项目名：`gin-layout`

- Go module：`github.com/wannanbigpig/gin-layout`
- 当前 `go.mod` 语言版本：`go 1.27`

这是一个偏后台管理场景的 Go 后端骨架，内置：

- JWT Access Token + Refresh Token Cookie 登录、校验、刷新、登出、会话撤销与强踢
- Casbin RBAC、数据权限、声明式路由和 API 权限元数据同步
- 管理员、角色、部门、菜单、API 权限体系
- 请求日志、登录日志、审计差异、请求统计与导出中心
- 本地 / 阿里云 OSS 存储、公开/私有文件访问、直传/分片上传、文件夹与引用管理
- 系统配置、系统字典、任务中心、系统通知、WebSocket 实时推送和仪表盘
- CLI 命令、迁移、代码/OpenAPI/错误码生成、定时任务、异步队列 worker
- 配置热更新、readiness、限流、安全响应头和依赖降级启动控制

运行形态包含三类进程：

1. `service`
   - 提供 HTTP API
   - 由中间件构建请求审计日志快照并记录请求统计
   - 队列开启且发布成功时把审计快照投递到 Asynq
   - 队列关闭或发布失败时写入进程内有界缓冲，由后台 goroutine 批量落库
   - 降级缓冲满或批量落库失败时才逐条同步兜底；存储不可用时带冷却熔断
   - 停服时等待审计降级缓冲、登录日志、实时订阅和配置订阅在各自超时内收尾

2. `worker`
   - 消费异步任务
   - 固定注册请求审计日志、请求日志导出和文件列表导出任务
   - `queue.consume_cron_fallback=true` 时还会注册 cron fallback handler

3. `cron`
   - 负责周期任务调度
   - 以数据库 `task_definitions` 的运营配置为运行时真相，并通过 Redis 通知和周期 reconcile 刷新进程缓存
   - 高风险任务在运行缓存异常时 fail-closed

## 3. 当前事实

这些是当前实现与开发规范。

### 3.1 Casbin v3

权限引擎是：

- `github.com/casbin/casbin/v3`
- `github.com/casbin/gorm-adapter/v3`

相关入口：

- [internal/access/casbin/casbin.go](internal/access/casbin/casbin.go)
- [internal/access/casbin/adapter.go](internal/access/casbin/adapter.go)

### 3.2 Claims-First 鉴权

请求上下文保存 claims 快照，不保存完整数据库 `AdminUser` 模型：

- [internal/service/auth/principal.go](internal/service/auth/principal.go)
- [internal/service/auth/login_token_ops.go](internal/service/auth/login_token_ops.go)
- [internal/middleware/parse_token.go](internal/middleware/parse_token.go)
- [internal/middleware/admin_auth.go](internal/middleware/admin_auth.go)

这意味着：

- 中间件不应为每个请求回表查用户。
- 需要用户实时数据库状态的地方，由具体业务接口显式查库。
- `AuthPrincipal.AdminUser()` 是轻量投影，不代表数据库最新状态。
- 每次解析访问令牌仍会检查 Redis 黑名单；MySQL 可用时还会查询登录日志撤销状态。这是 token/session 校验，不是回查完整 `AdminUser`。

### 3.3 请求日志

请求日志链路以审计日志为核心：

- 请求链路：缓存请求体、录制响应体，构建审计快照。
- 请求统计：在审计日志跳过/采样之前同步累计每日请求统计。
- 低风险只读成功请求：默认按 `1/N` 采样，慢请求、失败请求、高风险请求和 WebSocket 连接不参与该采样。
- 队列开启且投递成功：由 worker 异步落库。
- 队列关闭或投递失败：写入进程内有界缓冲，由后台 worker 批量异步落库。
- 缓冲满且没有可淘汰的低风险条目，或批量写失败：逐条同步 INSERT 兜底。
- 数据库存储不可用：进入冷却熔断，冷却后放行一次半开探测；进程退出时在超时内 drain 降级缓冲。

全局 zap 文件日志用于系统日志、错误日志和 panic 日志；每个请求日志按审计快照链路处理。

关键文件：

- [internal/middleware/logger.go](internal/middleware/logger.go)
- [internal/middleware/audit_queue.go](internal/middleware/audit_queue.go)
- [internal/middleware/audit_fallback_buffer.go](internal/middleware/audit_fallback_buffer.go)
- [internal/jobs/audit_log.go](internal/jobs/audit_log.go)
- [cmd/service/service.go](cmd/service/service.go)
- [cmd/worker/worker.go](cmd/worker/worker.go)

### 3.4 CORS

四个 CORS 列表字段的语义并不相同：

- `cors_origins: ["*"]`：`cors_credentials=false` 时返回字面 `Access-Control-Allow-Origin: *`；与 credentials 同时启用会在启动校验时报配置冲突。
- `cors_methods: ["*"]` 或空数组：展开为项目内置方法集合 `GET/POST/PUT/PATCH/DELETE/HEAD/OPTIONS`。
- `cors_headers: ["*"]` 或空数组：展开为项目固定安全头集合，不反射任意 `Access-Control-Request-Headers`。
- `cors_expose_headers: ["*"]` 或空数组：返回字面 `Access-Control-Expose-Headers: *`。
- `cors_origins` 空数组：不放行带 `Origin` 的跨域请求。

实现位置：

- [internal/middleware/cors.go](internal/middleware/cors.go)
- [internal/middleware/cors_test.go](internal/middleware/cors_test.go)
- [config/config.yaml.example](config/config.yaml.example)

### 3.5 错误国际化

错误文案由请求语言驱动：

- `RequestLocaleHandler` 从 `Accept-Language` 解析语言并写入请求上下文。
- 响应层根据上下文语言选择错误码或 message key 的文案语言。
- 通用业务错误未显式传 message 时走错误码文案表。
- 需要细粒度错误文案时，优先使用 `message key`，由响应层统一国际化。
- 参数校验错误由 validator translator 输出多语言文案，字段名优先使用 `label/json/form` 标签。
- 显式传入 `NewBusinessError(code, message)` 或 `api.Fail(code, message)` 的文本不会再次国际化；当前文件上传、任务中心、会话等代码仍有历史显式中英文消息，不能声称所有错误都已国际化。

关键文件：

- [internal/middleware/request_locale.go](internal/middleware/request_locale.go)
- [internal/pkg/i18n/locale.go](internal/pkg/i18n/locale.go)
- [internal/pkg/errors/error.go](internal/pkg/errors/error.go)
- [internal/pkg/errors/zh-cn.go](internal/pkg/errors/zh-cn.go)
- [internal/pkg/errors/en-us.go](internal/pkg/errors/en-us.go)
- [internal/pkg/response/response.go](internal/pkg/response/response.go)
- [internal/validator/runtime.go](internal/validator/runtime.go)
- [internal/validator/translation.go](internal/validator/translation.go)
- [internal/validator/binding.go](internal/validator/binding.go)

### 3.6 菜单 I18n

菜单标题由请求语言驱动：

- 列表/树：只返回当前语言 `title`，不返回 `title_i18n`。
- 详情：返回 `title_i18n` 供编辑回填，不返回 `title`。
- 写接口：使用 `title_i18n`，支持 `zh-CN` 与 `en-US`，至少一种非空。

关键文件：

- [internal/controller/admin_v1/auth_menu.go](internal/controller/admin_v1/auth_menu.go)
- [internal/service/menu/menu_query.go](internal/service/menu/menu_query.go)
- [internal/service/menu/menu_edit.go](internal/service/menu/menu_edit.go)
- [internal/resources/menu.go](internal/resources/menu.go)
- [tests/admin_test/menu_test.go](tests/admin_test/menu_test.go)

### 3.7 文件组织

典型职责包：

- `internal/service/admin/`
- `internal/service/role/`
- `internal/service/dept/`
- `internal/service/access/`
- `internal/service/auth/`
- `internal/service/audit/`
- `internal/service/menu/`
- `internal/service/taskcenter/`
- `internal/service/sys_config/`
- `internal/service/sys_dict/`
- `internal/service/notification/`
- `internal/service/dashboard/`
- `internal/filestorage/`
- `internal/cron/`
- `internal/jobs/`
- `internal/queue/`
- `internal/validator/`
- `internal/model/`

继续改时，优先往对应职责文件里放；可以同包拆文件，保持文件职责清晰。

## 4. 任务阅读索引

新会话建议先看：

1. [AGENTS.md](AGENTS.md)
2. [README.md](README.md)
3. [docs/COMMANDS_AND_TASKS.md](docs/COMMANDS_AND_TASKS.md)
4. [cmd/root.go](cmd/root.go)
5. [cmd/service/service.go](cmd/service/service.go)
6. [internal/routers](internal/routers)

按任务追加阅读：

- 新增或修改后台接口：
  - [internal/routers](internal/routers)
  - [internal/controller/admin_v1](internal/controller/admin_v1)
  - [internal/validator/form](internal/validator/form)
  - [internal/service](internal/service)
  - [internal/resources](internal/resources)
  - [tests/admin_test](tests/admin_test)

- 权限 / Casbin / 菜单可见性：
  - [internal/access/casbin](internal/access/casbin)
  - [internal/service/access](internal/service/access)
  - [internal/routers/admin_router.go](internal/routers/admin_router.go)
  - [rbac_model.conf](rbac_model.conf)

- 鉴权 / token / 当前用户：
  - [internal/middleware/parse_token.go](internal/middleware/parse_token.go)
  - [internal/middleware/admin_auth.go](internal/middleware/admin_auth.go)
  - [internal/service/auth](internal/service/auth)

- 请求日志 / 审计 / 队列：
  - [internal/middleware/logger.go](internal/middleware/logger.go)
  - [internal/middleware/audit_queue.go](internal/middleware/audit_queue.go)
  - [internal/middleware/audit_fallback_buffer.go](internal/middleware/audit_fallback_buffer.go)
  - [internal/jobs/audit_log.go](internal/jobs/audit_log.go)
  - [internal/service/audit](internal/service/audit)
  - [cmd/worker/worker.go](cmd/worker/worker.go)

- 菜单标题国际化：
  - [internal/service/menu](internal/service/menu)
  - [internal/model/menu_i18n.go](internal/model/menu_i18n.go)
  - [internal/resources/menu.go](internal/resources/menu.go)
  - [tests/admin_test/menu_test.go](tests/admin_test/menu_test.go)

- 模型基础能力 / 列表分页 / 删除：
  - [internal/model/base.go](internal/model/base.go)
  - [internal/model/base_crud.go](internal/model/base_crud.go)
  - [internal/model/base_list.go](internal/model/base_list.go)
  - [internal/model/base_tree.go](internal/model/base_tree.go)

- 文件存储 / 上传 / 文件引用：
  - [internal/filestorage](internal/filestorage)
  - [internal/service/file_resource.go](internal/service/file_resource.go)
  - [internal/service/file_resource_folder_upload.go](internal/service/file_resource_folder_upload.go)
  - [internal/service/file_object.go](internal/service/file_object.go)
  - [internal/service/file_reference.go](internal/service/file_reference.go)

- 任务中心 / worker / cron：
  - [internal/service/taskcenter](internal/service/taskcenter)
  - [internal/jobs](internal/jobs)
  - [internal/cron](internal/cron)
  - [cmd/cron](cmd/cron)
  - [cmd/worker](cmd/worker)

- 系统配置、字典和热更新：
  - [internal/service/sys_config](internal/service/sys_config)
  - [internal/service/sys_dict](internal/service/sys_dict)
  - [config/runtime.go](config/runtime.go)

## 5. 目录边界

目录需要按职责边界理解，而不只是物理位置。

### 5.1 `cmd/`

负责程序入口和启动编排，不负责业务逻辑。

关键命令：

- `service`
- `worker`
- `cron`
- `command`

### 5.2 `internal/controller/`

HTTP 控制器层。

职责：

- 收参
- 调 validator/form
- 调 service
- 返回 response

不应该在 controller 里写复杂业务判断、事务或批量数据修复逻辑。

这是目标边界，不是当前代码的绝对现状：`internal/controller/admin_v1/auth_message.go` 仍直接编排 WebSocket ticket 的 Redis 创建/消费、会话校验和升级流程；`auth.go` 也保留登录失败处理与审计差异编排。新增代码不要继续扩大这些 controller 例外，相关重构需保持 HTTP/WebSocket 契约兼容。

### 5.3 `internal/service/`

业务层核心。

职责：

- 业务规则
- 事务编排
- 模型组合查询
- 触发权限同步
- 触发 token 撤销

这个项目的大部分改动都应该优先落在 service 层。

### 5.4 `internal/model/`

GORM 模型层和基础数据访问层。

职责：

- 表结构
- 通用 CRUD
- 列表分页
- 树形节点辅助

复杂业务判断放在 service 层，model 保持数据模型与通用数据访问职责。

### 5.5 `internal/resources/`

响应整形层。

职责：

- 把 model / service 结果转换成前端要的结构。
- 提供统一分页集合、树形转换和少量原始集合包装能力。

大多数接口采用显式 transformer；但当前不是绝对规则：`RoleService.List` 仍通过 `resources.ToRawCollection` 把 `[]*model.Role` 直接包装进分页响应。新增代码不要扩大该例外，修改角色列表契约时应先评估兼容性再收敛。

### 5.6 `internal/validator/` 与 `internal/validator/form/`

参数对象和校验规则层。

职责：

- 定义请求参数 struct。
- 定义 tag 校验规则。
- 统一错误翻译和绑定错误处理。

### 5.7 `internal/service/access/`

权限同步协调层。

职责拆分为：

- `api_cache`
- `scope_resolver`
- `graph_loader`
- `coordinator`
- `menu_api_defaults`
- `menu_api_defaults_external`
- `system_defaults`
- `user_permission_sync`
- `role_menu_assignment`
- `data_scope_filter`
- `transaction`

写入或重建最终 Casbin 权限时，外层统一从 `PermissionSyncCoordinator` 进入；纯查询辅助（例如角色菜单 ID 读取、数据范围解析）可以直接使用职责明确的 access service。

## 6. 开发风格

### 6.1 命名

- service 名称优先以业务对象命名，如 `AdminUserService`、`RoleService`。
- constructor 优先用 `NewXxxService()`。
- 业务入口方法名直接用动词：`List`、`Create`、`Update`、`Delete`、`BindRole`。
- 内部辅助方法用小写，尽量语义直接：`buildListCondition`、`updateDeptRole`。

当前例外：`internal/service/notification` 使用通用名称 `Service` / `NewService()`；调用时依靠包名表达业务语义。

避免引入抽象但缺少项目语境的命名，例如：

- `Processor`
- `Manager`
- `Facade`
- `RepositoryFactory`

只有代码里存在明确同类模式时，才沿用同类命名。

### 6.2 结构

优先做“同包拆文件”，新增 package 前先确认职责边界确实独立。

结构整理方式：

- 保留 `package` 不变。
- 保留原 service 名称不变。
- 把一个大文件拆成多个职责文件。

### 6.3 错误处理

业务错误优先使用：

- `internal/pkg/errors`

典型方式：

- `e.NewBusinessError(...)`
- `e.NewBusinessErrorWithKey(...)`

业务错误码优先复用，保持现有错误码风格。要按请求语言返回的自定义提示应使用 message key；显式 message 是原样响应，当前仍有历史调用，不应误认为会自动翻译。

### 6.4 事务与权限刷新

事务与权限刷新规范：

- service 层负责事务编排。
- 权限相关写入优先复用 `access.RunInTransaction` 和 `PermissionSyncCoordinator`；taskcenter、auth 等独立子系统当前也存在直接 `db.Transaction(...)` 的既有写法。
- 权限或普通缓存刷新通常在事务提交后处理；需要事务内同步数据库权限关系时，提交后再 reload 共享 Casbin Enforcer。
- `PermissionSyncCoordinator.ReloadPolicyCacheWithCode` 的提交后 reload 失败只记录告警并返回 `nil`，避免把已提交写操作误报为失败；这意味着短时间内可能存在内存策略滞后风险。

“事务 + 后置刷新”保持在 service 层编排。

### 6.5 响应

JSON 业务接口通常走统一响应封装，不直接返回裸对象。文件下载/重定向和 WebSocket 升级按协议直接使用 Gin / HTTP writer，不套 JSON envelope。

所以：

- controller 保持现有 response 风格。
- 列表接口尽量返回 collection。
- detail 接口尽量走 transformer。
- 角色列表的 `ToRawCollection` 是当前唯一明确的 model 直出例外，不代表推荐模式。

### 6.6 注释

新增注释优先使用少量中文说明职责和意图；当前存量代码中中英文注释并存。除非必要复杂需求才写长注释。

可以写：

- `// BindRole 绑定角色。`
- `// ReloadPolicyCache 在事务提交后刷新共享 Casbin Enforcer 的内存策略。`

注释保持高信息密度，只解释职责、意图和复杂逻辑。

## 7. 写代码硬规则

### 7.1 先复用，再新增

做任何功能前，先查：

- service 是否存在类似逻辑。
- model 是否存在通用方法。
- validator 是否存在相同校验模式。
- resources 是否存在类似 transformer。

优先复用现有能力。

### 7.2 小步修改

这个项目是工程型仓库，不是实验仓库。

可以：

- 局部重构。
- 同包拆文件。
- 补测试。

保持现有范式，尤其避免突然引入：

- repository 模式全量替换。
- 泛型 service 框架大改。
- 新 DI 容器。
- 新 HTTP 框架。

### 7.3 保持接口签名稳定

如果只是做结构优化或内部修复：

- controller 签名不改。
- service 对外方法名尽量不改。
- HTTP API 不改。
- CLI 命令名不改。

### 7.4 新增行为要配测试

尤其是下面几类：

- 鉴权
- 权限同步
- CORS
- 日志截断
- token 撤销
- 默认值生成
- 事务提交后刷新

如果是新增接口，不仅要补直接相关的测试，还要默认在 `tests/` 目录下补接口级测试用例。

原因：

- 项目有明确的 `tests/admin_test` 集成测试入口。
- 只补 service 或 middleware 级测试，不足以证明接口链路真实可用。
- 新接口至少要验证路由、鉴权、请求参数、响应结构中的关键路径。

默认要求：

- 新增 HTTP 接口：补 `tests/` 目录下的接口测试。
- 修改现有 HTTP 接口行为：补或更新 `tests/` 目录下对应测试。
- 只改内部实现且对外行为不变：可以只补包内测试，但要能说明为什么不需要接口测试。

### 7.5 改完先验证

改完代码后，先自己跑测试，再决定是否结束本轮工作。

最小要求：

- 小改动：跑直接受影响包测试。
- 中等改动：跑相关子系统测试。
- 公共层或高风险改动：优先跑 `go test ./...`。

以下结束方式不符合项目要求：

- 只改代码，不跑测试。
- 只说“理论上没问题”。
- 只让用户自己去验证。

如果因为环境限制无法完成某项测试，结果里必须明确：

- 哪些测试跑了。
- 哪些测试没跑。
- 没跑的原因是什么。

推荐基线：

```bash
go test ./internal/...
go test ./...
go test ./internal/middleware ./internal/service/access ./internal/service/auth
```

## 8. 新增后台接口路径

建议按这个顺序：

1. 在 `internal/validator/form/` 定义参数 struct。
2. 在 `internal/model/` 复用或补充数据访问方法。
3. 在 `internal/service/` 写业务逻辑。
4. 在 `internal/resources/` 补返回结构。
5. 在 `internal/controller/admin_v1/` 加控制器方法。
6. 在 `internal/routers/` 的声明式路由树里注册。
7. 如涉及权限元数据，同步执行 `command api-route`。
8. 如涉及角色/菜单/API 关系变化，确认权限重建链路是否需要触发。
9. 在 `tests/` 目录下补接口级测试。

## 9. 高风险区域

### 9.1 `internal/service/access`

会影响：

- 菜单可见性
- API 权限
- Casbin 最终策略

### 9.2 `internal/service/auth`

风险点：

- 鉴权链路采用 claims-first。
- token 黑名单 / 撤销逻辑集中在认证服务内。
- 改不好会影响所有登录态请求。

### 9.3 `internal/middleware/logger*`

风险点：

- 请求体截断。
- 响应体捕获。
- 请求统计在日志跳过/采样之前同步写入。
- 低风险只读成功请求采样。
- 队列异步投递。
- 队列关闭或投递失败时的有界内存缓冲与批量异步落库。
- 缓冲满/批量失败时的同步兜底、数据库不可用熔断和停服 drain。
- 性能与日志正确性平衡。

### 9.4 `internal/model/base_*`

风险点：

- 很多 model 共用。
- 一旦改坏，会影响列表、分页、删除、防误删逻辑。

### 9.5 文件上传与存储

相关区域：

- `internal/filestorage`
- `internal/service/file_resource*`
- `internal/service/file_object.go`
- `internal/service/file_reference.go`

风险点：

- upload session / `complete_token` 是 complete 阶段服务端元数据真相。
- 物理对象复用必须区分公开对象与私有 owner。
- 本地和 OSS 的访问、删除、分片完成与引用关系必须保持一致。
- 文件夹与文件都采用软删除时，直属/递归删除、恢复和统计语义不能混淆。

### 9.6 任务中心、cron 与 worker

风险点：

- `task_definitions` 的代码定义字段和数据库运营字段边界不同。
- cron 以进程缓存执行，Redis 通知和 reconcile 负责刷新；高风险任务缓存异常时 fail-closed。
- worker 可选消费 cron fallback，同一周期任务不能被 cron 与 fallback 重复执行。
- 任务重试、取消、锁、执行记录和导出记录存在事务与并发边界。

### 9.7 路由、认证模式与缓存

风险点：

- `AdminRouteTree()` 同时驱动 Gin 注册和 API 元数据，不能只改单侧。
- `AuthModeNone`、`AuthModeLogin`、`AuthModeAuth` 语义不同；`AuthModeAuth` 默认需要菜单/API 绑定或显式测试豁免。
- API 认证模式变化后要同步数据库 `api` 表和 Redis 路由缓存，否则 fail-closed 可能表现为无接口操作权限。

## 10. 接手检查命令

先看工作树状态：

```bash
git status --short
```

再看测试基线：

```bash
go test ./...
```

如果只改中间件 / 权限 / 鉴权，优先跑对应子集：

```bash
go test ./internal/middleware ./internal/service/access ./internal/service/auth
```

### 10.1 2026-09-12 验证基线

- `go vet ./...`、`go mod verify` 和 `go run main.go -h` 当前通过。
- 不要用缓存命中代替 fresh test；本次 `go test -count=1 ./...` 未通过。
- 已确认的现有失败是 `TestFilterUserRolesByEnabledSetDropsDisabledRoles`：`filterUserRolesByEnabledSet()` 从 `map` 收集 `allRoleIDs`，测试却固定期待 `[2,4]`，实际可得到 `[4,2]`。这是当前源码/测试的顺序契约不一致，本次仅校正文档，未修改该实现。
- 全量测试会并行运行共享依赖的包；遇到 `tests` 或 `tests/admin_test` 的单次全量失败时，应再单包 fresh 复跑并保留原始失败证据，不能仅凭随后单包通过就宣称全量基线绿色。

## 11. 文档关系

根目录文档建议这样理解：

- [AGENTS.md](AGENTS.md)
  - 当前仓库协作规则、分层边界、接口、迁移、权限、上传、任务和验证要求；优先级高于本文的经验性说明。

- [README.md](README.md)
  - 对外项目介绍、基础使用说明。

- [docs/COMMANDS_AND_TASKS.md](docs/COMMANDS_AND_TASKS.md)
  - 命令说明、定时任务与操作约束。

- [docs/MIGRATE_COMMANDS.md](docs/MIGRATE_COMMANDS.md)
  - 迁移命令、版本和安全边界。

- [docs/SYSTEM_CONFIG_AND_DICT_GUIDELINES.md](docs/SYSTEM_CONFIG_AND_DICT_GUIDELINES.md)
  - 系统配置与字典的职责和 i18n 语义。

- [tests/README.md](tests/README.md)
  - 接口测试目录分工；其中接口总数和文件清单可能随路由增长滞后，实际覆盖以 `AdminRouteTree()` 和当前测试代码为准。

- 本文档
  - 给下一位 AI 的“当前状态 + 接手约束 + 写码风格”说明。

如果文档和代码冲突：

- 以代码为准。
- 再顺手修正文档。

## 12. 一句话接手策略

先读入口和当前链路，再读与你任务相关的 service；优先复用现有结构，保持命名和接口稳定，用小步修改把需求做完，维持项目既有风格。
