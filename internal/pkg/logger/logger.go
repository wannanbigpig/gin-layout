package logger

import (
	"fmt"
	"io"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	rotatelogs "github.com/lestrrat-go/file-rotatelogs"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"

	"github.com/wannanbigpig/gin-layout/config"
)

// swappableCore 是一个可在运行时原子替换底层 zapcore.Core 的包装。
// 借助它，全局 Logger 只需构建一次且永不重新赋值，热重载/测试替换只切换内部 core，
// 从而消除对导出变量 Logger 的并发读写竞争，同时保留 reload 对所有调用点生效。
type swappableCore struct {
	inner atomic.Pointer[zapcore.Core]
}

func (s *swappableCore) load() zapcore.Core {
	if c := s.inner.Load(); c != nil {
		return *c
	}
	return zapcore.NewNopCore()
}

func (s *swappableCore) store(core zapcore.Core) {
	if core == nil {
		core = zapcore.NewNopCore()
	}
	s.inner.Store(&core)
}

func (s *swappableCore) Enabled(level zapcore.Level) bool { return s.load().Enabled(level) }

func (s *swappableCore) With(fields []zapcore.Field) zapcore.Core { return s.load().With(fields) }

func (s *swappableCore) Check(entry zapcore.Entry, ce *zapcore.CheckedEntry) *zapcore.CheckedEntry {
	if s.load().Enabled(entry.Level) {
		return ce.AddCore(entry, s)
	}
	return ce
}

func (s *swappableCore) Write(entry zapcore.Entry, fields []zapcore.Field) error {
	return s.load().Write(entry, fields)
}

func (s *swappableCore) Sync() error { return s.load().Sync() }

var (
	rootCore = &swappableCore{}
	// Logger 仅构建一次，底层 core 通过 rootCore 原子替换；调用点可安全并发读取。
	Logger = zap.New(rootCore, zap.AddCaller())

	loggerOnce sync.Once
	loggerMu   sync.Mutex
	initErr    error
)

// InitLogger 初始化全局日志实例。
func InitLogger() error {
	loggerOnce.Do(func() {
		core, err := buildCore(config.GetConfig())
		if err != nil {
			initErr = fmt.Errorf("创建zap日志包失败: %w", err)
			return
		}
		loggerMu.Lock()
		rootCore.store(core)
		loggerMu.Unlock()
	})
	return initErr
}

// ReloadLogger 根据新配置重建底层日志 core。
func ReloadLogger(cfg *config.Conf) error {
	core, err := buildCore(cfg)
	if err != nil {
		return err
	}
	loggerMu.Lock()
	old := rootCore.load()
	rootCore.store(core)
	loggerMu.Unlock()
	_ = old.Sync()
	return nil
}

func current() *zap.Logger {
	return Logger
}

// ReplaceLoggerForTesting 临时替换全局 logger 的底层 core，并返回恢复函数。
func ReplaceLoggerForTesting(logger *zap.Logger) func() {
	loggerMu.Lock()
	previous := rootCore.load()
	if logger != nil {
		rootCore.store(logger.Core())
	} else {
		rootCore.store(zapcore.NewNopCore())
	}
	loggerMu.Unlock()

	return func() {
		loggerMu.Lock()
		rootCore.store(previous)
		loggerMu.Unlock()
	}
}

// Info 使用当前全局 logger 记录 info 日志。
func Info(msg string, fields ...zap.Field) {
	current().Info(msg, fields...)
}

// Error 使用当前全局 logger 记录 error 日志。
func Error(msg string, fields ...zap.Field) {
	current().Error(msg, fields...)
}

// Warn 使用当前全局 logger 记录 warn 日志。
func Warn(msg string, fields ...zap.Field) {
	current().Warn(msg, fields...)
}

func buildCore(cfg *config.Conf) (zapcore.Core, error) {
	l, err := buildLogger(cfg)
	if err != nil {
		return nil, err
	}
	return l.Core(), nil
}

// buildLogger 初始化 zap 日志
func buildLogger(cfg *config.Conf) (*zap.Logger, error) {
	if cfg == nil {
		cfg = config.GetConfig()
	}

	if cfg.Logger.Output == "stderr" {
		return zap.NewDevelopment()
	}

	encoderConfig := zap.NewProductionEncoderConfig()
	encoderConfig.EncodeTime = func(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
		enc.AppendString(t.Format("2006-01-02 15:04:05.000"))
	}
	encoderConfig.EncodeLevel = zapcore.CapitalLevelEncoder
	encoder := zapcore.NewConsoleEncoder(encoderConfig)
	filename := filepath.Join(cfg.BasePath, "logs", cfg.Logger.Filename)

	var writer zapcore.WriteSyncer
	if cfg.Logger.DefaultDivision == "size" {
		writer = zapcore.AddSync(getLumberJackWriter(cfg, filename))
	} else {
		rotateWriter, err := getRotateWriter(cfg, filename)
		if err != nil {
			return nil, err
		}
		writer = zapcore.AddSync(rotateWriter)
	}

	zapCore := zapcore.NewCore(encoder, writer, zap.InfoLevel)
	return zap.New(zapCore, zap.AddCaller()), nil
}

// getRotateWriter 按日期切割日志
func getRotateWriter(cfg *config.Conf, filename string) (io.Writer, error) {
	maxAge := time.Duration(cfg.Logger.DivisionTime.MaxAge)
	rotationTime := time.Duration(cfg.Logger.DivisionTime.RotationTime)
	hook, err := rotatelogs.New(
		filename+".%Y%m%d",
		rotatelogs.WithLinkName(filename),
		rotatelogs.WithMaxAge(time.Hour*24*maxAge),
		rotatelogs.WithRotationTime(time.Hour*rotationTime),
	)
	if err != nil {
		return nil, err
	}
	return hook, nil
}

// getLumberJackWriter 按文件切割日志
func getLumberJackWriter(cfg *config.Conf, filename string) io.Writer {
	return &lumberjack.Logger{
		Filename:   filename,
		MaxSize:    cfg.Logger.DivisionSize.MaxSize,
		MaxBackups: cfg.Logger.DivisionSize.MaxBackups,
		MaxAge:     cfg.Logger.DivisionSize.MaxAge,
		Compress:   cfg.Logger.DivisionSize.Compress,
	}
}
