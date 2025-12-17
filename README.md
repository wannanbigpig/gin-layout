# gin-layout

[![Go](https://github.com/wannanbigpig/gin-layout/actions/workflows/go.yml/badge.svg)](https://github.com/wannanbigpig/gin-layout/actions/workflows/go.yml)
[![CodeQL](https://github.com/wannanbigpig/gin-layout/actions/workflows/codeql.yml/badge.svg)](https://github.com/wannanbigpig/gin-layout/actions/workflows/codeql.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/wannanbigpig/gin-layout)](https://goreportcard.com/report/github.com/wannanbigpig/gin-layout)
[![GitHub license](https://img.shields.io/github/license/wannanbigpig/gin-layout)](https://github.com/wannanbigpig/gin-layout/blob/master/LICENSE)
[![Go Version](https://img.shields.io/badge/Go-%3E%3D1.23-blue.svg)](https://golang.org/)

> 基于 Gin 框架的企业级 Go 项目脚手架（Go Admin 后台管理系统），开箱即用的 Go 管理后台框架，快速构建 RESTful API 服务和管理后台系统

**前端项目：** [go-admin-ui](https://github.com/wannanbigpig/go-admin-ui) - 基于 Vue 3 + Element Plus 的管理后台前端项目

**API 文档：** [在线文档](https://wannanbigpig.apifox.cn/) - 完整的接口文档和使用说明

**演示地址：** [在线演示](https://x-l-admin.wannanbigpig.com/) - 可直接体验完整的管理后台功能

## 📌 项目简介

**gin-layout** 是一个功能完整的 **Go Admin 后台管理系统**框架，专为快速构建企业级管理后台而设计。本项目提供了完整的 RBAC 权限管理、JWT 认证、日志系统等核心功能，是构建 Go 管理后台、Go Admin Panel、Go 后台管理系统的理想选择。

## ✨ 特性

- 🚀 **开箱即用** - 完整的 Go Admin 项目结构，无需从零开始搭建
- 🔐 **RBAC 权限管理** - 基于 Casbin 的完整权限控制系统，适合构建 Go 管理后台系统
- 📝 **JWT 认证** - 支持 Token 生成、刷新和黑名单管理，保障 Go Admin 系统安全
- 📦 **数据库迁移** - 使用 migrate 进行数据库版本管理
- 📊 **日志系统** - 基于 zap 的高性能日志，支持文件和控制台输出
- 🔄 **CORS 支持** - 完整的跨域资源共享配置
- 📤 **文件上传** - 支持公开/私有文件存储，UUID 标识
- 🛡️ **参数验证** - 基于 validator 的请求参数验证
- 📈 **请求日志** - 自动记录 API 请求日志，便于 Go Admin 系统监控
- 🎯 **统一响应** - 标准化的 API 响应格式
- ⚙️ **配置管理** - 基于 Viper 的灵活配置系统
- 🔧 **CLI 工具** - 支持 server、command、cron 等多种命令
- 📱 **软删除** - 支持数据库软删除功能
- 🔒 **数据加密** - 支持敏感数据加密存储（如 Token 加密）

## 📋 目录结构

```
gin-layout/
├── cmd/                    # 命令行工具
│   ├── service/           # 服务器启动命令
│   ├── command/           # 单次执行命令
│   ├── cron/              # 定时任务
│   └── version/           # 版本信息
├── config/                # 配置文件
│   ├── autoload/          # 配置结构体定义
│   └── config.yaml.example  # 配置示例文件
├── data/                  # 数据层
│   ├── migrations/        # 数据库迁移文件
│   ├── mysql.go          # MySQL 连接
│   └── redis.go          # Redis 连接
├── internal/              # 内部代码（不对外暴露）
│   ├── controller/        # 控制器层
│   ├── service/           # 业务逻辑层
│   ├── model/             # 数据模型
│   ├── middleware/        # 中间件
│   ├── pkg/               # 内部工具包
│   ├── resources/         # 资源转换器
│   ├── routers/           # 路由定义
│   └── validator/         # 参数验证
├── pkg/                   # 公共包（可对外暴露）
│   └── utils/             # 工具函数
├── storage/               # 文件存储
│   ├── public/            # 公开文件
│   └── private/           # 私有文件
├── logs/                  # 日志文件目录
├── main.go                # 程序入口
└── README.md              # 项目说明
```

## 🚀 快速开始

本 Go Admin 框架支持快速搭建管理后台系统，适用于企业级 Go 后台管理系统开发。

### 环境要求

- Go >= 1.23
- MySQL >= 5.7
- Redis >= 5.0 (可选)

### 安装步骤

1. **克隆项目**
```bash
git clone https://github.com/wannanbigpig/gin-layout.git
cd gin-layout
```

2. **安装依赖**
```bash
go mod download
```

3. **配置数据库**

安装 migrate 工具（[安装文档](https://github.com/golang-migrate/migrate/tree/master/cmd/migrate)）

执行数据库迁移：
```bash
migrate -database 'mysql://root:root@tcp(127.0.0.1:3306)/go_layout?charset=utf8mb4&parseTime=True&loc=Local' \
  -path data/migrations up
```

4. **配置应用**

首次运行会自动从 `config/config.yaml.example` 复制配置文件到项目根目录的 `config.yaml`。

编辑项目根目录的 `config.yaml`，配置数据库和 Redis 连接信息：

```yaml
mysql:
  enable: true
  host: 127.0.0.1
  port: 3306
  database: go_layout
  username: root
  password: your_password

redis:
  enable: true
  host: 127.0.0.1
  port: 6379
  password: ""
  database: 0
```

5. **启动服务**

**正常启动（日常使用）：**
```bash
GO_ENV=development go run main.go service
```

**首次初始化（仅首次部署时需要）：**

首次部署时，需要初始化数据库中的 API 路由表和菜单-API 映射关系。你可以选择两种方式：

**方式一：完整初始化系统（推荐）**

一键完成所有初始化操作，包括回滚迁移、重新执行迁移、初始化路由和路由映射：

```bash
GO_ENV=development go run main.go command init-system
```

该命令会执行以下操作：
1. 回滚所有数据库迁移
2. 重新执行所有迁移
3. 重新初始化 API 路由表
4. 重新初始化菜单-API 映射关系

**方式二：分步初始化**

如果需要分步执行，可以使用以下命令：

**步骤 1：初始化 API 路由表**

扫描系统中定义的所有 API 路由并写入 `a_api` 表：

```bash
GO_ENV=development go run main.go command api-route
```

该命令会自动扫描系统中定义的所有 API 路由，并将路由信息（路径、方法、处理器等）写入 `a_api` 表中，用于权限管理和 API 文档生成。

**步骤 2：初始化菜单-API 映射关系**

API 路由表初始化完成后，建立菜单与 API 的映射关系：

```bash
GO_ENV=development go run main.go command menu-api-map
```

该命令会根据 `casbin_rule` 表中的权限规则，自动建立默认菜单与 API 的映射关系，初始化 `a_menu_api_map` 表。

> **重要提示**：
> - 初始化命令是独立的，可以随时单独执行
> - 日常启动服务时直接使用 `go run main.go service` 即可，无需任何参数
> - 如果只需要更新 API 路由表或菜单映射，可以单独执行对应的初始化命令
> - `command init-system` 命令会直接执行，无需确认
> - 系统会在每天凌晨2点自动执行初始化任务（通过 `go-layout cron` 启动定时任务服务）

**生产模式：**
```bash
go build -o go-layout main.go
./go-layout service
```

6. **测试接口**

```bash
# 健康检查
curl http://127.0.0.1:9001/ping
# 响应: {"message":"pong"}

# 示例接口
curl http://127.0.0.1:9001/admin/v1/demo
```

## 📖 使用说明

### Go Admin 系统功能

本框架提供了完整的 Go 管理后台功能，包括：

- **用户管理** - 管理员用户管理、角色分配、部门管理
- **权限管理** - 基于 RBAC 的权限控制系统，支持角色、菜单、API 权限管理
- **登录认证** - JWT Token 认证、登录日志记录、Token 刷新机制
- **日志管理** - 登录日志、请求日志记录和查询
- **菜单管理** - 动态菜单配置，支持多级菜单
- **API 管理** - API 路由自动扫描和权限配置

### 命令行工具

项目支持多种命令，使用 `-h` 查看帮助：

```bash
go run main.go -h
```

**可用命令：**

- `service` - 启动 API 服务器
- `command` - 执行单次命令（包括初始化命令）
  - `command api-route` - 初始化 API 路由表
  - `command menu-api-map` - 初始化菜单-API 映射关系
  - `command init-system` - 初始化系统数据（回滚迁移、重新执行迁移、重新初始化路由和路由映射）
  - `command demo` - 执行示例命令
- `cron` - 启动定时任务（每天凌晨2点自动执行系统初始化）
- `version` - 查看版本信息

### 配置说明

配置文件位置：
- **开发模式**：项目根目录的 `config.yaml`
- **生产模式**：执行文件所在目录的 `config.yaml`
- **自定义路径**：可通过 `-c` 或 `--config` 参数指定配置文件绝对路径

主要配置项：

- **app** - 应用配置（环境、调试模式、语言等）
- **jwt** - JWT 配置（密钥、过期时间等）
- **mysql** - MySQL 数据库配置
- **redis** - Redis 配置
- **logger** - 日志配置（输出方式、文件切割等）

详细配置说明请参考 `config/config.yaml.example`。

**配置文件查找顺序**：
1. 如果通过 `-c` 或 `--config` 参数指定了路径，使用指定的配置文件
2. 开发模式（`GO_ENV=development`）：从当前工作目录查找 `config.yaml`
3. 生产模式：从执行文件所在目录查找 `config.yaml`

### API 路由

- **公开接口** - `/admin/v1/login`、`/admin/v1/login-captcha` 等
- **需要认证** - 其他接口需要在请求头中携带 `Authorization: Bearer <token>`

### 分支说明

- **basic_layout** - 基础脚手架分支，提供干净的开发环境
  - [接口文档](https://apifox.com/apidoc/shared-721e0594-dea4-4d86-bad3-851b51c16e03)
- **x_l_admin** - 管理台服务分支，包含完整的 RBAC 权限管理
  - [接口文档](https://apifox.com/apidoc/shared-c429e6ec-8246-4eb4-a503-3927602af312)

### 前端项目

本项目配套的前端管理后台项目：

- **[go-admin-ui](https://github.com/wannanbigpig/go-admin-ui)** - 基于 Vue 3 + Element Plus + Vite 构建的现代化管理后台前端
  - 完整的 RBAC 权限控制
  - 动态路由和菜单生成
  - 响应式布局设计
  - 丰富的组件和工具函数

## 🚢 部署

### 构建

```bash
# 构建可执行文件
go build -o go-layout main.go
```

### 使用 Supervisor 管理进程

创建 `/etc/supervisor/conf.d/go-layout.conf`：

```ini
[program:go-layout]
command=/path/to/go-layout service -c=/path/to/config.yaml
directory=/path/to/go-layout
autostart=true
autorestart=true
startsecs=5
user=www-data
redirect_stderr=true
stdout_logfile=/path/to/go-layout/supervisord.log
```

启动服务：
```bash
supervisorctl reread
supervisorctl update
supervisorctl start go-layout
```

### Nginx 反向代理

```nginx
server {
    listen 80;
    server_name api.example.com;

    location / {
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_pass http://127.0.0.1:9001;
    }
}
```

### Docker 部署（可选）

```dockerfile
FROM golang:1.23-alpine AS builder
WORKDIR /app
COPY . .
RUN go build -o go-layout main.go

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /app/go-layout .
COPY --from=builder /app/config/config.yaml.example ./config.yaml
# 或者使用自定义配置文件路径：
# CMD ["./go-layout", "service", "-c", "/path/to/config.yaml"]
EXPOSE 9001
CMD ["./go-layout", "service"]
```

## ⚙️ 生产环境注意事项

1. **配置文件路径** - 生产环境建议使用 `-c` 参数指定配置文件的绝对路径，或确保 `config.yaml` 位于执行文件所在目录
2. **日志路径** - 配置 `base_path`，日志文件会保存在 `{base_path}/logs/` 目录
3. **JWT 密钥** - 确保 JWT secret_key 足够复杂且保密
4. **数据库连接** - 配置合适的连接池大小
5. **CORS 配置** - 生产环境建议配置具体的允许源，而不是允许所有源

## 🛠️ 技术栈

本 Go Admin 框架采用现代化的技术栈，为构建高性能的管理后台系统提供坚实基础。

### 核心框架
- [Gin](https://github.com/gin-gonic/gin) - HTTP Web 框架
- [GORM](https://gorm.io/) - ORM 框架
- [Viper](https://github.com/spf13/viper) - 配置管理
- [Cobra](https://github.com/spf13/cobra) - CLI 框架

### 认证与权限
- [JWT](https://github.com/golang-jwt/jwt) - JWT 认证
- [Casbin](https://github.com/casbin/casbin) - 权限控制

### 数据存储
- [MySQL](https://github.com/go-sql-driver/mysql) - 关系型数据库
- [Redis](https://github.com/go-redis/redis) - 缓存数据库

### 工具库
- [Zap](https://github.com/uber-go/zap) - 高性能日志库
- [Validator](https://github.com/go-playground/validator) - 参数验证
- [UUID](https://github.com/google/uuid) - UUID 生成

更多依赖请查看 `go.mod` 文件。

## 📝 开发指南

### 构建 Go Admin 系统

使用本框架可以快速构建功能完整的 Go 管理后台系统，支持用户管理、权限控制、数据管理等核心功能。

### 添加新接口

1. 在 `internal/controller/` 创建控制器
2. 在 `internal/service/` 实现业务逻辑
3. 在 `internal/routers/` 注册路由
4. 在 `internal/validator/form/` 定义参数验证

### 数据库迁移

创建新的迁移文件：
```bash
migrate create -ext sql -dir data/migrations -seq add_user_table
```

执行迁移：
```bash
migrate -database 'mysql://...' -path data/migrations up
```

回滚迁移：
```bash
migrate -database 'mysql://...' -path data/migrations down
```

## 🔍 相关搜索关键词

- Go Admin - Go 后台管理系统框架
- Go 管理后台 - 基于 Gin 的管理后台系统
- Go Admin Panel - Go 管理面板框架
- Go 后台框架 - 企业级 Go 后台管理系统
- Go Admin System - 完整的 Go 管理后台解决方案
- Gin Admin - 基于 Gin 框架的管理后台
- Go 管理系统 - RBAC 权限管理的 Go 系统
- Go 后台管理 - 开箱即用的 Go Admin 框架

## 🤝 贡献

欢迎提交 Issue 和 Pull Request！

1. Fork 本仓库
2. 创建特性分支 (`git checkout -b feature/AmazingFeature`)
3. 提交更改 (`git commit -m 'Add some AmazingFeature'`)
4. 推送到分支 (`git push origin feature/AmazingFeature`)
5. 开启 Pull Request

## 📄 许可证

本项目采用 MIT 许可证，详情请查看 [LICENSE](LICENSE) 文件。

## 🙏 致谢

感谢所有为这个项目做出贡献的开发者！

## 📮 联系方式

如有问题或建议，请通过以下方式联系：

- 提交 [Issue](https://github.com/wannanbigpig/gin-layout/issues)
- 查看 [接口文档](https://apifox.com/apidoc/shared-721e0594-dea4-4d86-bad3-851b51c16e03)

---

⭐ 如果这个项目对你有帮助，请给个 Star 支持一下！
