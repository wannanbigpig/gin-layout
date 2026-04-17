package cron

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/robfig/cron/v3"
	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"github.com/wannanbigpig/gin-layout/cmd/bootstrapx"
	"github.com/wannanbigpig/gin-layout/data"
	log "github.com/wannanbigpig/gin-layout/internal/pkg/logger"
)

var (
	Cmd = bootstrapx.WrapCommand(&cobra.Command{
		Use:     "cron",
		Short:   "Starting a scheduled task",
		Example: "go-layout cron",
		RunE: func(cmd *cobra.Command, args []string) error {
			return Start()
		},
	}, bootstrapx.Requirements{Data: true})
)

// Start 启动定时任务服务
func Start() error {
	crontab, err := newScheduler()
	if err != nil {
		return err
	}
	if err := registerTasks(crontab); err != nil {
		log.Logger.Error("定时任务启动失败", zap.Error(err))
		return fmt.Errorf("定时任务启动失败: %w", err)
	}

	// 启动定时器
	crontab.Start()

	log.Logger.Info("Cron service started successfully")

	// 优雅关闭
	waitForShutdown()
	stopCtx := crontab.Stop()
	<-stopCtx.Done()
	if err := data.Shutdown(); err != nil {
		return fmt.Errorf("shutdown data resources failed: %w", err)
	}
	log.Logger.Info("Cron service stopped gracefully")
	return nil
}

func newScheduler() (*cron.Cron, error) {
	logger := &cronLogger{}
	scheduler := cron.New(
		cron.WithSeconds(),
		cron.WithChain(cron.Recover(logger)),
	)
	if scheduler == nil {
		return nil, fmt.Errorf("创建定时任务调度器失败")
	}
	return scheduler, nil
}

// waitForShutdown 等待关闭信号，实现优雅关闭
func waitForShutdown() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go handleSignals(cancel)
	<-ctx.Done()
}

// handleSignals 处理系统信号（SIGINT、SIGTERM）
func handleSignals(cancel context.CancelFunc) {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	sig := <-sigChan
	log.Logger.Warn("Received shutdown signal", zap.String("signal", sig.String()))
	cancel()
}

// cronLogger 定时任务日志记录器
type cronLogger struct{}

// Info 记录信息日志
func (cl *cronLogger) Info(msg string, keysAndValues ...interface{}) {
	if len(keysAndValues) > 0 {
		log.Logger.Info(fmt.Sprintf(msg, keysAndValues...))
	} else {
		log.Logger.Info(msg)
	}
}

// Error 记录错误日志
func (cl *cronLogger) Error(err error, msg string, keysAndValues ...interface{}) {
	errorMsg := err.Error()
	if len(keysAndValues) > 0 {
		errorMsg += " " + fmt.Sprintf(msg, keysAndValues...)
	} else if msg != "" {
		errorMsg += " " + msg
	}
	log.Logger.Error(errorMsg, zap.Error(err))
}
