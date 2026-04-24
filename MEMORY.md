# MEMORY

这份文档写给接手 `gin-layout` 的 AI Agent。它不是项目营销介绍，而是为了让下一位 AI 尽快知道：

- 项目当前真实状态是什么
- 主要入口、核心链路和高风险区域在哪
- 关键约束和禁止事项是什么
- 写代码时如何保持当前项目风格

## 0. 快速结论

先记住这些，不确定时再往下查细节：

- 代码是真相，文档只是辅助；文档和代码冲突时，以代码为准并顺手修正文档。
- 新功能优先沿用现有 `service / model / resources / validator` 分层。
- 保持现有工程范式，不引入重框架化、函数式大重写、新 DI 容器或 repository 全量替换。
- Casbin 使用 `github.com/casbin/casbin/v3`。
- 鉴权链路采用 claims-first：请求上下文保存 `AuthPrincipal`，不是完整数据库 `AdminUser`。
- 错误文案按请求语言国际化返回；需要自定义错误文案时优先使用错误码或 message key，而不是在 controller 里硬编码中文。
- 请求审计是“审计快照 + 队列优先”：队列开启时异步落库，队列关闭时才同步落库审计日志。
- CORS 使用常规 `["*"]` 语义。
- 菜单列表/树只返回当前语言 `title`；菜单详情返回 `title_i18n`，不返回 `title`。
- 继续改代码时保持小步修改、局部重组、命名稳定。
- 改完代码必须自己验证；新增或修改 HTTP 行为默认要补 `tests/` 接口级测试。

## 1. 禁止事项

当前代码库禁止这样做：

- 引入 `casbin/v2` 或混用不同主版本的 Casbin adapter。
- 在认证中间件里每个请求都回表查询完整 `AdminUser`；账号状态变更应通过服务层撤销 token 或更新会话状态生效。
- 把 `AuthPrincipal.AdminUser()` 当成数据库实时状态。
- 在队列开启时同步落库请求审计日志。
- 把请求日志设计成“每个请求同步写文件 + 异步审计”的双写链路。
- 把 CORS 空数组当成全放开；显式全放开使用 `["*"]`。
- 让菜单列表或用户菜单树返回 `title_i18n`。
- 让菜单详情返回 `title`。
- 让菜单写接口接收 `title`；写接口使用 `title_i18n`。
- 把业务逻辑、事务或批量数据修复塞进 controller。
- 把所有逻辑塞回单个超大 service 文件。
- 绕过 `PermissionSyncCoordinator` 分散写入 Casbin 权限。
- 只改代码不跑测试，或把验证责任留给用户或下一个 AI。

## 2. 当前定位

项目名：`gin-layout`

这是一个偏后台管理场景的 Go 后端骨架，内置：

- JWT 登录、校验、刷新、登出
- Casbin RBAC 接口权限控制
- 管理员、角色、部门、菜单、API 权限体系
- 请求日志、登录日志、审计日志
- 文件上传与本地文件访问
- CLI 命令、定时任务、异步队列 worker

运行形态包含三类进程：

1. `service`
   - 提供 HTTP API
   - 构建请求审计日志快照
   - 队列开启时把审计快照投递到异步队列
   - 队列关闭时在当前请求链路同步落库审计日志

2. `worker`
   - 消费异步任务
   - 主要消费请求审计日志异步落库

3. `cron`
   - 负责周期任务

## 3. 当前事实

这些是当前实现与开发规范。

### 3.1 Casbin v3

权限引擎是：

- `github.com/casbin/casbin/v3`
- `github.com/casbin/gorm-adapter/v3`

相关入口：

- [internal/access/casbin/casbin.go](/Users/liuml/data/go/src/go-layout/internal/access/casbin/casbin.go:1)
- [internal/access/casbin/adapter.go](/Users/liuml/data/go/src/go-layout/internal/access/casbin/adapter.go:1)

### 3.2 Claims-First 鉴权

请求上下文保存 claims 快照，不保存完整数据库 `AdminUser` 模型：

- [internal/service/auth/principal.go](/Users/liuml/data/go/src/go-layout/internal/service/auth/principal.go:1)
- [internal/middleware/parse_token.go](/Users/liuml/data/go/src/go-layout/internal/middleware/parse_token.go:1)
- [internal/middleware/admin_auth.go](/Users/liuml/data/go/src/go-layout/internal/middleware/admin_auth.go:1)

这意味着：

- 中间件不应为每个请求回表查用户。
- 需要用户实时数据库状态的地方，由具体业务接口显式查库。
- `AuthPrincipal.AdminUser()` 是轻量投影，不代表数据库最新状态。

### 3.3 请求日志

请求日志链路以审计日志为核心：

- 请求链路：缓存请求体、录制响应体，构建审计快照。
- 队列开启：把审计快照投给队列，由 worker 异步落库。
- 队列关闭：在当前请求链路同步落库审计日志。

全局 zap 文件日志用于系统日志、错误日志和 panic 日志；每个请求日志按审计快照链路处理。

