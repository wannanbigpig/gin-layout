# gin-layout 
### Gin Project Template
> 本项目使用 gin 框架为核心搭建的一个模板，可以基于本项目快速完成业务开发

### 运行
拉取代码后在项目根目录执行如下命令：
```shell
go run cmd/main.go
```

### 目录结构
```
.
|——.gitignore
|——go.mod
|——LICENSE
|——README.md
|——boot    // 项目初始化目录
|  └──boot.go
|——cmd    // 执行命令存放目录
|  └──main.go    // 项目入库 main 包
|——config    // 这里通常维护一些本地调试用的样例配置文件
|  └──autoload    // 配置文件的结构体定义包
|     └──logger.go
|     └──mysql.go
|     └──server.go
|  └──config.go    // 配置初始化文件
|  └──config.example.ini    // .ini 配置示例文件
|  └──config.ini    // .ini 配置文件
|——data    // 数据初始化目录
|  └──mysql.go    // mysql数据库初始化文件
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
```

### 生产环境注意事项
> 在构建生产环境时，请配置好 `.ini` 文件中基础路径 `base_path`，所有的日志记录文件会保存在该目录下的 `/gin-layout/logs/` 里面，该基础路径默认为执行命令的目录