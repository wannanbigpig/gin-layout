# tests 目录说明

当前项目对外路由共 `42` 个：

- 根路由 `GET /ping` 1 个
- 管理端 `/admin/v1` 路由 41 个

目前这 `42` 个接口都能在 `tests` 目录下找到对应测试入口。

## 目录职责

- `test.go`
  - 测试公共初始化入口
  - 负责装载配置、初始化数据库、验证器、Gin Router
- `ping_test.go`
  - 根路由 `GET /ping`
- `admin_test/`
  - 管理端接口测试目录
  - 按资源域拆分，同一资源的读写测试放在同一个文件里

## admin_test 文件分工

- `public_routes_test.go`
  - `/admin/v1/demo`
  - `/admin/v1/file/:uuid`
- `auth_routes_test.go`
  - `/admin/v1/auth/captcha`
  - `/admin/v1/auth/login`
  - `/admin/v1/auth/check-token`
  - `/admin/v1/auth/logout`
- `common_routes_test.go`
  - `/admin/v1/common/upload`
- `admin_user_test.go`
  - `GET /admin/v1/admin-user/*`
  - `POST /admin/v1/admin-user/update-profile`
  - `POST /admin/v1/admin-user/update-password`
  - `POST /admin/v1/admin-user/create`
  - `POST /admin/v1/admin-user/update`
  - `POST /admin/v1/admin-user/delete`
  - `POST /admin/v1/admin-user/bind-role`
- `permission_routes_test.go`
  - `POST /admin/v1/permission/update`
  - `GET /admin/v1/permission/list`
- `menu_test.go`
  - `GET /admin/v1/menu/*`
  - `POST /admin/v1/menu/create`
  - `POST /admin/v1/menu/update`
  - `POST /admin/v1/menu/delete`
- `role_test.go`
  - `GET /admin/v1/role/*`
  - `POST /admin/v1/role/create`
  - `POST /admin/v1/role/update`
  - `POST /admin/v1/role/delete`
  - `POST /admin/v1/role/menu-access`
- `department_test.go`
  - `GET /admin/v1/department/*`
  - `POST /admin/v1/department/create`
  - `POST /admin/v1/department/update`
  - `POST /admin/v1/department/delete`
  - `POST /admin/v1/department/user-access`
- `log_routes_test.go`
  - `GET /admin/v1/log/request/list`
  - `GET /admin/v1/log/request/detail`
  - `GET /admin/v1/log/login/list`
  - `GET /admin/v1/log/login/detail`

## 辅助文件

- `admin_test/admin_test.go`
  - `TestMain`
  - 请求发送、鉴权头注入、统一响应解析
- `admin_test/test_helpers_test.go`
  - 环境判断、测试资源命名、查找、清理、兜底数据创建

## 维护规则

- 新增接口时，必须在 `tests` 目录补对应测试。
- 管理端新接口默认放到 `tests/admin_test`，并按资源域归类。
- 同一资源的读写测试优先合并在同一个 `*_test.go` 文件内。
