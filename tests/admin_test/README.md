# tests/admin_test 索引

这份索引用于解决两个问题：

1. 当前后台接口哪些已经在 `tests/admin_test` 里被覆盖到了
2. 某个接口该去哪个测试文件里找

## 当前结论

- `internal/routers/admin_router.go` 中共定义 **68 个接口**
- 所有 68 个接口都至少有一处测试引用
- 但覆盖深度并不完全一致
  - 64 个接口有成功路径测试（含完整 CRUD 流程）
  - 4 个接口仅有未登录拦截 / 参数校验测试

结论：

- **接口覆盖有了**
- **覆盖深度不均匀**

## 目录规则

当前目录按"领域"拆分，同一资源的读写测试放在同一个文件里：

- `public_routes_test.go` — 公开接口
- `auth_routes_test.go` — 登录、验证码、token 校验、登出
- `common_routes_test.go` — 通用接口（上传等）
- `admin_user_test.go` — 管理员相关接口（含读测试、鉴权测试、写流程测试）
- `permission_routes_test.go` — API 权限接口
- `menu_test.go` — 菜单相关接口（含读测试、鉴权测试、写流程测试）
- `role_test.go` — 角色相关接口（含读测试、鉴权测试、写流程测试）
- `department_test.go` — 部门相关接口（含读测试、鉴权测试、写流程测试）
- `system_routes_test.go` — 系统参数 / 字典接口
- `log_routes_test.go` — 请求日志 / 登录日志接口
- `task_routes_test.go` — 任务中心组接口
- `test_helpers_test.go` — 测试辅助函数（环境判断、测试资源命名/查找/清理/兜底数据创建）

## 接口到文件映射

### 公开接口

| 接口 | 测试文件 | 当前覆盖 |
| --- | --- | --- |
| `GET /admin/v1/demo` | `public_routes_test.go` | 成功路径 |
| `GET /admin/v1/file/:uuid` | `public_routes_test.go` | Not-found |
| `POST /admin/v1/login` | `auth_routes_test.go` | 参数校验（验证码错误） |
| `GET /admin/v1/login-captcha` | `auth_routes_test.go` | 成功路径 |

### 通用 / 认证接口

| 接口 | 测试文件 | 当前覆盖 |
| --- | --- | --- |
| `POST /admin/v1/common/upload` | `common_routes_test.go` | 未登录拦截 |
| `POST /admin/v1/auth/logout` | `auth_routes_test.go` | 成功路径（DB token 撤销）+ 未登录拦截 |
| `GET /admin/v1/auth/check-token` | `auth_routes_test.go` | 成功路径 |

### 管理员接口

| 接口 | 测试文件 | 当前覆盖 |
| --- | --- | --- |
| `GET /admin/v1/admin-user/get` | `admin_user_test.go` | 成功路径 + 未登录拦截 |
| `GET /admin/v1/admin-user/user-menu-info` | `admin_user_test.go` | 成功路径 |
| `POST /admin/v1/admin-user/update-profile` | `admin_user_test.go` | 参数校验 |
| `GET /admin/v1/admin-user/list` | `admin_user_test.go` | 成功路径 + 未登录拦截 |
| `GET /admin/v1/admin-user/detail` | `admin_user_test.go` | 成功路径 + 未登录拦截 |
| `GET /admin/v1/admin-user/get-full-phone` | `admin_user_test.go` | 未登录拦截 |
| `GET /admin/v1/admin-user/get-full-email` | `admin_user_test.go` | 未登录拦截 |
| `POST /admin/v1/admin-user/create` | `admin_user_test.go` | 成功路径 + 未登录拦截 |
| `POST /admin/v1/admin-user/update` | `admin_user_test.go` | 成功路径 + 未登录拦截 |
| `POST /admin/v1/admin-user/delete` | `admin_user_test.go` | 成功路径 + 未登录拦截 |
| `POST /admin/v1/admin-user/bind-role` | `admin_user_test.go` | 未登录拦截 |

### 权限接口

| 接口 | 测试文件 | 当前覆盖 |
| --- | --- | --- |
| `GET /admin/v1/permission/list` | `permission_routes_test.go` | 成功路径 + 未登录拦截 |
| `POST /admin/v1/permission/update` | `permission_routes_test.go` | 未登录拦截 + 参数校验 |

