package system_init

import (
	"fmt"

	"github.com/spf13/cobra"
	"go.uber.org/zap"

	consolex "github.com/wannanbigpig/gin-layout/internal/console"
	log "github.com/wannanbigpig/gin-layout/internal/pkg/logger"
	"github.com/wannanbigpig/gin-layout/internal/service/system"
)

var (
	initSystemAssumeYes bool

	// InitSystemCmd 手动执行初始化系统命令
	InitSystemCmd = &cobra.Command{
		Use:   "init-system",
		Short: "Initialize system data manually",
		Long: `This command manually initializes the system data, which includes:
1. Rollback all database migrations
2. Re-execute all migrations
3. Re-initialize API routes
4. Rebuild final user API permissions

This is the same task that runs automatically at 2:00 AM daily.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runInitSystem()
		},
	}
)

func init() {
	InitSystemCmd.Flags().BoolVarP(&initSystemAssumeYes, "yes", "y", false, "Skip confirmation prompt")
}

// runInitSystem 执行初始化系统
func runInitSystem() error {
	// 用户确认
	if !consolex.ConfirmOperation("此命令将执行系统初始化，包括回滚迁移、重新执行迁移、重新初始化 API 路由并重建用户最终 API 权限。此操作会清空现有数据，确定要继续吗？ [Y/N]: ", initSystemAssumeYes) {
		fmt.Println("操作已取消。")
		return nil
	}

	fmt.Println("开始执行初始化系统任务...")
	log.Logger.Info("手动执行初始化系统任务")

	if err := system.ReinitializeSystemData(); err != nil {
		log.Logger.Error("初始化系统任务执行失败", zap.Error(err))
		fmt.Printf("初始化系统失败: %v\n", err)
		return err
	}

	fmt.Println("初始化系统任务执行完成！")
	log.Logger.Info("手动执行初始化系统任务完成")
	return nil
}
