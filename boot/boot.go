package boot

import (
	"flag"
	"fmt"
	"github.com/wannanbigpig/gin-layout/config"
	"github.com/wannanbigpig/gin-layout/data"
	"github.com/wannanbigpig/gin-layout/internal/command"
	"github.com/wannanbigpig/gin-layout/internal/pkg/logger"
	"github.com/wannanbigpig/gin-layout/internal/routers"
	"github.com/wannanbigpig/gin-layout/internal/validator"
	"os"
	"strings"
)

var (
	configPath   string
	printVersion bool
	run          string
)

func init() {
	flag.StringVar(&run, "r", "http", "执行命令默认运行http服务")
	flag.StringVar(&configPath, "c", "", "请输入配置文件绝对路径")
	flag.BoolVar(&printVersion, "v", false, "查看版本")
	flag.Parse()

	if printVersion {
		// 打印版本号
		println(version)
		os.Exit(0)
	}

	// 1、初始化配置
	config.InitConfig(configPath)

	// 2、初始化zap日志
	logger.InitLogger()

	// 3、初始化数据库
	data.InitData()

	// 4、初始化验证器
	validator.InitValidatorTrans("zh")
}

func Run() {
	script := strings.Split(run, ":")
	switch script[0] {
	case "http":
		r := routers.SetRouters()
		err := r.Run(fmt.Sprintf("%s:%d", config.Config.Server.Host, config.Config.Server.Port))
		if err != nil {
			panic(err)
		}
	case "command":
		if len(script) != 2 {
			panic("命令错误，缺少重要参数")
		}
		command.Run(script[1])
	default:
		panic("执行脚本错误")
	}
}
