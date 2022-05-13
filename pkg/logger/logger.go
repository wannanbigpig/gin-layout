package logger

import (
	rotatelogs "github.com/lestrrat-go/file-rotatelogs"
	"github.com/natefinch/lumberjack"
	"github.com/wannanbigpig/gin-layout/config"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"io"
	"log"
	"path/filepath"
	"time"
)

var Logger *zap.Logger

func InitLogger() {
	Logger = createZapLog()
}

// initZapLog 初始化 zap 日志
func createZapLog() *zap.Logger {
	// 非生产环境下生成日志实例
	if config.Config.AppEnv != "prod" {
		if Logger, err := zap.NewDevelopment(); err == nil {
			return Logger
		} else {
			log.Fatal("创建zap日志包失败，详情：" + err.Error())
		}
	}

	encoderConfig := zap.NewProductionEncoderConfig()
	encoderConfig.EncodeTime = func(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
		enc.AppendString(t.Format("2006-01-02 15:04:05.000"))
	}

	// 在日志文件中使用大写字母记录日志级别
	encoderConfig.EncodeLevel = zapcore.CapitalLevelEncoder
	encoder := zapcore.NewConsoleEncoder(encoderConfig)
	filename := filepath.Join(config.Config.StaticBasePath, "/logs/", config.Config.Logger.Filename)
	// 按天切割日志
	// writer := zapcore.AddSync(getRotateWriter(filename))

	// 按文件大小切割日志
	writer := zapcore.AddSync(getLumberJackWriter(filename))
	zapCore := zapcore.NewCore(encoder, writer, zap.InfoLevel)
	// zap.AddStacktrace(zap.WarnLevel)
	return zap.New(zapCore, zap.AddCaller())
}

// getRotateWriter 按日期切割日志
func getRotateWriter(filename string) (hook io.Writer) {
	// 生成 rotateLogs 的 Logger 实际生成的文件名
	// demo.log是指向最新日志的链接
	// 保存15天内的日志，每分割一次日志
	hook, err := rotatelogs.New(
		filename+".%Y%m%d", // 没有使用 go 风格反人类的format格式
		rotatelogs.WithLinkName(filename),
		rotatelogs.WithMaxAge(time.Hour*24*15),
		//rotatelogs.WithRotationTime(time.Hour), // 默认一天
	)

	if err != nil {
		panic(err)
	}
	return hook

}

// getLumberJackWriter 按文件切割日志
func getLumberJackWriter(filename string) (hook io.Writer) {
	// 日志切割配置
	hook = &lumberjack.Logger{
		Filename:   filename,                        //日志文件的位置
		MaxSize:    config.Config.Logger.MaxSize,    //在进行切割之前，日志文件的最大大小（以MB为单位）
		MaxBackups: config.Config.Logger.MaxBackups, //保留旧文件的最大个数
		MaxAge:     config.Config.Logger.MaxAge,     //保留旧文件的最大天数
		Compress:   config.Config.Logger.Compress,   //是否压缩/归档旧文件
	}
	return
}
