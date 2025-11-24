package cron

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/robfig/cron/v3"
	"github.com/spf13/cobra"
	"go.uber.org/zap"

	log "github.com/wannanbigpig/gin-layout/internal/pkg/logger"
)

const (
	// cronSchedule 定时任务执行计划（每5秒执行一次）
	cronSchedule = "0/5 * * * * *"
	// timeFormat 时间格式
	timeFormat = "2006-01-02 15:04:05"
)

var (
	Cmd = &cobra.Command{
		Use:     "cron",
		Short:   "Starting a scheduled task",
		Example: "go-layout cron",
		PreRun: func(cmd *cobra.Command, args []string) {
			// 计划任务中使用数据请先初始化数据库链接
			// data.InitData()
		},
		Run: func(cmd *cobra.Command, args []string) {
			Start()
		},
	}
)

// Start 启动定时任务服务
func Start() {
	// 初始化定时器
	crontab := createCronScheduler()

	// 添加任务
	if err := addCronJob(crontab); err != nil {
		log.Logger.Error("Failed to add cron job", zap.Error(err))
		os.Exit(1)
	}

	// 启动定时器
	crontab.Start()
	defer crontab.Stop()

	log.Logger.Info("Cron service started successfully")

	// 优雅关闭
	waitForShutdown()
	log.Logger.Info("Cron service stopped gracefully")
}

// createCronScheduler 创建定时任务调度器
func createCronScheduler() *cron.Cron {
	myLog := &cronLogger{}
	return cron.New(
		cron.WithSeconds(),
		cron.WithChain(cron.Recover(myLog)),
	)
}

// addCronJob 添加定时任务
func addCronJob(crontab *cron.Cron) error {
	myLog := &cronLogger{}
	job := cron.NewChain(
		cron.SkipIfStillRunning(myLog),
		cron.Recover(myLog),
	).Then(cron.FuncJob(runTask))

	_, err := crontab.AddJob(cronSchedule, job)
	return err
}

// waitForShutdown 等待关闭信号，实现优雅关闭
func waitForShutdown() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go handleSignals(cancel)
	<-ctx.Done()
}

// runTask 执行定时任务
func runTask() {
	log.Logger.Info("计划任务 demo 执行：", zap.String("time", time.Now().Format(timeFormat)))
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