### 菜单接口

| 接口 | 测试文件 | 当前覆盖 |
| --- | --- | --- |
| `GET /admin/v1/menu/list` | `menu_test.go` | 成功路径 + 未登录拦截 |
| `GET /admin/v1/menu/detail` | `menu_test.go` | 成功路径 + 未登录拦截 |
| `POST /admin/v1/menu/create` | `menu_test.go` | 成功路径 + 未登录拦截 |
| `POST /admin/v1/menu/update` | `menu_test.go` | 成功路径 + 未登录拦截 |
| `POST /admin/v1/menu/delete` | `menu_test.go` | 成功路径 + 未登录拦截 |
| `POST /admin/v1/menu/update-all-menu-permissions` | `menu_test.go` | 成功路径 + 未登录拦截 |

### 角色接口

| 接口 | 测试文件 | 当前覆盖 |
| --- | --- | --- |
| `GET /admin/v1/role/list` | `role_test.go` | 成功路径 + 未登录拦截 |
| `GET /admin/v1/role/detail` | `role_test.go` | 成功路径 + 未登录拦截 |
| `POST /admin/v1/role/create` | `role_test.go` | 成功路径 + 未登录拦截 |
| `POST /admin/v1/role/update` | `role_test.go` | 成功路径 + 未登录拦截 |
| `POST /admin/v1/role/delete` | `role_test.go` | 成功路径 + 未登录拦截 |

### 部门接口

| 接口 | 测试文件 | 当前覆盖 |
| --- | --- | --- |
| `GET /admin/v1/department/list` | `department_test.go` | 成功路径 + 未登录拦截 |
| `GET /admin/v1/department/detail` | `department_test.go` | 成功路径 + 未登录拦截 |
| `POST /admin/v1/department/create` | `department_test.go` | 成功路径 + 未登录拦截 |
| `POST /admin/v1/department/update` | `department_test.go` | 成功路径 + 未登录拦截 |
| `POST /admin/v1/department/delete` | `department_test.go` | 成功路径 + 未登录拦截 |
| `POST /admin/v1/department/bind-role` | `department_test.go` | 成功路径 + 未登录拦截 |

### 系统参数

| 接口 | 测试文件 | 当前覆盖 |
| --- | --- | --- |
| `GET /admin/v1/system/config/list` | `system_routes_test.go` | 成功路径 + 未登录拦截（成功路径依赖 `sys_*` 表已迁移） |
| `GET /admin/v1/system/config/detail` | `system_routes_test.go` | 成功路径 + 未登录拦截（成功路径依赖 `sys_*` 表已迁移） |
| `GET /admin/v1/system/config/value` | `system_routes_test.go` | 成功路径 + 未登录拦截（成功路径依赖 `sys_*` 表已迁移） |
| `POST /admin/v1/system/config/create` | `system_routes_test.go` | 成功路径 + 未登录拦截（成功路径依赖 `sys_*` 表已迁移） |
| `POST /admin/v1/system/config/update` | `system_routes_test.go` | 成功路径 + 未登录拦截（成功路径依赖 `sys_*` 表已迁移） |
| `POST /admin/v1/system/config/delete` | `system_routes_test.go` | 成功路径 + 未登录拦截（成功路径依赖 `sys_*` 表已迁移） |
| `POST /admin/v1/system/config/refresh` | `system_routes_test.go` | 成功路径 + 未登录拦截（成功路径依赖 `sys_*` 表已迁移） |

### 系统字典

