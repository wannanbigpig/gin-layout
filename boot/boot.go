package boot

import (
	// 如果不要连接数据库，请注释下面一方初始化数据的代码
	_ "github.com/wannanbigpig/gin-layout/data"
	"github.com/wannanbigpig/gin-layout/internal/validator"
	"github.com/wannanbigpig/gin-layout/pkg/logger"
)

func init() {
	// 1、初始化zap日志
	logger.InitLogger()

	// 2、初始化验证器
	if err := validator.InitValidatorTrans("zh"); err != nil {
		panic("验证器初始化失败")
	}
}
