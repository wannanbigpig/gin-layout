# MEMORY

这份文档写给接手 `gin-layout` 的 AI Agent，不是给首次了解项目的人做营销介绍，而是帮助下一位 AI 在最短时间内回答这几个问题：

- 这个项目现在到底是什么状态
- 主要代码入口和核心链路在哪
- 哪些旧约定已经被改掉，不能再按老理解写
- 继续改代码时，应该遵守什么风格，避免把项目写散

结论先说：

- 代码是真相，文档只是辅助
- 新功能优先沿用现有 service / model / resource / validator 分层
- 不要把这个项目往“重框架化”或“函数式大重写”方向带
- 保持小步修改、局部重组、命名稳定

## 1. 项目当前定位

项目名：`gin-layout`

这是一个偏后台管理场景的 Go 后端骨架，已经内置了这些能力：

- JWT 登录、校验、刷新、登出
- Casbin RBAC 接口权限控制
- 管理员、角色、部门、菜单、API 权限体系
- 请求日志、登录日志、审计日志
- 文件上传与本地文件访问
- CLI 命令、定时任务、异步队列 worker

当前运行形态不是只有一个 HTTP 服务，而是三类进程：

1. `service`
   - 提供 HTTP API
   - 处理同步文件日志
   - 把审计日志快照投递到异步队列

2. `worker`
   - 消费异步任务
   - 当前主要消费请求审计日志异步落库

3. `cron`
   - 负责周期任务

## 2. 必须先知道的当前事实

这些是“现在的真实实现”，不要按旧印象写。

### 2.1 Casbin 已统一到 v3

当前权限引擎是：

- `github.com/casbin/casbin/v3`

相关入口：

- [internal/access/casbin/casbin.go](/Users/liuml/data/go/src/go-layout/internal/access/casbin/casbin.go:1)

不要再引入 `casbin/v2` 风格代码，也不要混用不同主版本 adapter。

### 2.2 认证链路已经是 claims-first

当前请求上下文里不再默认塞完整 `AdminUser` 模型，而是保存 claims 快照：

- [internal/service/auth/principal.go](/Users/liuml/data/go/src/go-layout/internal/service/auth/principal.go:1)

这意味着：

- 中间件默认不应为每个请求回表查用户
- 需要用户实时数据库状态的地方，由具体业务接口显式查库
- `AuthPrincipal.AdminUser()` 只是兼容旧逻辑的轻量投影，不代表数据库最新状态

### 2.3 请求日志是“同步文件 + 异步审计”

当前日志链路分两段：

- 同步：写文件日志
- 异步：把审计快照投给队列，由 worker 落库

关键文件：

- [internal/middleware/logger.go](/Users/liuml/data/go/src/go-layout/internal/middleware/logger.go:1)
- [internal/middleware/audit_queue.go](/Users/liuml/data/go/src/go-layout/internal/middleware/audit_queue.go:1)
- [internal/jobs/audit_log.go](/Users/liuml/data/go/src/go-layout/internal/jobs/audit_log.go:1)

不要把请求链路里的审计写库重新改回同步。

### 2.4 CORS 已改成常规 `*` 语义

当前支持：

- `cors_origins: ["*"]`
- `cors_methods: ["*"]`
- `cors_headers: ["*"]`
- `cors_expose_headers: ["*"]`

实现位置：

- [internal/middleware/cors.go](/Users/liuml/data/go/src/go-layout/internal/middleware/cors.go:1)

不要再引入“空数组表示全放开”的特殊语义。

### 2.5 大文件已经按职责拆过一轮

最近做过一轮结构整理，典型例子：

- `internal/service/admin/`
- `internal/service/role/`
- `internal/service/dept/`
- `internal/service/access/`
- `internal/validator/`
- `internal/model/`

所以继续改时，优先往已有职责文件里放，不要再把所有逻辑塞回一个超大文件。

## 3. 推荐阅读顺序

一个新的 AI 会话，建议按这个顺序看代码：

1. [README.md](/Users/liuml/data/go/src/go-layout/README.md:1)
2. [AI_DEPLOYMENT.md](/Users/liuml/data/go/src/go-layout/AI_DEPLOYMENT.md:1)
3. [cmd/root.go](/Users/liuml/data/go/src/go-layout/cmd/root.go:1)
4. [cmd/service/service.go](/Users/liuml/data/go/src/go-layout/cmd/service/service.go:1)
5. [internal/routers](/Users/liuml/data/go/src/go-layout/internal/routers)
6. [internal/controller/admin_v1](/Users/liuml/data/go/src/go-layout/internal/controller/admin_v1)
7. [internal/service](/Users/liuml/data/go/src/go-layout/internal/service)
8. [internal/model](/Users/liuml/data/go/src/go-layout/internal/model)

如果任务涉及权限，再追加：

