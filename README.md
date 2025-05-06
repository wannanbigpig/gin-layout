# gin-layout
[![Go](https://github.com/wannanbigpig/gin-layout/actions/workflows/go.yml/badge.svg)](https://github.com/wannanbigpig/gin-layout/actions/workflows/go.yml)
[![CodeQL](https://github.com/wannanbigpig/gin-layout/actions/workflows/codeql.yml/badge.svg)](https://github.com/wannanbigpig/gin-layout/actions/workflows/codeql.yml)
[![Sync to Gitee](https://github.com/wannanbigpig/gin-layout/actions/workflows/gitee-sync.yml/badge.svg?branch=master)](https://github.com/wannanbigpig/gin-layout/actions/workflows/gitee-sync.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/wannanbigpig/gin-layout)](https://goreportcard.com/report/github.com/wannanbigpig/gin-layout)
[![GitHub license](https://img.shields.io/github/license/wannanbigpig/gin-layout)](https://github.com/wannanbigpig/gin-layout/blob/master/LICENSE)

### Gin Project Template
> 本项目使用 gin 框架为核心搭建的一个脚手架，可以基于本项目快速完成业务开发，开箱📦 即用

### 分支说明
#### basic_layout 
作为脚手架基本分支，不会更新太多API接口上来，作为一个干净的脚手架，可以提供一个更好的开发体验
> 已实现接口文档：[点击跳转至接口文档](https://apifox.com/apidoc/shared-721e0594-dea4-4d86-bad3-851b51c16e03)

#### x_l_admin
分支后面会作为一个管理台服务添加更多接口，例如rbac权限管理等
> 已实现接口文档：[点击跳转至接口文档](https://apifox.com/apidoc/shared-c429e6ec-8246-4eb4-a503-3927602af312)

### 运行
拉取代码：
```shell
git clone https://github.com/wannanbigpig/gin-layout.git
```
执行迁移文件：

安装migrate [查看安装文档](https://github.com/golang-migrate/migrate/tree/master/cmd/migrate)

使用migrate [更多用法](https://github.com/golang-migrate/migrate)
```shell
# 执行迁移文件创建数据表
migrate -database 'mysql://root:root@tcp(127.0.0.1:3306)/go_layout?charset=utf8mb4&parseTime=True&loc=Local' -path data/migrations up
```
启动脚手架服务：
```shell
# 建议开启GO111MODULE
# go env -w GO111MODULE=on

# 下载依赖
go mod download

# 首次运行会自动复制一份示例配置（config/config.example.yaml）文件到config目录(config/config.yaml)
# 临时启动服务
GO_ENV=development go run main.go server

# 项目起来后执行下面命令访问示例路由
curl "http://127.0.0.1:9001/ping"
# {"message":"pong"}
curl "http://127.0.0.1:9001/api/v1/hello-world"
# {"code":0,"message":"OK","data":{"result":"hello gin-layout"},"cost":"6.151µs"}
curl "http://127.0.0.1:9001/api/v1/hello-world?name=world"
# {"code":0,"message":"OK","data":{"result":"hello world"},"cost":"6.87µs"}
```
更多用法使用以下命令查看:
```shell
go run main.go -h
```

### 部署
```shell
# 打包项目
go build -o cmd/go_layout main.go

# 运行时请配置指定config文件的位置，否则可能会出现找不到配置的情况，修改完配置请重启
cmd/go-layout server -c="指定配置文件位置（/home/go-layout-config.yaml）"

# 使用 supervisord 管理进程配置示例如下
[program:go-layout]
command=/home/go-layout/go_layout -c=/home/go/go-layout/config.yaml
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
        proxy_pass http://172.0.0.1:9001;
    }
}
```

### 生产环境注意事项
> 在构建生产环境时，请配置好 `.yaml` 文件中基础路径 `base_path`，所有的日志记录文件会保存在该目录下的 `{base_path}/gin-layout/logs/` 里面，该基础路径默认为执行命令的目录

### 其他说明
##### 项目中使用到的包
- 核心：[gin](https://github.com/gin-gonic/gin)
- CLI命令管理：[github.com/spf13/cobra](https://github.com/spf13/cobra)
- 配置：[spf13/viper](https://github.com/spf13/viper)
- 参数验证：[github.com/go-playground/validator/v10](https://github.com/go-playground/validator)
- 日志：[go.uber.org/zap](https://github.com/uber-go/zap)、[github.com/natefinch/lumberjack](http://github.com/natefinch/lumberjack)、[github.com/lestrrat-go/file-rotatelogs](https://github.com/lestrrat-go/file-rotatelogs)
- 数据库：[gorm.io/gorm](https://github.com/go-gorm/gorm)、[go-redis/v8](https://github.com/go-redis/redis)
- 还有其他不一一列举，更多请查看`go.mod`文件

### 代码贡献
不完善的地方，欢迎大家 Fork 并提交 PR！