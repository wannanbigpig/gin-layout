# gin-layout 
[![Go](https://github.com/wannanbigpig/gin-layout/actions/workflows/go.yml/badge.svg)](https://github.com/wannanbigpig/gin-layout/actions/workflows/go.yml)
[![CodeQL](https://github.com/wannanbigpig/gin-layout/actions/workflows/codeql.yml/badge.svg)](https://github.com/wannanbigpig/gin-layout/actions/workflows/codeql.yml)
[![Sync to Gitee](https://github.com/wannanbigpig/gin-layout/actions/workflows/gitee-sync.yml/badge.svg?branch=master)](https://github.com/wannanbigpig/gin-layout/actions/workflows/gitee-sync.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/wannanbigpig/gin-layout)](https://goreportcard.com/report/github.com/wannanbigpig/gin-layout)
[![GitHub license](https://img.shields.io/github/license/wannanbigpig/gin-layout)](https://github.com/wannanbigpig/gin-layout/blob/master/LICENSE)

Translations: [English](README.md) | [简体中文](README_zh.md)
### Gin Project Template
> Gin framework is used as the core of this project to build a scaffold, based on the project can be quickly completed business development, out of the box 📦

### RUN
Pull the code and execute the following command in the project root directory:
```shell
# You are advised to enable GO111MODULE
# go env -w GO111MODULE=on

# Download the dependent
go mod download

# run
go run cmd/main.go

# When the project gets up, execute the following command to access the sample route
curl "http://127.0.0.1:9999/api/v1/hello-world"
# {"code":0,"message":"OK","data":{"result":"hello gin-layout"},"cost":"6.151µs"}
curl "http://127.0.0.1:9999/api/v1/hello-world?name=world"
# {"code":0,"message":"OK","data":{"result":"hello world"},"cost":"6.87µs"}
```

### The directory structure
```
.
|——.gitignore
|——go.mod
|——go.sum
|——LICENSE
|——README.md
|——boot    // Project initialization directory
|  └──boot.go
|——cmd    // Run the command to save the directory
|  └──main.go    // main
|——config    // Some sample configuration files are usually maintained for local debugging
|  └──autoload    // Structure definition package for configuration files
|     └──app.go
|     └──logger.go
|     └──mysql.go
|     └──redis.go
|     └──server.go
|  └──config.example.ini    // Configuration sample file
|  └──config.example.yaml    // Configuration sample file
|  └──config.go    // Configure the initialization file
|——data    // Data initialization directory
|  └──data.go
|  └──mysql.go
|  └──redis.go
|——internal    // All the code for the service that is not exposed to the public, the usual business logic is below this, use internal to avoid misreferences
|  └──controller    // Controller code
|     └──v1
|        └──auth.go    // Complete process demo code, including database table operations
|        └──helloword.go    // Basic demo code
|     └──base.go
|  └──middleware    // Middleware directory
|     └──cors.go
|     └──logger.go
|     └──recovery.go
|     └──requestCost.go
|  └──model    // Business data access
|     └──admin_users.go
|     └──base.go
|  └──pkg    // Internal use package
|     └──error_code    // Error code definition
|        └──code.go
|        └──en-us.go
|        └──zh-cn.go
|     └──logger    // Log processing
|        └──logger.go
|     └──response    // Unified response output
|        └──response.go
|  └──routers    // Route definition
|     └──apiRouter.go
|     └──router.go
|  └──service    // The business logic
|     └──auth.go
|  └──validator    // Request parameter validator
|     └──form    // Form Parameter Definitions
|        └──auth.go
|     └──validator.go
|——pkg    // A package that can be used externally
|  └──convert    // Data type conversion
|     └──convert.go
|  └──utils    // Help function
|     └──utils.go
```

### Precautions for production environment
> When building the production environment, set the `base_path` in the `.yaml` file. All log files are saved in the `{base_path}/gin-layout/logs/` directory. By default, the base path is the directory where the command is executed

### Other instructions
##### Packages used in the project
- core：[gin](https://github.com/gin-gonic/gin)
- configure：[gopkg.in/yaml.v3](https://github.com/go-yaml/yaml)、[gopkg.in/ini.v1](https://github.com/go-ini/ini)
- parameter validation：[github.com/go-playground/validator/v10](https://github.com/go-playground/validator)、[github.com/natefinch/lumberjack](http://github.com/natefinch/lumberjack)、[github.com/lestrrat-go/file-rotatelogs](https://github.com/lestrrat-go/file-rotatelogs)
- logger：[go.uber.org/zap](https://github.com/uber-go/zap)
- database：[gorm.io/gorm](https://github.com/go-gorm/gorm)、[go-redis/v8](https://github.com/go-redis/redis)
- There are many others, see the 'go.mod' file for more

### Contributions
Any imperfections are welcome to Fork and submit PR!

### License
##### MIT
[![FOSSA Status](https://app.fossa.com/api/projects/git%2Bgithub.com%2Fwannanbigpig%2Fgin-layout.svg?type=large)](https://app.fossa.com/projects/git%2Bgithub.com%2Fwannanbigpig%2Fgin-layout?ref=badge_large)