1. [internal/access/casbin](/Users/liuml/data/go/src/go-layout/internal/access/casbin)
2. [internal/service/access](/Users/liuml/data/go/src/go-layout/internal/service/access)
3. `rbac_model.conf`

如果任务涉及鉴权，再追加：

1. [internal/middleware/parse_token.go](/Users/liuml/data/go/src/go-layout/internal/middleware/parse_token.go:1)
2. [internal/middleware/admin_auth.go](/Users/liuml/data/go/src/go-layout/internal/middleware/admin_auth.go:1)
3. [internal/service/auth](/Users/liuml/data/go/src/go-layout/internal/service/auth)

## 4. 目录理解方式

不要只把目录当成物理位置，要理解它们的职责边界。

### 4.1 `cmd/`

负责程序入口和启动编排，不负责业务逻辑。

关键命令：

- `service`
- `worker`
- `cron`
- `command`

### 4.2 `internal/controller/`

HTTP 控制器层。

职责：

- 收参
- 调 validator/form
- 调 service
- 返回 response

不应该在 controller 里写复杂业务判断、事务或批量数据修复逻辑。

### 4.3 `internal/service/`

业务层核心。

职责：

- 业务规则
- 事务编排
- 模型组合查询
- 触发权限同步
- 触发 token 撤销

这个项目的大部分改动都应该优先落在 service 层。

### 4.4 `internal/model/`

GORM 模型层和基础数据访问层。

职责：

- 表结构
- 通用 CRUD
- 列表分页
- 树形节点辅助

不要把大段复杂业务判断塞进 model。

### 4.5 `internal/resources/`

响应整形层。

职责：

- 把 model / service 结果转换成前端要的结构

已有项目风格是显式 transformer，不是直接把 model 原样 JSON 输出。

### 4.6 `internal/validator/` 与 `internal/validator/form/`

参数对象和校验规则层。

职责：

- 定义请求参数 struct
- 定义 tag 校验规则
- 统一错误翻译和绑定错误处理

### 4.7 `internal/service/access/`

权限同步协调层。

当前已按职责拆分为：

- `scope_resolver`
- `graph_loader`
- `policy_builder`
- `coordinator`
- `user_permission_sync`

外层统一从 `PermissionSyncCoordinator` 进入，不要把业务层直接散落地接 Casbin 写权限。

## 5. 当前项目风格

这是最重要的一节。下一位 AI 要尽量“像这个项目原本就在这样写”，而不是带入别的仓库风格。

### 5.1 命名风格

- service 名称以业务对象命名，如 `AdminUserService`、`RoleService`
- constructor 用 `NewXxxService()`
- 业务入口方法名直接用动词：`List`、`Create`、`Update`、`Delete`、`BindRole`
- 内部辅助方法用小写，尽量语义直接：`buildListCondition`、`updateDeptRole`

不要突然引入很抽象的命名，例如：

- `Processor`
- `Manager`
- `Facade`
- `RepositoryFactory`

除非代码里已经有明确同类模式。

### 5.2 结构风格

优先做“同包拆文件”，不要轻易新增一层新 package。

项目当前接受的结构整理方式是：

- 保留 `package` 不变
- 保留原 service 名称不变
- 把一个大文件拆成多个职责文件

这比横向新建一堆新抽象更符合当前代码库风格。

### 5.3 错误处理风格

业务错误优先使用：

- `internal/pkg/errors`

典型方式：

- `e.NewBusinessError(...)`

已有业务错误就复用，不要重复发明一套新的错误码风格。

### 5.4 事务风格

已有项目偏好：

- service 层显式事务
- 事务工具统一复用
- 权限刷新或缓存刷新在事务提交后处理

不要把“事务 + 后置刷新”拆散到 controller 或 middleware。

### 5.5 响应风格

已有项目不是直接返回裸对象，而是走统一响应封装。

所以：

- controller 保持现有 response 风格
- 列表接口尽量返回 collection
- detail 接口尽量走 transformer

### 5.6 注释风格

当前项目注释是“少量中文、说明职责和意图”，不是论文式注释，除非必要复杂需求才写长注释。
明确注释的目的是说清楚职责和意图，而不是写文档。

可以写：

- `// BindRole 绑定角色。`
- `// ReloadPolicyCache 在事务提交后刷新共享 Casbin Enforcer 的内存策略。`

不要写大段无信息密度的注释，也不要给每一行都配解释。

## 6. 写代码时要遵守的几条硬规则

### 6.1 先复用，再新增

做任何功能前，先查：

- 现有 service 是否已有类似逻辑
- model 是否已有通用方法
- validator 是否已有相同校验模式
- resources 是否已有类似 transformer

不要重复造轮子。

### 6.2 优先小步修改，不做整层重写

这个项目是工程型仓库，不是实验仓库。

所以：

- 可以局部重构
- 可以拆文件
- 可以补测试
- 但不要无缘无故把整层改成另一种范式

