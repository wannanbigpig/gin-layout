package boot

import (
	"flag"
	"github.com/wannanbigpig/gin-layout/config"
	"github.com/wannanbigpig/gin-layout/data"
	"github.com/wannanbigpig/gin-layout/internal/pkg/logger"
	"github.com/wannanbigpig/gin-layout/internal/validator"
	"os"
)

var (
	configPath   string
	printVersion bool
	Run          string
)

func init() {
	flag.StringVar(&Run, "r", "http", "执行命令默认运行http服务")
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
