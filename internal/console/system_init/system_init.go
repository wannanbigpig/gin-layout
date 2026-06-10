package system_init

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"github.com/wannanbigpig/gin-layout/config"
	log "github.com/wannanbigpig/gin-layout/internal/pkg/logger"
	"github.com/wannanbigpig/gin-layout/internal/service/system"
)

const (
	ansiRed                = "\033[31m"
	ansiBoldRed            = "\033[1;31m"
	ansiReset              = "\033[0m"
	backupConfirmationText = "已备份"
)

var (
	initSystemAssumeYes bool
	initSystemDatabase  string
	initSystemBackupOK  bool

	errInitSystemCancelled = errors.New("init-system cancelled")

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
			return runInitSystem(os.Stdin, os.Stdout)
		},
	}
)

func init() {
	InitSystemCmd.Flags().BoolVarP(&initSystemAssumeYes, "yes", "y", false, "Skip ordinary confirmation prompt; destructive database confirmation is still required")
	InitSystemCmd.Flags().StringVar(&initSystemDatabase, "confirm-database", "", "Database name that must match mysql.database to confirm destructive reset")
	InitSystemCmd.Flags().BoolVar(&initSystemBackupOK, "backup-confirmed", false, "Confirm that the target database has been backed up before destructive reset")
}

// runInitSystem 执行初始化系统
func runInitSystem(in io.Reader, out io.Writer) error {
	cfg := config.GetConfig()
	reader := bufio.NewReader(in)
	printDangerousResetWarning(out, cfg)

	if !initSystemAssumeYes {
		ok, err := readYesNo(reader, out, "此命令将清空当前数据库对象并重建系统数据。是否继续？ [Y/N]: ")
		if err != nil {
			return err
		}
		if !ok {
			fmt.Fprintln(out, "操作已取消。")
			return nil
		}
	}

	if err := confirmDangerousReset(reader, out, cfg); err != nil {
		if errors.Is(err, errInitSystemCancelled) {
			fmt.Fprintln(out, "操作已取消。")
			return nil
		}
		return err
	}

	fmt.Fprintln(out, "开始执行初始化系统任务...")
	log.Logger.Info("手动执行初始化系统任务")

	if err := system.ReinitializeSystemData(); err != nil {
		log.Logger.Error("初始化系统任务执行失败", zap.Error(err))
		fmt.Fprintf(out, "初始化系统失败: %v\n", err)
		return err
	}

	fmt.Fprintln(out, "初始化系统任务执行完成！")
	log.Logger.Info("手动执行初始化系统任务完成")
	return nil
}

func printDangerousResetWarning(out io.Writer, cfg *config.Conf) {
	fmt.Fprintln(out, colorRed("============================================================", true))
	fmt.Fprintln(out, colorRed("危险操作：init-system 会删除当前配置数据库中的所有对象和数据，然后重新执行 migrations。", true))
	fmt.Fprintln(out, colorRed("请确认目标库无误，并且已经完成可恢复的数据备份。", true))
	fmt.Fprintln(out, colorRed("============================================================", true))
	fmt.Fprintln(out, "当前配置：")
	fmt.Fprintf(out, "  app_env: %s\n", valueOrUnknown(appEnv(cfg)))
	fmt.Fprintf(out, "  debug: %v\n", cfg != nil && cfg.Debug)
	fmt.Fprintf(out, "  base_url: %s\n", valueOrUnknown(baseURL(cfg)))
	fmt.Fprintln(out, "当前 MySQL：")
	fmt.Fprintf(out, "  enable: %v\n", cfg != nil && cfg.Mysql.Enable)
	fmt.Fprintf(out, "  host: %s\n", valueOrUnknown(mysqlHost(cfg)))
	fmt.Fprintf(out, "  port: %d\n", mysqlPort(cfg))
	fmt.Fprintf(out, "  database: %s\n", valueOrUnknown(mysqlDatabase(cfg)))
	fmt.Fprintf(out, "  username: %s\n", valueOrUnknown(mysqlUsername(cfg)))
	fmt.Fprintf(out, "  password: (hidden)\n")
	fmt.Fprintf(out, "  table_prefix: %s\n", valueOrEmpty(mysqlTablePrefix(cfg)))
	fmt.Fprintln(out)
}

func confirmDangerousReset(reader *bufio.Reader, out io.Writer, cfg *config.Conf) error {
	expectedDatabase := strings.TrimSpace(mysqlDatabase(cfg))
	if expectedDatabase == "" {
		return fmt.Errorf("mysql.database 为空，拒绝执行 init-system")
	}

	if !initSystemBackupOK {
		backupText, err := readLine(reader, out, fmt.Sprintf("请确认已经备份目标库。请输入 %q 继续: ", backupConfirmationText))
		if err != nil {
			return err
		}
		if backupText != backupConfirmationText {
			return errInitSystemCancelled
		}
	} else {
		fmt.Fprintln(out, "已通过 --backup-confirmed 确认备份。")
	}

	inputDatabase := strings.TrimSpace(initSystemDatabase)
	if inputDatabase == "" {
		var err error
		inputDatabase, err = readLine(reader, out, fmt.Sprintf("请再次输入当前数据库名 %q 继续清空并重建: ", expectedDatabase))
		if err != nil {
			return err
		}
	}
	if inputDatabase != expectedDatabase {
		return fmt.Errorf("数据库确认失败：输入 %q 与当前配置 mysql.database=%q 不一致", inputDatabase, expectedDatabase)
	}

	return nil
}

func readYesNo(reader *bufio.Reader, out io.Writer, prompt string) (bool, error) {
	input, err := readLine(reader, out, prompt)
	if err != nil {
		return false, err
	}
	switch strings.ToLower(input) {
	case "y", "yes":
		return true, nil
	default:
		return false, nil
	}
}

func readLine(reader *bufio.Reader, out io.Writer, prompt string) (string, error) {
	fmt.Fprint(out, prompt)
	input, err := reader.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		log.Logger.Error("Failed to read user input", zap.Error(err))
		_, _ = fmt.Fprintln(os.Stderr, "reading standard input:", err)
		return "", err
	}
	return strings.TrimSpace(input), nil
}

func colorRed(message string, bold bool) string {
	if bold {
		return ansiBoldRed + message + ansiReset
	}
	return ansiRed + message + ansiReset
}

func appEnv(cfg *config.Conf) string {
	if cfg == nil {
		return ""
	}
	return cfg.AppEnv
}

func baseURL(cfg *config.Conf) string {
	if cfg == nil {
		return ""
	}
	return cfg.BaseURL
}

func mysqlHost(cfg *config.Conf) string {
	if cfg == nil {
		return ""
	}
	return cfg.Mysql.Host
}

func mysqlPort(cfg *config.Conf) uint16 {
	if cfg == nil {
		return 0
	}
	return cfg.Mysql.Port
}

func mysqlDatabase(cfg *config.Conf) string {
	if cfg == nil {
		return ""
	}
	return cfg.Mysql.Database
}

func mysqlUsername(cfg *config.Conf) string {
	if cfg == nil {
		return ""
	}
	return cfg.Mysql.Username
}

func mysqlTablePrefix(cfg *config.Conf) string {
	if cfg == nil {
		return ""
	}
	return cfg.Mysql.TablePrefix
}

func valueOrUnknown(value string) string {
	if strings.TrimSpace(value) == "" {
		return "(unknown)"
	}
	return value
}

func valueOrEmpty(value string) string {
	if strings.TrimSpace(value) == "" {
		return "(empty)"
	}
	return value
}
