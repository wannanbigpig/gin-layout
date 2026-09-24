# tests/admin_test 索引

管理端路由以 `internal/routers/admin_router.go` 为真相。当前非 Release 模式有 `138` 个 `/admin/v1` 接口，Release 模式不含 `GET /admin/v1/demo`，共 `137` 个。

## 全路由覆盖机制

`route_coverage_test.go` 从 `routers.AppRouteTree()` 动态收集路由：

- `131` 个受保护接口全部执行匿名请求并断言 `401`。
- 所有受保护 GET 和写接口再使用真实认证进入参数或业务层；写接口使用不会污染业务数据的校验载荷，少数无必填字段接口使用隔离条件。
- `7` 个公开管理端接口维护显式状态场景；新增公开接口没有场景时测试失败。

这保证每个声明接口都有真实请求入口，但不把鉴权覆盖等同于业务成功。业务写接口继续按领域补充成功、校验和状态流。

`route_scenarios_test.go` 对全部 `138` 个路由进行第二层约束：`99` 个业务流程、`25` 个契约边界、`14` 个外部依赖场景。分类必须与路由树一一对应，新增、删除或重复分类都会失败；契约和外部依赖分类也是后续补齐成功流程的明确清单。

## 分组与测试文件

| 路由分组 | 数量 | 主要测试文件 | 业务覆盖重点 |
| --- | ---: | --- | --- |
| other / login / auth_public / common_ws | 7 | `public_routes_test.go`、`auth_routes_test.go`、`common_routes_test.go` | demo、文件公开访问、验证码、登录/刷新/登出、WS ticket |
| dashboard | 4 | `dashboard_test.go` | personal、overview、statistics、monitor 成功路径与权限边界 |
| todo | 6 | `todo_test.go` | 当前用户隔离、手工待办 CRUD、摘要与业务待办只读边界 |
| common / auth / session | 5 | `common_routes_test.go`、`auth_routes_test.go`、`session_test.go` | 上传校验、token、真实会话列表和撤销 |
| adminUser | 13 | `admin_user_test.go` | 详情、options、CRUD、资料和角色绑定 |
| permission | 3 | `permission_routes_test.go` | 列表、options、更新校验 |
| menu | 6 | `menu_test.go` | 完整 CRUD、权限同步、i18n Replace 语义 |
| role | 6 | `role_test.go` | options、完整 CRUD 和菜单授权 |
| department | 6 | `department_test.go` | 完整 CRUD 和角色绑定 |
| product | 5 | `product_test.go` | 使用真实部门前置的完整 CRUD |
| sysConfig | 7 | `system_routes_test.go` | 完整 CRUD、缓存刷新、可见性和 i18n PATCH 语义 |
| sysDict | 10 | `system_routes_test.go` | 类型/字典项完整 CRUD、options 和 i18n PATCH 语义 |
| file | 28 | `file_resource_test.go` | 目录生命周期、列表/回收站/引用、上传、导出和对象状态接口校验 |
| export | 2 | `log_routes_test.go` | 列表和详情 NotFound |
| notification | 5 | `notification_test.go` | 指定用户发送、列表、未读数、单条/全部已读 |
| storage | 4 | `storage_test.go`、`system_routes_test.go` | 配置读取/保存、本地存储连通性、敏感字段校验 |
| log | 9 | `log_routes_test.go` | 全局与个人请求日志、登录日志、详情边界、导出、脱敏配置 |
| task | 12 | `task_routes_test.go` | 定义/执行/统计/趋势/cron 查询和操作参数校验 |

合计 `138` 个管理端接口。

## MySQL 与前置数据

- `TestMain` 使用测试专用配置（`config.test.yaml` 或 `GOTEST_CONFIG`）初始化真实 MySQL；配置缺失、MySQL 未启用或不可连接时整个测试包失败。不会读取业务 `config.yaml`。
- 默认认证主体是数据库中的 `ID=1` 超级管理员；测试库必须先完成 migration 和系统初始数据。
- product 流程会读取一个真实部门作为前置；session、notification、folder 流程创建专属记录并在结束时清理。
- 所有写入数据使用唯一名称或 marker，测试不能依赖运行顺序。

## 外部依赖边界

- Redis 启用时测试 WS ticket 到 Upgrade 校验；禁用时必须返回 `ServiceDependencyNotReady`。
- queue 未在测试 Router 初始化时，导出接口允许返回 `ServiceDependencyNotReady`；可用时验证提交成功。
- storage 使用 local driver 做连通性测试；OSS 直传和分片接口只做认证/参数契约，避免操作配置中的真实 Bucket。
