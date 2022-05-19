# gin-layout
[![Go](https://github.com/wannanbigpig/gin-layout/actions/workflows/go.yml/badge.svg)](https://github.com/wannanbigpig/gin-layout/actions/workflows/go.yml)
[![CodeQL](https://github.com/wannanbigpig/gin-layout/actions/workflows/codeql.yml/badge.svg)](https://github.com/wannanbigpig/gin-layout/actions/workflows/codeql.yml)
[![Sync to Gitee](https://github.com/wannanbigpig/gin-layout/actions/workflows/gitee-sync.yml/badge.svg?branch=master)](https://github.com/wannanbigpig/gin-layout/actions/workflows/gitee-sync.yml)
[![FOSSA Status](https://app.fossa.com/api/projects/git%2Bgithub.com%2Fwannanbigpig%2Fgin-layout.svg?type=shield)](https://app.fossa.com/projects/git%2Bgithub.com%2Fwannanbigpig%2Fgin-layout?ref=badge_shield)

Translations: [English](README.md) | [简体中文](README_zh.md)
### Gin Project Template
> 本项目使用 gin 框架为核心搭建的一个脚手架，可以基于本项目快速完成业务开发，开箱📦 即用

### 运行
拉取代码后在项目根目录执行如下命令：
```shell
# 建议开启GO111MODULE
# go env -w GO111MODULE=on

# 下载依赖
go mod download

# 运行
go run cmd/main.go
```

### 目录结构
```
.
|——.gitignore
|——go.mod
|——go.sum
|——LICENSE
|——README.md
|——boot    // 项目初始化目录
|  └──boot.go
|——cmd    // 执行命令存放目录
|  └──main.go    // 项目入口 main 包
|——config    // 这里通常维护一些本地调试用的样例配置文件
|  └──autoload    // 配置文件的结构体定义包
|     └──app.go
|     └──logger.go
|     └──mysql.go
|     └──server.go
|  └──config.go    // 配置初始化文件
|  └──config.example.ini    // .ini 配置示例文件
|  └──config.example.yaml    // .yaml 配置示例文件
|——data    // 数据初始化目录
|  └──mysql.go
|  └──data.go
|——internal    // 该服务所有不对外暴露的代码，通常的业务逻辑都在这下面，使用internal避免错误引用
|  └──controller    // 控制器代码
|     └──v1
|        └──auth.go    // 完整流程演示代码，包含数据库表的操作
|        └──helloword.go    // 基础演示代码
|     └──base.go
|  └──middleware    // 中间件目录
|     └──cors.go
|     └──recovery.go
|     └──requestCost.go
|  └──model    // 业务数据访问
|     └──admin_users.go
|     └──base.go
|  └──pkg    // 内部使用包
|     └──error_code    // 错误码定义
|        └──code.go
|        └──en-us.go
|        └──zh-cn.go
|     └──response    // 统一响应输出
|        └──response.go
|  └──routers    // 路由定义
|     └──apiRouter.go
|     └──router.go
|  └──service    // 业务逻辑
|     └──auth.go
|  └──validator    // 请求参数验证器
|     └──form    // 表单参数定义
|        └──auth.go
|     └──validator.go
|——pkg    // 可以被外部使用的包
|  └──convert    // 数据类型转换
|     └──convert.go
|  └──logger    // 日志处理
|     └──logger.go
|  └──response    // 响应处理
|     └──response.go
|  └──utils    // 帮助函数
|     └──utils.go
```

### 生产环境注意事项
> 在构建生产环境时，请配置好 `.yaml` 文件中基础路径 `base_path`，所有的日志记录文件会保存在该目录下的 `{base_path}/gin-layout/logs/` 里面，该基础路径默认为执行命令的目录

### 其他说明
##### 项目中使用到的包
- 核心：[gin](https://github.com/gin-gonic/gin)
- 配置：[gopkg.in/yaml.v3](https://github.com/go-yaml/yaml)、[gopkg.in/ini.v1](https://github.com/go-ini/ini) （默认使用yaml）
- 参数验证：[github.com/go-playground/validator/v10](https://github.com/go-playground/validator)、[github.com/natefinch/lumberjack](http://github.com/natefinch/lumberjack)、[github.com/lestrrat-go/file-rotatelogs](https://github.com/lestrrat-go/file-rotatelogs)
- 日志：[go.uber.org/zap](https://github.com/uber-go/zap)
- 数据库：[gorm.io/gorm](https://github.com/go-gorm/gorm)
- 还有其他不一一列举，更多请查看`go.mod`文件

### 代码贡献
不完善的地方，欢迎大家 Fork 并提交 PR！

### LICENSE
##### MIT
[![FOSSA Status](https://app.fossa.com/api/projects/git%2Bgithub.com%2Fwannanbigpig%2Fgin-layout.svg?type=large)](https://app.fossa.com/projects/git%2Bgithub.com%2Fwannanbigpig%2Fgin-layout?ref=badge_large)
