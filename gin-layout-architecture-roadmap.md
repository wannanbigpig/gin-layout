# gin-layout 架构完善、权限体系与开源社区建设规划

> 项目：[`wannanbigpig/gin-layout`](https://github.com/wannanbigpig/gin-layout)  
> 评审日期：2026-09-14  
> 文档性质：架构审计 + 演进路线图 + 开源治理规划  
> 目标定位建议：**面向中后台、SaaS 和企业业务系统的生产级 Gin 后端基础框架（Production-ready Gin Backend Foundation）**

---

## 1. 结论摘要

`gin-layout` 当前已经不是一个简单的 Gin Demo，而是一套具备较完整后台基础设施的工程骨架：

- JWT Access Token + Refresh Token 双 Token 认证
- RBAC 用户 / 角色 / 部门 / 菜单 / API 权限
- 基于 Casbin 的接口鉴权
- 角色数据范围 DataScope
- Redis 数据权限缓存 + singleflight 防击穿
- 登录日志、请求日志
- 文件存储
- Asynq 异步任务
- Cron 定时任务
- WebSocket / 系统通知
- I18n
- 数据库 Migration
- Cobra CLI
- CRUD Generator
- `/ping` 与 readiness 探针
- Go CI + CodeQL
- 独立 API / Worker / Cron 运行模式

**整体架构方向是合理的。**

后续不建议把项目做成“另一个若依 / Gin-Vue-Admin”，不断往核心仓库塞商城、CRM、工作流等业务功能。更适合形成自己的差异化：

> **保持核心框架克制，重点强化权限安全、模块化、SaaS 多租户、可观测性、代码生成、扩展点和开源工程质量。**

当前最值得优先解决的几个问题：

1. DataScope 已经实现得不错，但仍依赖业务 Service **显式接入**，存在新模块遗漏数据权限的风险。
2. 功能权限目前主要围绕菜单 / API / Casbin，建议增加稳定的 **Permission Code（业务权限码）**。
3. 目录主要仍按 `controller/service/model/...` 技术层组织，项目继续扩大后会出现横向目录膨胀。
4. Model 中仍承担数据库查询职责，建议逐步引入 Repository。
5. 未来若面向 SaaS/ERP，多租户 Tenant 是最重要的架构缺口之一。
6. 审计日志需要从“请求日志”升级为“业务资源变更审计”。
7. 需要统一 Request ID / Trace ID，并逐步接入 Metrics / OpenTelemetry。
8. Generator 可以成为项目核心竞争力，应升级成“模块生成器”而非简单 CRUD 生成器。
9. 当前开源治理和 Release 体系明显弱于代码本身，建议在 v1.0 前重点补齐。
10. 不建议现在微服务化，继续坚持 **模块化单体（Modular Monolith）** 更符合项目阶段。

---

# 2. 本次参考的开源项目

本规划主要参考以下项目。参考不是要求照搬，而是观察成熟项目在权限、租户、模块化、文档、社区治理等方面已经验证过的实践。

## 2.1 Gin-Vue-Admin

仓库：

- <https://github.com/flipped-aurora/gin-vue-admin>

主要参考点：

- Gin + GORM + Casbin + JWT 的成熟后台组合
- 用户、角色、菜单、API 权限模型
- Swagger/API 文档
- CRUD / 表单生成
- 插件生态
- README 内直接提供贡献规范
- 文档、演示站、社区入口相对完整

其 README 明确描述了 JWT、动态路由、动态菜单、Casbin、代码生成器，以及角色可分配 API 权限和菜单权限。

**来源：**

- `flipped-aurora/gin-vue-admin/README.md`
- <https://github.com/flipped-aurora/gin-vue-admin/blob/main/README.md>

**对 gin-layout 的启示：**

不是继续堆同样的后台功能，而是学习其：

- 开箱体验
- API 文档
- Generator
- Demo / Docs
- 社区入口

---

## 2.2 Simple Admin

仓库：

- <https://github.com/suyuan32/simple-admin-core>

技术路线：

- go-zero
- Ent
- Casbin
- Vben Admin

主要参考点：

- 模块化拆分
- API / RPC / Web 三端代码生成
- 多语言
- 服务注册发现
- 独立文件模块、任务模块、消息模块、会员模块
- 多租户版本
- 社区模块机制

其 README 将 Core、File、Job、Member、Message Center 等拆分成独立模块，并明确提供代码生成、动态路由权限和多租户版本。

**来源：**

- `suyuan32/simple-admin-core/README.md`
- <https://github.com/suyuan32/simple-admin-core/blob/main/README.md>

**对 gin-layout 的启示：**

未来业务能力应尽量模块化，核心仓库保持稳定，外围功能通过 Module / Driver / Extension 扩展。

---

## 2.3 GoWind Admin

仓库：

- <https://github.com/tx7do/go-wind-admin>

技术路线：

- Go Kratos
- Protobuf
- Ent / GORM
- Casbin / OPA / Zanzibar
- Asynq
- Jaeger
- 多前端

主要参考点：

- 多租户
- 行级数据权限
- 字段级权限
- 审计日志
- Trace ID
- 单体 / 微服务可切换
- OpenAPI / Protobuf 驱动
- 生产安全能力
- Tenant 隔离默认注入

其 README 明确说明：

- 支持多租户、多角色、多部门、菜单/按钮/数据级权限；
- 审计日志覆盖登录、操作、API、数据访问、权限变更、策略评估，并记录 `trace_id`；
- 使用 Ent Privacy 对租户查询自动注入租户过滤；
- 创建、更新、删除也做租户隔离；
- 支持角色行级数据范围和字段级权限。

**来源：**

- `tx7do/go-wind-admin/README.md`
- <https://github.com/tx7do/go-wind-admin/blob/main/README.md>

**对 gin-layout 的启示：**

重点参考它的：

- 默认安全的 Tenant Isolation
- 审计模型
- 行级 + 字段级权限
- Observability
- 单体优先但保留未来服务化能力

不建议直接复制其 Kratos / Protobuf / Ent 重型架构。

---

## 2.4 GoAdmin

仓库：

- <https://github.com/GoAdminGroup/go-admin>

主要参考点：

- 开源贡献流程
- 插件/主题扩展
- 将简单改动与复杂改动区分处理
- 提交前强制执行测试

其 `CONTRIBUTING.md` 明确约定：

- 小修复可以直接提交 PR；
- 较复杂功能应先通过 Issue 讨论；
- 提交前执行 `make test`；
- 明确列出翻译、Bug Hunting、安全、主题、插件等社区贡献方向。

**来源：**

- `GoAdminGroup/go-admin/CONTRIBUTING.md`
- <https://github.com/GoAdminGroup/go-admin/blob/main/CONTRIBUTING.md>

**对 gin-layout 的启示：**

社区建设不能只靠“欢迎 PR”，必须主动定义低门槛贡献入口。

---

# 3. gin-layout 当前架构评估

## 3.1 当前定位

当前 README 已经明确把项目定位为：

> 基于 Gin 的后台管理系统脚手架，把后台管理场景中高频、重复、工程化要求高的基础能力沉淀成可直接落地的后端骨架。

这是一个正确的定位。

**来源：**

- `gin-layout/README.md`
- <https://github.com/wannanbigpig/gin-layout/blob/master/README.md>

当前项目的优势不是某个单独功能，而是已经形成一整套工程能力：

```text
HTTP / Gin
    │
    ├── Auth
    ├── RBAC / Casbin
    ├── DataScope
    ├── Logs
    ├── File
    ├── Queue / Asynq
    ├── Cron
    ├── Realtime
    ├── I18n
    ├── Migration
    ├── CLI
    ├── Generator
    └── Health Probe
```

建议继续沿这个方向发展。

---

# 4. 权限体系评审

## 4.1 当前 RBAC 关系已经比较完整

当前仓库中可以看到：

```text
AdminUser
   │
   ├── AdminUserRoleMap
   │          │
   │          └── Role
   │                │
   │                ├── RoleMenuMap
   │                └── RoleDataScopeDept
   │
   └── AdminUserDeptMap
              │
              └── Department
                     │
                     └── DeptRoleMap
```

意味着用户权限来源至少包含：

1. 用户直接绑定角色；
2. 用户所在部门继承部门角色；
3. 角色分配菜单；
4. 角色配置 DataScope；
5. 自定义 DataScope 可以绑定部门。

**来源：**

- `internal/model/admin_user_role_map.go`
- `internal/model/dept_role_map.go`
- `internal/model/role_menu_map.go`
- `internal/model/role_data_scope_dept.go`
- `internal/model/role.go`

仓库：

- <https://github.com/wannanbigpig/gin-layout/tree/master/internal/model>

### 当前评价

**RBAC 基础：完整度较高。**

建议不要再为了“完整”盲目增加更多 Role 类型，而是把已有模型进一步稳定化。

---

# 5. DataScope 当前实现评审

## 5.1 当前已经实现 5 档数据范围

`Role.DataScope` 当前定义：

```text
1 = 全部数据
2 = 本部门及子级
3 = 本部门
4 = 仅本人
5 = 自定义部门
```

**来源：**

- `internal/model/role.go`
- <https://github.com/wannanbigpig/gin-layout/blob/master/internal/model/role.go>

这套模型本身是成熟且实用的。

---

## 5.2 DataScopeFilter 的实现质量较好

当前：

- `DataScopeInput` 不直接依赖 `gin.Context`
- 支持 HTTP、Worker、Cron 使用
- 支持异步任务权限快照
- Redis 缓存权限计算结果
- 使用 `singleflight` 防止缓存击穿
- Redis 不可用时回源数据库
- 权限计算失败时返回 `1=0`
- 超级管理员可以短路
- 多角色 DataScope 做并集
- 任意角色拥有 ALL 即全部放行
- 部门子树动态收集
- 有缓存失效接口

**来源：**

- `internal/service/access/data_scope_filter.go`
- <https://github.com/wannanbigpig/gin-layout/blob/master/internal/service/access/data_scope_filter.go>

其中这一点尤其值得保留：

```text
无法正确计算权限
        ↓
      1 = 0
        ↓
     默认拒绝
```

属于 **fail-closed**，比失败后默认放行更安全。

---

# 6. 当前 DataScope 最大问题

代码中已经明确写着：

> 当前数据权限由业务 service 显式接入；系统管理、日志、文件、任务等治理模块不默认套用。

**来源：**

- `internal/service/access/data_scope_filter.go`

这意味着：

```text
新建业务模块
     │
     ├── 开发者记得调用 DataScope
     │        ↓
     │      正常
     │
     └── 忘记调用
              ↓
            越权风险
```

这是当前权限体系最值得优先解决的问题。

## 建议：DataScope 从“显式加入”变成“默认启用”

目标原则：

> **需要数据权限的业务查询默认受保护，只有明确声明才能绕过。**

推荐逐步引入：

```go
db := repository.WithContext(ctx)

db = access.ApplyDataScope(
    ctx,
    db,
    access.Scope{
        DepartmentColumn: "department_id",
        OwnerColumn:      "created_by",
    },
)
```

进一步可以抽象成：

```go
repo.List(
    ctx,
    access.DepartmentScope("department_id"),
    access.OwnerScope("created_by"),
)
```

对于明确不需要 DataScope 的治理模块：

```go
access.WithoutDataScope(ctx)
```

但必须显式声明。

这样安全模型由：

```text
忘了加权限 = 越权
```

变成：

```text
忘了特殊处理 = 仍然受权限限制
```

## 参考来源

GoWind Admin README 描述的 Tenant 隔离使用 Ent Privacy 自动注入读取过滤，并对 Create/Update/Delete 同样做跨租户限制。

参考：

- <https://github.com/tx7do/go-wind-admin/blob/main/README.md>

这里参考的是“**默认安全 / 自动注入**”思想，而不是要求 gin-layout 必须换 Ent。

---

# 7. 增加 Permission Code：把业务权限与 HTTP API 解耦

当前很多后台系统天然形成：

```text
Role
 ↓
Menu
 ↓
API
```

但随着业务复杂度上升，直接把 HTTP API 当作业务权限会越来越难维护。

例如业务权限：

```text
order.refund
```

可能涉及：

```text
POST /orders/:id/refund
POST /payments/:id/refund
POST /refund-records
```

建议新增稳定的 **Permission Code**：

```text
product.view
product.create
product.update
product.delete

order.view
order.export
order.refund

inventory.adjust
user.disable
```

推荐关系：

```text
Role
  ↓
Permission
  ↓
API
```

菜单按钮也绑定 Permission：

```text
Menu/Button
      ↓
Permission
```

最终：

```text
        Role
         │
         ▼
    Permission
      /      \
     ▼        ▼
    API     Menu/Button
```

### 好处

1. HTTP URL 改版不会直接破坏角色权限。
2. Web、APP、RPC、Agent 可以共享业务权限。
3. Casbin Policy 更容易表达稳定资源。
4. 审计日志可以直接记录 `order.refund`。
5. 前端按钮权限不必绑定具体 URL。

## 参考来源

Gin-Vue-Admin 当前明确区分角色的 API 权限和菜单权限：

- <https://github.com/flipped-aurora/gin-vue-admin/blob/main/README.md>

GoWind Admin进一步明确存在权限点以及菜单/按钮权限标识：

- <https://github.com/tx7do/go-wind-admin/blob/main/README.md>

这里的 Permission Code 是结合这些实践后对 gin-layout 的进一步抽象建议。

---

# 8. 为未来 Agent 预留统一授权模型

如果未来 ERP 中加入 AI Agent：

```text
用户：
“帮我把订单 123 退款”
```

Agent Tool：

```text
refund_order
```

不能只判断：

```text
用户能访问 /agent/chat
```

而必须继续经过业务权限：

```text
refund_order
     │
     ▼
order.refund
     │
     ▼
RBAC
     │
     ▼
DataScope
     │
     ▼
Resource Policy
```

推荐未来形成：

```text
HTTP API ─┐
RPC       ├── Permission Code ── RBAC
Agent Tool┘
```

这是 `gin-layout` 以后支持智能体的一个很好的差异化方向。

---

# 9. RBAC + DataScope + ABAC 三层权限模型

后续不要试图用 Casbin RBAC 解决所有业务授权。

建议明确三层：

## RBAC

回答：

> 用户有没有执行某类操作的资格？

例如：

```text
order.refund
inventory.adjust
```

## DataScope

回答：

> 用户可以看到哪些记录？

例如：

```text
全部
本部门
本部门及下级
本人
自定义部门
```

## ABAC / Resource Policy

回答：

> 针对“这一条具体资源”，当前是否允许操作？

例如：

```text
Order.MerchantID == User.MerchantID
AND
Order.Status == "paid"
AND
RefundAmount <= Order.PayAmount
```

最终：

```text
Request
   │
   ▼
Permission / RBAC
   │
   ▼
Data Scope
   │
   ▼
Resource Policy / ABAC
   │
   ▼
Business Action
```

不需要一开始就引入复杂 Policy Engine，可以先定义 Go 接口：

```go
type Policy[T any] interface {
    CanView(ctx context.Context, resource T) bool
    CanUpdate(ctx context.Context, resource T) bool
    CanDelete(ctx context.Context, resource T) bool
}
```

---

# 10. 多租户是未来 SaaS / ERP 最大架构缺口之一

如果项目希望支持：

```text
SaaS ERP
多商户
多企业
多门店
```

应该尽早定义 Tenant，而不是简单在所有表中增加 `tenant_id`。

建议概念拆分：

```text
Platform
   │
   ▼
Tenant
   │
   ├── Merchant / Company
   │       │
   │       ├── Store
   │       └── Warehouse
   │
   └── User
           │
           └── Department
```

需要明确：

- Tenant：SaaS 隔离边界
- Merchant：业务主体
- Store：经营门店
- Department：内部组织结构

不要将四者混为一个概念。

---

# 11. Tenant 必须默认自动隔离

最危险的实现是：

```go
db.Where("tenant_id = ?", tenantID)
```

由业务开发者每次手写。

应该最终形成：

```text
Request
   │
Tenant Resolver
   │
Context
   │
Repository
   │
Tenant Scope
   │
Data Scope
   │
Business Query
```

统一 Query Pipeline：

```text
Tenant Scope
     ↓
Data Scope
     ↓
Soft Delete
     ↓
Business Condition
     ↓
SQL
```

并覆盖：

- SELECT
- CREATE
- UPDATE
- DELETE

### 特别注意

Create 时需要防：

```json
{
  "tenant_id": 999
}
```

由客户端伪造 Tenant。

应由 Context 自动赋值。

Update/Delete 也必须附加 Tenant 条件。

## 参考来源

GoWind Admin README 描述的多租户方案：

- 查询自动注入 Tenant Filter
- Create 防伪造租户
- Update/Delete 注入 Tenant Predicate
- 跨租户操作命中 0 行

来源：

- <https://github.com/tx7do/go-wind-admin/blob/main/README.md>

---

# 12. 当前目录结构问题

当前主要结构是：

```text
internal/
├── access
├── controller
├── service
├── model
├── validator
├── resources
├── routers
├── middleware
├── queue
├── cron
├── jobs
├── filestorage
├── runtime
└── ...
```

**来源：**

- <https://github.com/wannanbigpig/gin-layout/tree/master/internal>

这在项目当前规模下没有问题。

但业务模块达到几十个后，会逐渐变成：

```text
controller/
    product.go
    order.go
    inventory.go
    customer.go
    supplier.go
    ...

service/
    product.go
    order.go
    inventory.go
    ...

model/
    product.go
    order.go
    inventory.go
    ...
```

这属于典型的“横向分层目录膨胀”。

---

# 13. 建议逐步演进到 Module First

不建议进行一次大爆炸式重构。

可以从新增模块开始采用：

```text
internal/
├── modules/
│   ├── system/
│   │   ├── user/
│   │   ├── role/
│   │   ├── menu/
│   │   └── department/
│   │
│   ├── product/
│   │   ├── model.go
│   │   ├── repository.go
│   │   ├── service.go
│   │   ├── handler.go
│   │   ├── request.go
│   │   ├── resource.go
│   │   └── routes.go
│   │
│   ├── order/
│   └── inventory/
│
├── platform/
│   ├── auth/
│   ├── access/
│   ├── database/
│   ├── cache/
│   ├── queue/
│   ├── storage/
│   ├── tracing/
│   └── config/
│
└── bootstrap/
```

核心原则：

```text
modules  = 业务能力
platform = 通用基础设施
```

这样未来某个模块拆为独立服务时边界已经存在。

---

# 14. 不建议当前微服务化

当前 `gin-layout` 已经支持：

```text
service
worker
cron
```

这是非常适合中大型单体应用的运行模型。

建议未来 1.x 之前坚持：

> **Modular Monolith First**

不要因为参考 Simple Admin / GoWind 就立即加入：

- 服务注册
- 服务发现
- gRPC
- Kafka
- 分布式事务
- 大规模 Protobuf
- 服务网格

这些能力只有业务真正需要时再引入。

## 参考

Simple Admin明确走 go-zero 分布式路线：

- <https://github.com/suyuan32/simple-admin-core/blob/main/README.md>

GoWind Admin基于 Kratos，但专门提供单体模式：

- <https://github.com/tx7do/go-wind-admin/blob/main/README.md>

对 gin-layout 更合适的做法是：

> 吸收“边界明确”的思想，不照搬微服务复杂度。

---

# 15. 引入 Service Boundary

即使不拆微服务，也建议避免业务模块直接访问其他模块 Model。

例如不要：

```go
model.NewOrder().Find(...)
```

被 Product、Inventory、Agent 到处调用。

推荐：

```go
type OrderService interface {
    Create(ctx context.Context, req CreateOrder) (*Order, error)
    Cancel(ctx context.Context, id uint) error
    Get(ctx context.Context, id uint) (*Order, error)
}
```

其他模块只能依赖这个 Boundary。

这样未来：

```text
Local Service
     ↓
Interface
     ↓
RPC Client
```

替换成本非常低。

---

# 16. Repository 层建议正式引入

当前 Model 仍包含数据库操作，例如 `Role` 中有：

```text
EnabledIdsByIds
AllIds
FindByCode
```

**来源：**

- `internal/model/role.go`
- <https://github.com/wannanbigpig/gin-layout/blob/master/internal/model/role.go>

当前相当于：

```text
Model = Entity + DAO
```

对于脚手架早期完全可接受。

但如果目标是长期维护的企业级 Foundation，建议变成：

```text
Model       = 数据结构
Repository  = 持久化
Service     = 业务
Handler     = HTTP
```

示例：

```go
type RoleRepository interface {
    FindByID(ctx context.Context, id uint) (*Role, error)
    FindByCode(ctx context.Context, code string) (*Role, error)
    List(ctx context.Context, query RoleQuery) ([]Role, error)
    Save(ctx context.Context, role *Role) error
}
```

### 收益

- Service 单元测试更简单
- 数据库操作集中
- Tenant / DataScope 可统一放进 Repository
- Transaction 边界更清楚
- Model 不再绑定 GORM 查询实现

---

# 17. 建议加入 Transaction Manager

业务系统迟早出现：

```text
创建订单
  ↓
扣库存
  ↓
写支付单
  ↓
写操作日志
```

建议统一事务：

```go
type TransactionManager interface {
    WithinTransaction(
        ctx context.Context,
        fn func(ctx context.Context) error,
    ) error
}
```

Service 不直接感知全局 DB：

```go
txManager.WithinTransaction(ctx, func(ctx context.Context) error {
    ...
})
```

以后 Repository 从 Context 获取当前 Transaction。

---

# 18. 审计系统需要从 Request Log 升级

当前已有：

- 登录日志
- 请求日志
- Task 运行记录

下一步建议新增真正的：

```text
Audit Event
```

结构例如：

```json
{
  "actor_id": 123,
  "tenant_id": 10,
  "action": "order.refund",
  "resource_type": "order",
  "resource_id": "9876",
  "before": {
    "status": "paid"
  },
  "after": {
    "status": "refunded"
  },
  "ip": "127.0.0.1",
  "trace_id": "abc123"
}
```

重点记录：

```text
WHO
WHEN
WHERE
ACTION
RESOURCE
BEFORE
AFTER
TRACE
```

尤其需要覆盖：

- 登录
- 权限变更
- 角色变更
- Tenant 变更
- 数据导出
- 删除
- 退款
- 库存调整
- 安全配置变更

## 参考

GoWind Admin README明确把登录、操作、API、数据访问、权限变更、策略评估拆成六类审计日志，并记录 `trace_id`。

来源：

- <https://github.com/tx7do/go-wind-admin/blob/main/README.md>

---

# 19. Trace ID / Request ID

建议从现在开始统一：

```text
HTTP
 │
 ▼
request_id / trace_id
 │
 ├── Logger
 ├── SQL
 ├── Redis
 ├── Audit
 ├── Queue Payload
 ├── Worker
 ├── Cron
 └── Agent Tool
```

例如：

```text
API Request
trace_id = abc

   ↓ enqueue

Task
trace_id = abc

   ↓

Worker
trace_id = abc
```

未来排查订单、库存、异步任务问题会非常有价值。

---

# 20. Observability

建议逐步加入：

## Metrics

提供：

```text
/metrics
```

典型指标：

```text
http_requests_total
http_request_duration_seconds

db_query_duration_seconds

queue_pending_total
queue_failed_total
task_duration_seconds

cron_failed_total

websocket_connections
```

## Tracing

未来可选：

- OpenTelemetry
- Jaeger
- Tempo

但不要把具体 Backend 写死。

## 参考

GoWind Admin README已经将 Jaeger / 分布式链路追踪列为生产能力。

来源：

- <https://github.com/tx7do/go-wind-admin/blob/main/README.md>

---

# 21. Storage Driver

当前项目已经存在 `filestorage`，并依赖阿里云 OSS SDK。

**来源：**

- `internal/filestorage`
- `go.mod`
- <https://github.com/wannanbigpig/gin-layout/blob/master/go.mod>

建议进一步稳定成接口：

```go
type Storage interface {
    Put(ctx context.Context, ...)
    Delete(ctx context.Context, ...)
    Exists(ctx context.Context, ...)
    URL(ctx context.Context, ...)
    SignedURL(ctx context.Context, ...)
}
```

官方维护：

- Local
- Aliyun OSS
- S3 Compatible

社区可以贡献：

- MinIO
- Tencent COS
- Qiniu
- Azure Blob

这种 Driver 是非常好的 `good first issue`。

---

# 22. Module / Extension 机制

不要急着实现复杂插件市场。

先定义轻量：

```go
type Module interface {
    Name() string
    Routes() []Route
    Migrations() []Migration
    Commands() []Command
    Jobs() []Job
}
```

注册：

```go
modules.Register(
    system.New(),
    product.New(),
    example.New(),
)
```

未来：

```text
Plugin ≈ Module + Manifest
```

即可继续演进。

## 参考

Simple Admin把文件、定时任务、会员、消息中心拆分为独立模块。

来源：

- <https://github.com/suyuan32/simple-admin-core/blob/main/README.md>

GoAdmin则长期维护 Theme / Plugin 扩展生态。

来源：

- <https://github.com/GoAdminGroup/go-admin/blob/main/CONTRIBUTING.md>

---

# 23. Generator 应成为 gin-layout 核心竞争力

当前已经支持：

```bash
go run main.go command gen --from=gen/specs/product.yaml
```

并且生成模板已经接入 DataScope。

**来源：**

- `internal/console/gen`
- `internal/console/gen/templates/service.go.tmpl`
- `README.md`

建议升级成：

```bash
gin-layout gen module product
```

生成：

```text
modules/product/
├── model.go
├── repository.go
├── service.go
├── handler.go
├── request.go
├── resource.go
├── routes.go
├── permission.go
├── migration/
└── *_test.go
```

同时生成：

- Permission
- Route Metadata
- DataScope 接入
- Migration
- API Schema
- Unit Test Skeleton

最终让：

> “30 秒创建一个符合项目规范的业务模块”

成为最核心卖点之一。

## 参考

Gin-Vue-Admin强调一分钟生成基础 CRUD，并自动分配权限。

来源：

- <https://github.com/flipped-aurora/gin-vue-admin/blob/main/README.md>

Simple Admin明确提供 Web/API/RPC 三端代码生成。

来源：

- <https://github.com/suyuan32/simple-admin-core/blob/main/README.md>

---

# 24. OpenAPI / API Contract

当前主要通过 Apifox 提供文档。

建议未来进一步做到：

```text
代码 / Schema
     │
     ▼
OpenAPI 3.x
     │
     ├── Swagger UI
     ├── TypeScript SDK
     ├── Go Client
     └── Agent Tool Schema
```

这样后续：

```text
API Contract
     ↓
Front-end SDK
     ↓
Agent Tool
```

共用一套契约。

## 参考

Gin-Vue-Admin已经通过 Swagger 自动生成接口文档：

- <https://github.com/flipped-aurora/gin-vue-admin/blob/main/README.md>

Simple Admin也将 go-swagger 作为核心能力：

- <https://github.com/suyuan32/simple-admin-core/blob/main/README.md>

---

# 25. 安全体系建议

当前已经具备：

- CodeQL
- Go CI
- JWT
- Casbin
- 双 Token
- Redis 黑名单/缓存
- fail-closed DataScope

**来源：**

- `.github/workflows/codeql.yml`
- `.github/workflows/go.yml`
- `README.md`

建议继续加入：

```text
govulncheck
gosec
dependency review
Dependabot
secret scan
```

并增加：

```text
SECURITY.md
```

安全漏洞应提供私下报告方式，而不是要求用户创建公开 Issue。

---

# 26. 测试策略

当前已经存在：

- Role 测试
- AdminUser 测试
- DataScopeFilter 测试

**来源：**

- `internal/service/role/role_test.go`
- `internal/service/admin/admin_user_test.go`
- `internal/service/access/data_scope_filter_test.go`

建议不要追求全仓 100% Coverage。

优先级：

```text
Auth                 90%+
Permission/RBAC      90%+
DataScope            95%+
Tenant Isolation     95%+
Migration            90%+
Generator            Golden Tests
```

普通 CRUD 可以维持更合理的覆盖率。

尤其必须测试：

```text
无 Role
禁用 Role
多 Role 并集
ALL Scope
Self Scope
Dept Scope
Custom Scope
缓存失效
Redis 不可用
DB 查询失败
跨 Tenant SELECT
跨 Tenant UPDATE
跨 Tenant DELETE
伪造 Tenant CREATE
```

---

# 27. CI Gate

当前已有：

- Go Workflow
- CodeQL

**来源：**

- `.github/workflows/go.yml`
- `.github/workflows/codeql.yml`

未来 PR Gate 推荐：

```text
gofmt check
go vet
golangci-lint
go test ./...
go test -race ./...
govulncheck
CodeQL
migration check
generator golden test
```

后期增加：

```text
MySQL Integration Test
Redis Integration Test
```

---

# 28. 当前开源治理问题

当前 `.github` 主要有：

```text
pull_request_template.md
workflows/
```

**来源：**

- `.github/`
- <https://github.com/wannanbigpig/gin-layout/tree/master/.github>

对于准备正式做社区来说还不够。

建议增加：

```text
CONTRIBUTING.md
CODE_OF_CONDUCT.md
SECURITY.md
CHANGELOG.md
ROADMAP.md
```

以及：

```text
.github/
├── ISSUE_TEMPLATE/
│   ├── bug_report.yml
│   ├── feature_request.yml
│   └── config.yml
├── pull_request_template.md
├── dependabot.yml
└── workflows/
```

---

# 29. CONTRIBUTING.md 应写什么

至少写清楚：

1. 如何启动项目
2. 如何启动 MySQL / Redis
3. 如何执行 Migration
4. 如何执行测试
5. 如何新增 Module
6. 如何新增 Route
7. 如何新增 Permission
8. 如何新增 DataScope 业务模块
9. Commit 格式
10. PR 格式
11. Breaking Change 如何讨论
12. 大功能先 Issue / Discussion

## 参考

GoAdmin CONTRIBUTING 明确区分：

- trivial fix：直接 PR
- involved change：先 Issue 讨论
- 提交前执行 `make test`

来源：

- <https://github.com/GoAdminGroup/go-admin/blob/main/CONTRIBUTING.md>

Gin-Vue-Admin README也直接提供 Issue 与 PR 规范：

- <https://github.com/flipped-aurora/gin-vue-admin/blob/main/README.md>

---

# 30. 社区贡献入口

不能只写：

> Welcome PR

建议主动准备：

```text
good first issue
help wanted
documentation
plugin
storage
translation
security
performance
```

适合新贡献者的任务：

- MinIO Storage Driver
- Tencent COS Driver
- PostgreSQL CI
- 新语言翻译
- Redis Health Check
- Docs 示例
- Generator Template Test
- OpenAPI 文档
- Notification Driver

## 参考

GoAdmin CONTRIBUTING 主动列出：

- Translation
- Bug Hunting
- Security Problems
- Themes
- Plugins

来源：

- <https://github.com/GoAdminGroup/go-admin/blob/main/CONTRIBUTING.md>

---

# 31. README 应从“说明书”升级成“项目首页”

推荐结构：

```text
Logo

一句话定位

Badges

Demo

Screenshot

Why gin-layout

Features

Architecture

Quick Start

Documentation

Roadmap

Contributing

Community

License
```

尤其需要：

## Why gin-layout

告诉用户为什么不用：

- Gin-Vue-Admin
- go-admin
- Kratos
- go-zero

建议强调：

```text
轻量但工程化
模块化单体优先
RBAC + DataScope
声明式路由
API / Worker / Cron
Task Center
Migration
Generator
生产友好
不过度绑定业务
```

---

# 32. 明确 Non-goals

建议 README 新增：

## Non-goals

```text
Gin Layout 不计划：

- 成为完整 ERP 成品
- 内置完整商城
- 内置 CRM / OA 全套业务
- 强制用户采用微服务
- 为所有业务提供“标准实现”
```

核心目标：

> 提供稳定的后台业务基础设施和扩展约定。

这能够避免项目逐渐变成“大杂烩”。

---

# 33. Docs Site

README 不应承载所有文档。

推荐：

```text
docs/
├── guide/
├── architecture/
├── development/
├── security/
└── contributing/
```

使用 VitePress / Docusaurus 均可。

文档结构建议：

```text
Guide
├── Introduction
├── Installation
├── Configuration
└── Deployment

Architecture
├── Overview
├── Authentication
├── Authorization
├── DataScope
├── MultiTenant
├── Task System
└── Storage

Development
├── Create Module
├── Migration
├── Generator
├── Queue
├── Cron
└── Testing

Contribution
├── Setup
├── Coding Style
└── PR Guide
```

---

# 34. 当前 Release 管理问题

GitHub Releases 当前可以看到旧的：

```text
v0.2.0
V0.2.0
```

状态仍然是：

```text
draft = true
prerelease = true
```

并且创建时间在 2022 年，target branch 为旧的 `basic_layout`。

来源：

- <https://github.com/wannanbigpig/gin-layout/releases>

这意味着目前实际上没有形成稳定的 Release 体系。

---

# 35. 版本策略：Semantic Versioning

正式采用：

```text
MAJOR.MINOR.PATCH
```

例如：

```text
v0.5.0
v0.5.1
v0.6.0
v1.0.0
```

规则：

```text
PATCH
兼容性 Bug Fix

MINOR
兼容性新功能

MAJOR
Breaking Change
```

在 v1.0 前：

```text
v0.x
```

允许相对频繁调整内部 API。

不要过早发布 v1.0。

---

# 36. 推荐版本路线

## v0.5 — Access Foundation

目标：

> 权限模型收口。

建议完成：

- Permission Code
- Role → Permission
- Permission → API
- Permission → Menu/Button
- DataScope 默认安全机制
- 权限缓存失效矩阵
- Super Admin 明确定义
- 权限单元测试矩阵
- Agent Tool Permission 预留

---

## v0.6 — Multi-Tenant

目标：

> SaaS Ready。

建议完成：

- Tenant Model
- Tenant Resolver
- Tenant Context
- Tenant Scope
- Create 防 Tenant 伪造
- Update/Delete Tenant Predicate
- Tenant Admin
- Tenant Config
- Tenant Test Matrix
- Cross-Tenant Security Test

---

## v0.7 — Modular Architecture

目标：

> 模块边界稳定。

建议完成：

- `modules/`
- `platform/`
- Repository
- Transaction Manager
- Service Boundary
- Module Interface
- Gradual Migration Guide

---

## v0.8 — Observability & Audit

建议完成：

- Request ID
- Trace ID
- Audit Event
- Metrics
- Prometheus
- OpenTelemetry Hook
- Queue Metrics
- Cron Metrics
- Audit Retention

---

## v0.9 — Developer Experience

建议完成：

- Module Generator
- Repository Generator
- Permission Generator
- Migration Generator
- Test Skeleton
- OpenAPI
- TypeScript SDK
- Extension / Driver Interface
- Docs Site
- Example Project

---

## v1.0 — Stable Foundation

v1.0 应意味着：

- 稳定配置结构
- 稳定目录约定
- 稳定 Module API
- 稳定 Permission API
- 稳定 Migration 策略
- 完整升级指南
- 完整文档
- 完整安全策略
- 正式 Release Automation

---

# 37. CHANGELOG

建议采用：

> Keep a Changelog + Semantic Versioning

格式：

```markdown
## [0.6.0] - 2026-XX-XX

### Added

- Multi-tenant support
- Tenant scoped repository

### Changed

- DataScope is enabled by default

### Fixed

- Permission cache invalidation

### Security

- Prevent cross-tenant query leakage

### Breaking

- Repository API changed
```

---

# 38. Commit 规范

推荐 Conventional Commits：

```text
feat(auth): add refresh token rotation

feat(access): add permission code

fix(rbac): invalidate policy cache after role update

fix(tenant): prevent cross tenant update

refactor(access): move data scope into repository

docs: add architecture guide

test(access): cover custom scope union

chore(deps): upgrade gin
```

这样可以用于自动：

- Release Notes
- Changelog
- Version Bump

---

# 39. Branch 模型

不建议复杂 Git Flow。

推荐：

```text
master/main
    ↑
    PR
```

短生命周期分支：

```text
feat/multi-tenant
feat/permission-code
fix/token-refresh
docs/data-scope
```

所有正式版本通过 Tag：

```text
v0.5.0
v0.6.0
```

触发 Release Workflow。

---

# 40. Release Automation

后续可以引入：

- GoReleaser
- GitHub Actions Release
- Conventional Commits
- 自动生成 Release Notes

Tag：

```text
v0.6.0
```

自动：

```text
Test
  ↓
Build
  ↓
Security Scan
  ↓
Generate Changelog
  ↓
Create GitHub Release
  ↓
Upload Binary
  ↓
Build Docker Image
```

---

# 41. 社区建设重点不是宣传，而是降低贡献成本

理想体验：

## 10 分钟

新用户可以完成：

```bash
git clone
docker compose up
make migrate
make dev
```

## 30 分钟

新贡献者可以完成：

```text
创建一个 Module
新增一个 API
增加测试
提交 PR
```

社区活跃度很大程度取决于这里，而不是单纯 Star 数。

---

# 42. 建议建立 Extension Points

优先设计低风险扩展点：

```text
Storage Driver
Notification Driver
Module
Middleware
Validator
Translator
Task Handler
Audit Sink
Metrics Exporter
```

让社区贡献者不需要修改核心权限代码也能产生价值。

---

# 43. 项目差异化定位

不建议宣传：

```text
用户管理
角色管理
菜单管理
字典管理
```

这些几乎所有 Admin 项目都有。

建议宣传：

```text
Declarative Routing

RBAC + DataScope

Fail-Closed Permission

Tenant Isolation

Task Center

API / Worker / Cron

CLI & Generator

Migration First

Modular Monolith

Production Ready

Agent-ready Permission Model
```

推荐英文定位：

> **Production-ready Gin backend foundation for admin panels, SaaS and enterprise applications.**

推荐中文定位：

> **面向中后台、SaaS 与企业业务系统的生产级 Gin 后端基础框架。**

---

# 44. 推荐开发优先级

按照风险和收益排序：

| Priority | 项目 | 原因 |
|---|---|---|
| P0 | DataScope 默认安全化 | 当前最大的潜在权限遗漏风险 |
| P0 | Permission Code | 稳定业务权限模型 |
| P0 | 权限缓存失效测试 | 防止权限变更延迟 |
| P1 | Repository | 为 Tenant / DataScope 统一处理奠定基础 |
| P1 | Multi-Tenant | SaaS / ERP 的核心基础 |
| P1 | Audit | 企业系统必备 |
| P1 | Trace ID | 生产排障基础 |
| P1 | CONTRIBUTING / SECURITY | 正式开源必备 |
| P2 | Module Architecture | 控制项目继续膨胀 |
| P2 | Generator 2.0 | 提升项目差异化 |
| P2 | OpenAPI | 前端 / Agent 契约统一 |
| P2 | Metrics / OTel | Production Ready |
| P2 | Docs Site | 降低采用成本 |
| P3 | Plugin / Module Ecosystem | 社区规模扩大后推进 |
| P3 | 微服务 | 真正出现组织/性能需求后再拆 |

---

# 45. 推荐的最终架构

```text
                       gin-layout

┌─────────────────────────────────────────────────────┐
│                    Entry Points                     │
│                                                     │
│      HTTP       WebSocket      CLI     Worker Cron  │
└─────────────────────────┬───────────────────────────┘
                          │
                          ▼
┌─────────────────────────────────────────────────────┐
│                     Platform                        │
│                                                     │
│ Auth │ Permission │ Tenant │ DataScope │ Audit      │
│                                                     │
│ DB │ Redis │ Queue │ Storage │ Trace │ Metrics      │
└─────────────────────────┬───────────────────────────┘
                          │
                          ▼
┌─────────────────────────────────────────────────────┐
│                     Modules                         │
│                                                     │
│ System │ Product │ Order │ Inventory │ Merchant ... │
└─────────────────────────┬───────────────────────────┘
                          │
                          ▼
┌─────────────────────────────────────────────────────┐
│                  Infrastructure                     │
│                                                     │
│ MySQL │ Redis │ OSS/S3 │ Prometheus │ OpenTelemetry │
└─────────────────────────────────────────────────────┘
```

授权链路：

```text
Request
   │
   ▼
Authentication
   │
   ▼
Permission / RBAC
   │
   ▼
Tenant Isolation
   │
   ▼
Data Scope
   │
   ▼
Resource Policy / ABAC
   │
   ▼
Business Service
   │
   ▼
Audit
```

---

# 46. 不建议做的事情

短期不建议：

1. 为了“架构先进”立刻改成微服务。
2. 为了模仿其他 Admin 项目加入大量业务模块。
3. 为了 DDD 而引入过多抽象层。
4. 一次性重写整个现有目录。
5. 把所有授权都塞给 Casbin。
6. Tenant 依赖开发者手写 `WHERE tenant_id = ?`。
7. DataScope 继续永久依赖开发者手动记得调用。
8. 在权限模型没稳定前发布 v1.0。
9. 只追求 Star，不投资文档、测试和贡献体验。

---

# 47. 建议未来半年执行顺序

## 第一阶段：权限稳定

```text
Permission Code
DataScope Safe-by-default
Cache Invalidation
Access Tests
```

## 第二阶段：持久层与 Tenant

```text
Repository
Transaction Manager
Tenant Context
Tenant Scope
Tenant Security Tests
```

## 第三阶段：工程生产化

```text
Audit
Trace ID
Metrics
OpenTelemetry
Security Tooling
```

## 第四阶段：开发者体验

```text
Module Generator
OpenAPI
Docs Site
Example
```

## 第五阶段：社区与 Release

```text
CONTRIBUTING
SECURITY
ROADMAP
CHANGELOG
Issue Template
Good First Issue
Release Automation
```

然后：

```text
v0.9
  ↓
API Freeze
  ↓
RC
  ↓
v1.0
```

---

# 48. 来源索引

## gin-layout 自身

### 项目说明

- <https://github.com/wannanbigpig/gin-layout>
- <https://github.com/wannanbigpig/gin-layout/blob/master/README.md>

### Go 依赖

- <https://github.com/wannanbigpig/gin-layout/blob/master/go.mod>

### Role / DataScope

- <https://github.com/wannanbigpig/gin-layout/blob/master/internal/model/role.go>
- <https://github.com/wannanbigpig/gin-layout/blob/master/internal/model/role_data_scope_dept.go>

### 用户 / 部门 / Role Mapping

- <https://github.com/wannanbigpig/gin-layout/blob/master/internal/model/admin_user_role_map.go>
- <https://github.com/wannanbigpig/gin-layout/blob/master/internal/model/dept_role_map.go>
- <https://github.com/wannanbigpig/gin-layout/blob/master/internal/model/role_menu_map.go>

### DataScopeFilter

- <https://github.com/wannanbigpig/gin-layout/blob/master/internal/service/access/data_scope_filter.go>

### Generator

- <https://github.com/wannanbigpig/gin-layout/tree/master/internal/console/gen>

### Internal 架构

- <https://github.com/wannanbigpig/gin-layout/tree/master/internal>

### CI

- <https://github.com/wannanbigpig/gin-layout/tree/master/.github/workflows>

### Release

- <https://github.com/wannanbigpig/gin-layout/releases>

---

## Gin-Vue-Admin

仓库：

- <https://github.com/flipped-aurora/gin-vue-admin>

主要参考：

- RBAC
- API / Menu 权限
- Swagger
- Generator
- README / Contribution
- Docs / Demo / Ecosystem

直接来源：

- <https://github.com/flipped-aurora/gin-vue-admin/blob/main/README.md>

---

## Simple Admin

仓库：

- <https://github.com/suyuan32/simple-admin-core>

主要参考：

- 模块拆分
- 多租户版本
- Web/API/RPC Generator
- 社区模块
- 分布式架构的边界设计

直接来源：

- <https://github.com/suyuan32/simple-admin-core/blob/main/README.md>

---

## GoWind Admin

仓库：

- <https://github.com/tx7do/go-wind-admin>

主要参考：

- Tenant Isolation
- Row-level Permission
- Field-level Permission
- Audit
- Trace
- Observability
- 单体 / 微服务双模式

直接来源：

- <https://github.com/tx7do/go-wind-admin/blob/main/README.md>

---

## GoAdmin

仓库：

- <https://github.com/GoAdminGroup/go-admin>

主要参考：

- Contribution Workflow
- Theme / Plugin Contribution
- Test Before PR
- 社区任务入口

直接来源：

- <https://github.com/GoAdminGroup/go-admin/blob/main/CONTRIBUTING.md>

---

# 49. 最终建议

`gin-layout` 下一阶段最值得建立的核心认知是：

> **它不是后台 Demo，而是后台业务系统的 Foundation。**

因此未来衡量一个新功能是否应该进入核心仓库，可以问三个问题：

```text
1. 大多数后台系统是否都需要它？

2. 它是否属于基础设施，而不是具体业务？

3. 它能否形成稳定的扩展接口，让社区在核心之外继续扩展？
```

三个答案大部分都是“是”，才建议进入 Core。

如果答案是：

```text
商城订单
CRM 客户
审批工作流
ERP 采购
直播
IM
```

这些更适合：

```text
Example
Community Module
Extension
Independent Repository
```

而不是继续塞入 gin-layout Core。

长期来看，项目最值得建立的技术标签不是：

> “功能最多的 Golang 管理后台”

而是：

> **“一个权限可靠、默认安全、工程化强、模块边界清晰、适合长期维护的 Gin 企业后端基础框架。”**

这会比单纯复制成熟 Admin 项目的功能列表更有辨识度，也更符合 gin-layout 当前已经形成的技术方向。