尤其不要突然引入：

- repository 模式全量替换
- 泛型 service 框架大改
- 新 DI 容器
- 新 HTTP 框架

### 6.3 保持接口签名稳定

如果只是做结构优化或内部修复：

- controller 签名不改
- service 对外方法名尽量不改
- HTTP API 不改
- CLI 命令名不改

这是当前项目非常重要的稳定性原则。

### 6.4 新增行为要配测试

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

- 这个项目已经有明确的 `tests/admin_test` 集成测试入口
- 只补 service 或 middleware 级测试，不足以证明接口链路真实可用
- 新接口至少要验证路由、鉴权、请求参数、响应结构中的关键路径

默认要求：

- 新增 HTTP 接口：补 `tests/` 目录下的接口测试
- 修改现有 HTTP 接口行为：补或更新 `tests/` 目录下对应测试
- 只改内部实现且对外行为不变：可以只补包内测试，但要能说明为什么不需要接口测试

### 6.5 每次改完代码，先自行验证

这是接手本项目时必须遵守的执行规则：

- 改完代码后，先自己跑测试，再决定是否结束本轮工作
- 不要把“验证是否正常”的责任留给下一个 AI 或用户
- 如果改动影响面明确，至少跑受影响包的测试
- 如果改动涉及公共层、中间件、权限、鉴权、模型基础层，优先跑 `go test ./...`

最小要求：

- 小改动：跑直接受影响包测试
- 中等改动：跑相关子系统测试
- 公共层或高风险改动：跑全量测试

如果这次改动引入了新行为、新分支、新配置语义或修复过的回归点，默认应补测试。

不要接受下面这种结束方式：

- 只改代码，不跑测试
- 只说“理论上没问题”
- 只让用户自己去验证

如果因为环境限制无法完成某项测试，也必须在结果里明确写出：

- 哪些测试跑了
- 哪些测试没跑
- 没跑的原因是什么

推荐基线：

- 包级验证：`go test ./internal/...`
- 全量验证：`go test ./...`
- 中间件 / 权限 / 鉴权改动：`go test ./internal/middleware ./internal/service/access ./internal/service/auth`

## 7. 最常见开发路径

如果要新增一个后台接口，建议按这个顺序：

1. 在 `internal/validator/form/` 定义参数 struct
2. 在 `internal/model/` 复用或补充数据访问方法（查库、写库、事务内操作）
3. 在 `internal/service/` 写业务逻辑（组合 model、权限校验、事务协调）
4. 在 `internal/resources/` 补返回结构
5. 在 `internal/controller/admin_v1/` 加控制器方法
6. 在 `internal/routers/` 的声明式路由树里注册
7. 如涉及权限元数据，同步执行 `command api-route`
8. 如涉及角色/菜单/API 关系变化，确认权限重建链路是否需要触发

## 8. 高风险区域

这些地方改动前必须先看上下游：

### 8.1 `internal/service/access`

风险点：

- 会影响菜单可见性
- 会影响 API 权限
- 会影响 Casbin 最终策略

### 8.2 `internal/service/auth`

风险点：

- claims-first 已经落地
- token 黑名单 / 撤销逻辑已收口
- 改不好会影响所有登录态请求

### 8.3 `internal/middleware/logger*`

风险点：

- 请求体截断
- 响应体捕获
- 队列异步投递
- 性能与日志正确性平衡

### 8.4 `internal/model/base_*`

风险点：

- 很多 model 共用
- 一旦改坏，会影响列表、分页、删除、防误删逻辑

## 9. 接手时的推荐检查命令

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

## 10. 文档之间的关系

当前根目录文档建议这样理解：

- [README.md](/Users/liuml/data/go/src/go-layout/README.md:1)
  - 对外项目介绍、基础使用说明

- [AI_DEPLOYMENT.md](/Users/liuml/data/go/src/go-layout/AI_DEPLOYMENT.md:1)
  - 部署、启动、验证、排障

- 本文档
  - 给下一位 AI 的“当前状态 + 接手约束 + 写码风格”说明

如果文档和代码冲突：

- 以代码为准
- 再顺手修正文档

## 11. 一句话接手策略

先读入口和当前链路，再读与你任务相关的 service；优先复用现有结构，保持命名和接口稳定，用小步修改把需求做完，不要把项目写成另一套风格。

## 12. 最近优化记录

2026-04-16 完成一轮 OPTIMIZATION_REPORT.md 优化修复：

- JWT Secret 为空现在会 Fatal 退出（不再静默生成随机值）
- Redis/DB 关闭失败现在记录日志（不再忽略错误）
- policy_builder.go Slice 预分配容量优化
- api_cache.go Redis 客户端复用优化
- 清理 request_cost.go 注释代码

详见 [memory/optimization_20260416.md](memory/optimization_20260416.md)
