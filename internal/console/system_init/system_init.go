package system_init

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"go.uber.org/zap"

	log "github.com/wannanbigpig/gin-layout/internal/pkg/logger"
	"github.com/wannanbigpig/gin-layout/internal/service/system"
)

var (
	// InitSystemCmd 手动执行初始化系统命令
	InitSystemCmd = &cobra.Command{
		Use:   "init-system",
		Short: "Initialize system data manually",
		Long: `This command manually initializes the system data, which includes:
1. Rollback all database migrations
2. Re-execute all migrations
3. Re-initialize API routes
4. Re-initialize menu-API mappings

This is the same task that runs automatically at 2:00 AM daily.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runInitSystem()
		},
	}
)

// runInitSystem 执行初始化系统
func runInitSystem() error {
	// 用户确认
	if !confirmOperation("此命令将执行系统初始化，包括回滚迁移、重新执行迁移、重新初始化路由和路由映射。此操作会清空现有数据，确定要继续吗？ [Y/N]: ") {
		fmt.Println("操作已取消。")
		return nil
	}

	fmt.Println("开始执行初始化系统任务...")
	log.Logger.Info("手动执行初始化系统任务")

	resetService := system.NewResetService()
	if err := resetService.ReinitializeSystemData(); err != nil {
		log.Logger.Error("初始化系统任务执行失败", zap.Error(err))
		fmt.Printf("初始化系统失败: %v\n", err)
		return err
	}

	fmt.Println("初始化系统任务执行完成！")
	log.Logger.Info("手动执行初始化系统任务完成")
	return nil
}

// confirmOperation 确认操作，返回用户是否确认
func confirmOperation(prompt string) bool {
	scanner := bufio.NewScanner(os.Stdin)
	fmt.Print(prompt)

	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			log.Logger.Error("Failed to read user input", zap.Error(err))
			_, err := fmt.Fprintln(os.Stderr, "reading standard input:", err)
			if err != nil {
				return false
			}
		}
		return false
	}

	input := strings.TrimSpace(strings.ToLower(scanner.Text()))
	return input == "y" || input == "yes"
}