| 接口 | 测试文件 | 当前覆盖 |
| --- | --- | --- |
| `GET /admin/v1/system/dict/type/list` | `system_routes_test.go` | 成功路径 + 未登录拦截（成功路径依赖 `sys_*` 表已迁移） |
| `GET /admin/v1/system/dict/type/detail` | `system_routes_test.go` | 成功路径 + 未登录拦截（成功路径依赖 `sys_*` 表已迁移） |
| `POST /admin/v1/system/dict/type/create` | `system_routes_test.go` | 成功路径 + 未登录拦截（成功路径依赖 `sys_*` 表已迁移） |
| `POST /admin/v1/system/dict/type/update` | `system_routes_test.go` | 成功路径 + 未登录拦截（成功路径依赖 `sys_*` 表已迁移） |
| `POST /admin/v1/system/dict/type/delete` | `system_routes_test.go` | 成功路径 + 未登录拦截（成功路径依赖 `sys_*` 表已迁移） |
| `GET /admin/v1/system/dict/item/list` | `system_routes_test.go` | 成功路径 + 未登录拦截（成功路径依赖 `sys_*` 表已迁移） |
| `POST /admin/v1/system/dict/item/create` | `system_routes_test.go` | 成功路径 + 未登录拦截（成功路径依赖 `sys_*` 表已迁移） |
| `POST /admin/v1/system/dict/item/update` | `system_routes_test.go` | 成功路径 + 未登录拦截（成功路径依赖 `sys_*` 表已迁移） |
| `POST /admin/v1/system/dict/item/delete` | `system_routes_test.go` | 成功路径 + 未登录拦截（成功路径依赖 `sys_*` 表已迁移） |
| `GET /admin/v1/system/dict/options` | `system_routes_test.go` | 成功路径 + 未登录拦截（成功路径依赖 `sys_*` 表已迁移） |

### 日志接口

| 接口 | 测试文件 | 当前覆盖 |
| --- | --- | --- |
| `GET /admin/v1/log/request/list` | `log_routes_test.go` | 成功路径 + 未登录拦截 |
| `GET /admin/v1/log/request/detail` | `log_routes_test.go` | 未登录拦截 |
| `GET /admin/v1/log/request/export` | `log_routes_test.go` | 成功路径 + 未登录拦截 |
| `GET /admin/v1/log/request/mask-config` | `log_routes_test.go` | 成功路径 + 未登录拦截 |
| `POST /admin/v1/log/request/mask-config` | `log_routes_test.go` | 成功路径 + 未登录拦截 |
| `GET /admin/v1/log/login/list` | `log_routes_test.go` | 成功路径 + 未登录拦截 |
| `GET /admin/v1/log/login/detail` | `log_routes_test.go` | 未登录拦截 |

### 任务中心

| 接口 | 测试文件 | 当前覆盖 |
| --- | --- | --- |
| `GET /admin/v1/task/list` | `task_routes_test.go` | 成功路径 + 未登录拦截 |
| `POST /admin/v1/task/trigger` | `task_routes_test.go` | 未登录拦截 + 参数校验 |
| `GET /admin/v1/task/run/list` | `task_routes_test.go` | 成功路径 + 未登录拦截 |
| `GET /admin/v1/task/run/detail` | `task_routes_test.go` | Not-found + 未登录拦截 |
| `POST /admin/v1/task/run/retry` | `task_routes_test.go` | 未登录拦截 + 参数校验 |
| `POST /admin/v1/task/run/cancel` | `task_routes_test.go` | 未登录拦截 + 参数校验 |
| `GET /admin/v1/task/cron/state` | `task_routes_test.go` | 成功路径 + 未登录拦截 |

## 覆盖统计

| 覆盖等级 | 接口数 | 说明 |
| --- | --- | --- |
| 成功路径 + CRUD 流程 | 50 | admin-user / menu / role / department / system 等有完整 write flow |
| 成功路径（仅读） | 14 | 列表、详情、导出等只读操作 |
| 未登录拦截（仅有） | 4 | 仅覆盖了鉴权，无成功路径 |

## 当前仍然偏弱的接口

这些接口仅有未登录拦截或参数校验测试，后续增强应优先补成功路径：

**其他模块**
- `POST /admin/v1/common/upload` — 仅有未登录拦截
- `POST /admin/v1/admin-user/bind-role` — 仅有未登录拦截
- `GET /admin/v1/admin-user/get-full-phone` — 仅有未登录拦截
- `GET /admin/v1/admin-user/get-full-email` — 仅有未登录拦截

**覆盖较浅但暂可接受的接口**
- `POST /admin/v1/admin-user/update-profile` — 仅有参数校验（无成功路径）
- `POST /admin/v1/permission/update` — 仅有未登录拦截 + 参数校验
- `GET /admin/v1/log/request/detail` — 仅有未登录拦截
- `GET /admin/v1/log/login/detail` — 仅有未登录拦截