关键文件：

- [internal/middleware/logger.go](/Users/liuml/data/go/src/go-layout/internal/middleware/logger.go:1)
- [internal/middleware/audit_queue.go](/Users/liuml/data/go/src/go-layout/internal/middleware/audit_queue.go:1)
- [internal/jobs/audit_log.go](/Users/liuml/data/go/src/go-layout/internal/jobs/audit_log.go:1)
- [cmd/worker/worker.go](/Users/liuml/data/go/src/go-layout/cmd/worker/worker.go:1)

### 3.4 CORS

支持常规 `*` 语义：

- `cors_origins: ["*"]`
- `cors_methods: ["*"]`
- `cors_headers: ["*"]`
- `cors_expose_headers: ["*"]`

实现位置：

- [internal/middleware/cors.go](/Users/liuml/data/go/src/go-layout/internal/middleware/cors.go:1)
- [config/config.yaml.example](/Users/liuml/data/go/src/go-layout/config/config.yaml.example:17)

### 3.5 错误国际化

错误文案由请求语言驱动：

- `RequestLocaleHandler` 从 `Accept-Language` 解析语言并写入请求上下文。
- 响应层根据上下文语言选择错误文案语言。
- 通用业务错误默认走错误码文案表。
- 需要细粒度错误文案时，优先使用 `message key`，由响应层统一国际化。
- 参数校验错误由 validator translator 输出多语言文案，字段名优先使用 `label/json/form` 标签。

关键文件：

- [internal/middleware/request_locale.go](/Users/liuml/data/go/src/go-layout/internal/middleware/request_locale.go:1)
- [internal/pkg/i18n/locale.go](/Users/liuml/data/go/src/go-layout/internal/pkg/i18n/locale.go:1)
- [internal/pkg/errors/error.go](/Users/liuml/data/go/src/go-layout/internal/pkg/errors/error.go:1)
- [internal/pkg/errors/zh-cn.go](/Users/liuml/data/go/src/go-layout/internal/pkg/errors/zh-cn.go:1)
- [internal/pkg/errors/en-us.go](/Users/liuml/data/go/src/go-layout/internal/pkg/errors/en-us.go:1)
- [internal/pkg/response/response.go](/Users/liuml/data/go/src/go-layout/internal/pkg/response/response.go:1)
- [internal/validator/runtime.go](/Users/liuml/data/go/src/go-layout/internal/validator/runtime.go:1)
- [internal/validator/translation.go](/Users/liuml/data/go/src/go-layout/internal/validator/translation.go:1)
- [internal/validator/binding.go](/Users/liuml/data/go/src/go-layout/internal/validator/binding.go:1)

### 3.6 菜单 I18n

菜单标题由请求语言驱动：

- 列表/树：只返回当前语言 `title`，不返回 `title_i18n`。
- 详情：返回 `title_i18n` 供编辑回填，不返回 `title`。
- 写接口：使用 `title_i18n`，支持 `zh-CN` 与 `en-US`，至少一种非空。

关键文件：

- [internal/controller/admin_v1/auth_menu.go](/Users/liuml/data/go/src/go-layout/internal/controller/admin_v1/auth_menu.go:1)
- [internal/service/menu/menu_query.go](/Users/liuml/data/go/src/go-layout/internal/service/menu/menu_query.go:1)
- [internal/service/menu/menu_edit.go](/Users/liuml/data/go/src/go-layout/internal/service/menu/menu_edit.go:1)
- [internal/resources/menu.go](/Users/liuml/data/go/src/go-layout/internal/resources/menu.go:1)
- [tests/admin_test/menu_test.go](/Users/liuml/data/go/src/go-layout/tests/admin_test/menu_test.go:1)

### 3.7 文件组织

典型职责包：

- `internal/service/admin/`
- `internal/service/role/`
- `internal/service/dept/`
- `internal/service/access/`
- `internal/validator/`
- `internal/model/`

继续改时，优先往对应职责文件里放；可以同包拆文件，保持文件职责清晰。

## 4. 任务阅读索引

新会话建议先看：

1. [README.md](/Users/liuml/data/go/src/go-layout/README.md:1)
2. [docs/COMMANDS_AND_TASKS.md](/Users/liuml/data/go/src/go-layout/docs/COMMANDS_AND_TASKS.md:1)
3. [cmd/root.go](/Users/liuml/data/go/src/go-layout/cmd/root.go:1)
4. [cmd/service/service.go](/Users/liuml/data/go/src/go-layout/cmd/service/service.go:1)
5. [internal/routers](/Users/liuml/data/go/src/go-layout/internal/routers)

按任务追加阅读：

- 新增或修改后台接口：
  - [internal/routers](/Users/liuml/data/go/src/go-layout/internal/routers)
  - [internal/controller/admin_v1](/Users/liuml/data/go/src/go-layout/internal/controller/admin_v1)
  - [internal/validator/form](/Users/liuml/data/go/src/go-layout/internal/validator/form)
  - [internal/service](/Users/liuml/data/go/src/go-layout/internal/service)
  - [internal/resources](/Users/liuml/data/go/src/go-layout/internal/resources)
  - [tests/admin_test](/Users/liuml/data/go/src/go-layout/tests/admin_test)

