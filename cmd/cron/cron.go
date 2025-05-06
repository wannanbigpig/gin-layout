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

func Start() {
	myLog := myLogger{}
	// 初始化定时器
	crontab := cron.New(cron.WithSeconds(), cron.WithChain(cron.Recover(myLog)))

	// 添加任务
	job := cron.NewChain(cron.SkipIfStillRunning(myLog), cron.Recover(myLog)).Then(cron.FuncJob(runTask))
	_, err := crontab.AddJob("0/5 * * * * *", job)
	if err != nil {
		log.Logger.Error("Error adding job:", zap.Error(err))
		os.Exit(1) // 优雅退出
	}

	// 启动定时器
	crontab.Start()

	// 监听系统信号以实现优雅关闭
	ctx, cancel := context.WithCancel(context.Background())
	go handleSignals(cancel)

	<-ctx.Done()   // 等待关闭信号
	crontab.Stop() // 停止定时器
	log.Logger.Info("Cron service stopped gracefully")
}

// 定义任务逻辑
func runTask() {
	log.Logger.Info("计划任务 demo 执行：", zap.String("time", time.Now().Format("2006-01-02 15:04:05")))
}

// 处理系统信号
func handleSignals(cancel context.CancelFunc) {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan
	log.Logger.Warn("Received shutdown signal, stopping cron service...")
	cancel()
}

type myLogger struct {
}

func (ml myLogger) Info(msg string, keysAndValues ...interface{}) {
	log.Logger.Info(fmt.Sprintf(msg, keysAndValues...))
}

func (ml myLogger) Error(err error, msg string, keysAndValues ...interface{}) {
	log.Logger.Error(err.Error() + fmt.Sprintf(msg, keysAndValues...))
}
