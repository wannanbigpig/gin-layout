# gin-layout 
[![Go](https://github.com/wannanbigpig/gin-layout/actions/workflows/go.yml/badge.svg)](https://github.com/wannanbigpig/gin-layout/actions/workflows/go.yml)
[![CodeQL](https://github.com/wannanbigpig/gin-layout/actions/workflows/codeql.yml/badge.svg)](https://github.com/wannanbigpig/gin-layout/actions/workflows/codeql.yml)
[![Sync to Gitee](https://github.com/wannanbigpig/gin-layout/actions/workflows/gitee-sync.yml/badge.svg?branch=master)](https://github.com/wannanbigpig/gin-layout/actions/workflows/gitee-sync.yml)
[![FOSSA Status](https://app.fossa.com/api/projects/git%2Bgithub.com%2Fwannanbigpig%2Fgin-layout.svg?type=shield)](https://app.fossa.com/projects/git%2Bgithub.com%2Fwannanbigpig%2Fgin-layout?ref=badge_shield)


Translations: [English](README.md) | [简体中文](README_zh.md)
### Gin Project Template
> Gin framework is used as the core of this project to build a template, based on this project can quickly complete business development

### RUN
Pull the code and execute the following command in the project root directory:
```shell
go run cmd/main.go
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
|     └──server.go
|  └──config.go    // Configure the initialization file
|  └──config.example.ini    // Configuration sample file
|  └──config.example.yaml    // Configuration sample file
|——data    // Data initialization directory
|  └──mysql.go
|  └──data.go
|——internal    // All the code for the service that is not exposed to the public, the usual business logic is below this, use internal to avoid misreferences
|  └──controller    // Controller code
|     └──v1
|        └──auth.go    // Complete process demo code, including database table operations
|        └──helloword.go    // Basic demo code
|     └──base.go
|  └──middleware    // Middleware directory
|     └──cors.go
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
|  └──logger    // Log processing
|     └──logger.go
|  └──response    // Response processing
|     └──response.go
|  └──utils    // Help function
|     └──utils.go
```

### Precautions for production environment
> When building the production environment, set the `base_path` in the `.yaml` file. All log files are saved in the `{base_path}/gin-layout/logs/` directory. By default, the base path is the directory where the command is executed

## License
[![FOSSA Status](https://app.fossa.com/api/projects/git%2Bgithub.com%2Fwannanbigpig%2Fgin-layout.svg?type=large)](https://app.fossa.com/projects/git%2Bgithub.com%2Fwannanbigpig%2Fgin-layout?ref=badge_large)