- 权限 / Casbin / 菜单可见性：
  - [internal/access/casbin](/Users/liuml/data/go/src/go-layout/internal/access/casbin)
  - [internal/service/access](/Users/liuml/data/go/src/go-layout/internal/service/access)
  - [rbac_model.conf](/Users/liuml/data/go/src/go-layout/rbac_model.conf:1)

- 鉴权 / token / 当前用户：
  - [internal/middleware/parse_token.go](/Users/liuml/data/go/src/go-layout/internal/middleware/parse_token.go:1)
  - [internal/middleware/admin_auth.go](/Users/liuml/data/go/src/go-layout/internal/middleware/admin_auth.go:1)
  - [internal/service/auth](/Users/liuml/data/go/src/go-layout/internal/service/auth)

- 请求日志 / 审计 / 队列：
  - [internal/middleware/logger.go](/Users/liuml/data/go/src/go-layout/internal/middleware/logger.go:1)
  - [internal/middleware/audit_queue.go](/Users/liuml/data/go/src/go-layout/internal/middleware/audit_queue.go:1)
  - [internal/jobs/audit_log.go](/Users/liuml/data/go/src/go-layout/internal/jobs/audit_log.go:1)
  - [internal/service/audit](/Users/liuml/data/go/src/go-layout/internal/service/audit)
  - [cmd/worker/worker.go](/Users/liuml/data/go/src/go-layout/cmd/worker/worker.go:1)

- 菜单标题国际化：
  - [internal/service/menu](/Users/liuml/data/go/src/go-layout/internal/service/menu)
  - [internal/model/menu_i18n.go](/Users/liuml/data/go/src/go-layout/internal/model/menu_i18n.go:1)
  - [internal/resources/menu.go](/Users/liuml/data/go/src/go-layout/internal/resources/menu.go:1)
  - [tests/admin_test/menu_test.go](/Users/liuml/data/go/src/go-layout/tests/admin_test/menu_test.go:1)

- 模型基础能力 / 列表分页 / 删除：
  - [internal/model/base.go](/Users/liuml/data/go/src/go-layout/internal/model/base.go:1)
  - [internal/model/base_crud.go](/Users/liuml/data/go/src/go-layout/internal/model/base_crud.go:1)
  - [internal/model/base_list.go](/Users/liuml/data/go/src/go-layout/internal/model/base_list.go:1)
  - [internal/model/base_tree.go](/Users/liuml/data/go/src/go-layout/internal/model/base_tree.go:1)

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

响应整形采用显式 transformer，不直接把 model 原样 JSON 输出。

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
- `system_defaults`
- `user_permission_sync`

外层统一从 `PermissionSyncCoordinator` 进入，业务层通过协调器触发 Casbin 权限同步。

## 6. 开发风格

### 6.1 命名

- service 名称以业务对象命名，如 `AdminUserService`、`RoleService`。
- constructor 用 `NewXxxService()`。
- 业务入口方法名直接用动词：`List`、`Create`、`Update`、`Delete`、`BindRole`。
- 内部辅助方法用小写，尽量语义直接：`buildListCondition`、`updateDeptRole`。

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

业务错误码优先复用，保持现有错误码风格。

### 6.4 事务与权限刷新

事务与权限刷新规范：

- service 层显式事务。
- 事务工具统一复用。
- 权限刷新或缓存刷新在事务提交后处理。

“事务 + 后置刷新”保持在 service 层编排。

### 6.5 响应

接口响应走统一响应封装，不直接返回裸对象。

所以：

- controller 保持现有 response 风格。
- 列表接口尽量返回 collection。
- detail 接口尽量走 transformer。

### 6.6 注释

注释使用少量中文，说明职责和意图；除非必要复杂需求才写长注释。

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
- 队列异步投递。
- 队列关闭时的同步审计落库。
- 性能与日志正确性平衡。

### 9.4 `internal/model/base_*`

风险点：

- 很多 model 共用。
- 一旦改坏，会影响列表、分页、删除、防误删逻辑。

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

## 11. 文档关系

根目录文档建议这样理解：

- [README.md](/Users/liuml/data/go/src/go-layout/README.md:1)
  - 对外项目介绍、基础使用说明。

- [docs/COMMANDS_AND_TASKS.md](/Users/liuml/data/go/src/go-layout/docs/COMMANDS_AND_TASKS.md:1)
  - 命令说明、定时任务与操作约束。

- 本文档
  - 给下一位 AI 的“当前状态 + 接手约束 + 写码风格”说明。

如果文档和代码冲突：

- 以代码为准。
- 再顺手修正文档。

## 12. 一句话接手策略

先读入口和当前链路，再读与你任务相关的 service；优先复用现有结构，保持命名和接口稳定，用小步修改把需求做完，维持项目既有风格。
