# gin-layout
[![Go](https://github.com/wannanbigpig/gin-layout/actions/workflows/go.yml/badge.svg)](https://github.com/wannanbigpig/gin-layout/actions/workflows/go.yml)
[![CodeQL](https://github.com/wannanbigpig/gin-layout/actions/workflows/codeql.yml/badge.svg)](https://github.com/wannanbigpig/gin-layout/actions/workflows/codeql.yml)
[![Sync to Gitee](https://github.com/wannanbigpig/gin-layout/actions/workflows/gitee-sync.yml/badge.svg?branch=master)](https://github.com/wannanbigpig/gin-layout/actions/workflows/gitee-sync.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/wannanbigpig/gin-layout)](https://goreportcard.com/report/github.com/wannanbigpig/gin-layout)
[![GitHub license](https://img.shields.io/github/license/wannanbigpig/gin-layout)](https://github.com/wannanbigpig/gin-layout/blob/master/LICENSE)

Translations: [简体中文](README.md) | [English](README_en.md)
### Gin Project Template
> 本项目使用 gin 框架为核心搭建的一个脚手架，可以基于本项目快速完成业务开发，开箱📦 即用

### 更新
新增命令行模式运行：

```shell 
# 测试
go run main.go -r command:demo
```


### HTTP 运行
拉取代码后在项目根目录执行如下命令：
```shell
# 建议开启GO111MODULE
# go env -w GO111MODULE=on

# 下载依赖
go mod download

# 首次运行会自动复制一份示例配置（config/config.example.yaml）文件到config目录(config/config.yaml)
go run main.go

# 项目起来后执行下面命令访问示例路由
curl "http://127.0.0.1:9999/api/v1/hello-world"
# {"code":0,"message":"OK","data":{"result":"hello gin-layout"},"cost":"6.151µs"}
curl "http://127.0.0.1:9999/api/v1/hello-world?name=world"
# {"code":0,"message":"OK","data":{"result":"hello world"},"cost":"6.87µs"}
```

### 部署
```shell
# 打包项目（如何打包其他os平台的包自行 google）
go build -o cmd/go_layout_liunx main.go

# 运行时请配置指定config文件的位置，否则可能会出现找不到配置的情况，修改完配置请重启
cmd/go-layout -c="指定配置文件位置（/home/go-layout-config.yaml）"

# 使用 supervisord 管理进程配置示例如下
[program:go-layout]
command=/home/go-layout/go_layout_liunx -c=/home/go/go-layout/config.yaml
directory=/home/go/go-layout
autostart=true
startsecs=5
user=root
redirect_stderr=true
stdout_logfile=/home/go/go-layout/supervisord_go_layout.log

# nginx 反向代理配置示例
server {
    listen 80;
    server_name api.xxx.com;
    location / {
        proxy_set_header Host $host;
        proxy_pass http://172.0.0.1:9999;
    }
}
```

更新swag 文档 `swag init`

### 目录结构
```
.
|——.gitignore
|——go.mod
|——go.sum
|——main.go    // 项目入口 main 包
|——LICENSE
|——README.md
|——boot    // 项目初始化目录
|  └──boot.go
|——config    // 这里通常维护一些本地调试用的样例配置文件
|  └──autoload    // 配置文件的结构体定义包
|     └──app.go
|     └──logger.go
|     └──mysql.go
|     └──redis.go
|     └──server.go
|  └──config.example.ini    // .ini 配置示例文件
|  └──config.example.yaml    // .yaml 配置示例文件
|  └──config.go    // 配置初始化文件
|——data    // 数据初始化目录
|  └──data.go
|  └──mysql.go
|  └──redis.go
|——internal    // 该服务所有不对外暴露的代码，通常的业务逻辑都在这下面，使用internal避免错误引用
|  └──controller    // 控制器代码
|     └──v1
|        └──auth.go    // 完整流程演示代码，包含数据库表的操作
|        └──helloword.go    // 基础演示代码
|     └──base.go
|  └──middleware    // 中间件目录
|     └──cors.go
|     └──logger.go
|     └──recovery.go
|     └──requestCost.go
|  └──model    // 业务数据访问
|     └──admin_users.go
|     └──base.go
|  └──pkg    // 内部使用包
|     └──errors    // 错误定义
|        └──code.go
|        └──en-us.go
|        └──zh-cn.go
|     └──logger    // 日志处理
|        └──logger.go
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
|  └──utils    // 帮助函数
|     └──utils.go
```

### 生产环境注意事项
> 在构建生产环境时，请配置好 `.yaml` 文件中基础路径 `base_path`，所有的日志记录文件会保存在该目录下的 `{base_path}/gin-layout/logs/` 里面，该基础路径默认为执行命令的目录

### 其他说明
##### 项目中使用到的包
- 核心：[gin](https://github.com/gin-gonic/gin)
- 配置：[gopkg.in/yaml.v3](https://github.com/go-yaml/yaml)、[gopkg.in/ini.v1](https://github.com/go-ini/ini) 
- 参数验证：[github.com/go-playground/validator/v10](https://github.com/go-playground/validator)
- 日志：[go.uber.org/zap](https://github.com/uber-go/zap)、[github.com/natefinch/lumberjack](http://github.com/natefinch/lumberjack)、[github.com/lestrrat-go/file-rotatelogs](https://github.com/lestrrat-go/file-rotatelogs)
- 数据库：[gorm.io/gorm](https://github.com/go-gorm/gorm)、[go-redis/v8](https://github.com/go-redis/redis)
- 还有其他不一一列举，更多请查看`go.mod`文件

### 代码贡献
不完善的地方，欢迎大家 Fork 并提交 PR！