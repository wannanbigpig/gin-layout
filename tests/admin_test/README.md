# tests/admin_test 索引

这份索引用于解决两个问题：

1. 当前后台接口哪些已经在 `tests/admin_test` 里被覆盖到了
2. 某个接口该去哪个测试文件里找

## 当前结论

- 当前 `internal/routers/admin_router.go` 中定义的 **41 个后台接口**，在 `tests/admin_test` 里都至少有一处测试引用
- 但覆盖深度并不完全一致
- 有些接口只有“未登录拦截”测试
- 有些接口有成功链路测试
- 有些写接口已经有完整 create/update/delete 流程测试

所以结论不是“完全没测”，而是：

- **接口覆盖有了**
- **覆盖深度不均匀**

## 目录规则

当前目录按“领域”拆分，同一资源的读写测试放在同一个文件里：

- `public_routes_test.go`
  - 公开接口
- `auth_routes_test.go`
  - 登录、验证码、token 校验、登出
- `common_routes_test.go`
  - 通用接口，例如上传
- `admin_user_test.go`
  - 管理员相关接口
  - 文件内同时包含读测试、鉴权测试、写流程测试
- `permission_routes_test.go`
  - API 权限接口
- `menu_test.go`
  - 菜单相关接口
  - 文件内同时包含读测试、鉴权测试、写流程测试
- `role_test.go`
  - 角色相关接口
  - 文件内同时包含读测试、鉴权测试、写流程测试
- `department_test.go`
  - 部门相关接口
  - 文件内同时包含读测试、鉴权测试、写流程测试
- `log_routes_test.go`
  - 请求日志 / 登录日志接口
- `test_helpers_test.go`
  - 测试辅助函数集合
  - 包含环境判断、测试资源命名、查找、清理、兜底数据创建

## 接口到文件映射

### 公开接口

| 接口 | 测试文件 | 当前覆盖 |
| --- | --- | --- |
| `GET /admin/v1/demo` | `public_routes_test.go` | 成功路径 |
| `GET /admin/v1/file/:uuid` | `public_routes_test.go` | 公开访问基础行为 |
| `POST /admin/v1/login` | `auth_routes_test.go` | 异常路径 |
| `GET /admin/v1/login-captcha` | `auth_routes_test.go` | 成功路径 |

### 通用 / 认证接口

| 接口 | 测试文件 | 当前覆盖 |
| --- | --- | --- |
| `POST /admin/v1/common/upload` | `common_routes_test.go` | 未登录拦截 |
| `POST /admin/v1/auth/logout` | `auth_routes_test.go` | 未登录拦截 |
| `GET /admin/v1/auth/check-token` | `auth_routes_test.go` | 成功路径 |

### 管理员接口

| 接口 | 测试文件 | 当前覆盖 |
| --- | --- | --- |
| `GET /admin/v1/admin-user/get` | `admin_user_test.go` | 成功路径 + 未登录拦截 |
| `GET /admin/v1/admin-user/user-menu-info` | `admin_user_test.go` | 成功路径 |
| `POST /admin/v1/admin-user/update-profile` | `admin_user_test.go` | 参数校验 |
| `GET /admin/v1/admin-user/list` | `admin_user_test.go` | 成功路径 + 未登录拦截 |
| `GET /admin/v1/admin-user/detail` | `admin_user_test.go` | 未登录拦截 + 成功路径 |
| `GET /admin/v1/admin-user/get-full-phone` | `admin_user_test.go` | 未登录拦截 |
| `GET /admin/v1/admin-user/get-full-email` | `admin_user_test.go` | 未登录拦截 |
| `POST /admin/v1/admin-user/create` | `admin_user_test.go` | 未登录拦截 + 成功路径 |
| `POST /admin/v1/admin-user/update` | `admin_user_test.go` | 未登录拦截 + 成功路径 |
| `POST /admin/v1/admin-user/delete` | `admin_user_test.go` | 未登录拦截 + 成功路径 |
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
| `GET /admin/v1/menu/detail` | `menu_test.go` | 未登录拦截 + 成功路径 |
| `POST /admin/v1/menu/create` | `menu_test.go` | 未登录拦截 + 成功路径 |
| `POST /admin/v1/menu/update` | `menu_test.go` | 未登录拦截 + 成功路径 |
| `POST /admin/v1/menu/delete` | `menu_test.go` | 未登录拦截 + 成功路径 |
| `POST /admin/v1/menu/update-all-menu-permissions` | `menu_test.go` | 未登录拦截 + 成功路径 |

### 角色接口

| 接口 | 测试文件 | 当前覆盖 |
| --- | --- | --- |
| `GET /admin/v1/role/list` | `role_test.go` | 成功路径 + 未登录拦截 |
| `GET /admin/v1/role/detail` | `role_test.go` | 未登录拦截 + 成功路径 |
| `POST /admin/v1/role/create` | `role_test.go` | 未登录拦截 + 成功路径 |
| `POST /admin/v1/role/update` | `role_test.go` | 未登录拦截 + 成功路径 |
| `POST /admin/v1/role/delete` | `role_test.go` | 未登录拦截 + 成功路径 |

### 部门接口

| 接口 | 测试文件 | 当前覆盖 |
| --- | --- | --- |
| `GET /admin/v1/department/list` | `department_test.go` | 成功路径 + 未登录拦截 |
| `GET /admin/v1/department/detail` | `department_test.go` | 未登录拦截 + 成功路径 |
| `POST /admin/v1/department/create` | `department_test.go` | 未登录拦截 + 成功路径 |
| `POST /admin/v1/department/update` | `department_test.go` | 未登录拦截 + 成功路径 |
| `POST /admin/v1/department/delete` | `department_test.go` | 未登录拦截 + 成功路径 |
| `POST /admin/v1/department/bind-role` | `department_test.go` | 未登录拦截 + 成功路径 |

### 日志接口

| 接口 | 测试文件 | 当前覆盖 |
| --- | --- | --- |
| `GET /admin/v1/log/request/list` | `log_routes_test.go` | 成功路径 + 未登录拦截 |
| `GET /admin/v1/log/request/detail` | `log_routes_test.go` | 未登录拦截 |
| `GET /admin/v1/log/login/list` | `log_routes_test.go` | 成功路径 + 未登录拦截 |
| `GET /admin/v1/log/login/detail` | `log_routes_test.go` | 未登录拦截 |

## 当前仍然偏弱的接口

这些接口虽然已经在 `tests/` 里有引用，但覆盖还偏浅：

- `POST /admin/v1/common/upload`
- `POST /admin/v1/auth/logout`
- `POST /admin/v1/admin-user/bind-role`
- `GET /admin/v1/admin-user/get-full-phone`
- `GET /admin/v1/admin-user/get-full-email`
- `POST /admin/v1/permission/update`
- `GET /admin/v1/log/request/detail`
- `GET /admin/v1/log/login/detail`

它们当前主要是：

- 未登录拦截测试
- 或参数校验测试

后续如果继续增强测试，应优先给这些接口补成功路径。
