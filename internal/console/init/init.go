package init

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

const (
	msgProcessingComplete     = "Processing complete."
	msgFailedToSaveRoute      = "Failed to save the initial route data to the routing table."
	msgMenuApiMapComplete     = "Menu-API mapping initialization complete."
	msgFailedToInitMenuApiMap = "Failed to initialize menu-API mapping."
)

var (
	ApiRouteCmd = &cobra.Command{
		Use:   "api-route",
		Short: "Initialize API route table",
		Long:  "This command scans all defined API routes in the system and stores them in the a_api table for permission management and API documentation.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runInitApiRoute()
		},
	}

	MenuApiMapCmd = &cobra.Command{
		Use:   "menu-api-map",
		Short: "Initialize menu-API mapping table from casbin_rule table",
		Long:  "This command initializes the a_menu_api_map table by extracting menu-API relationships from the casbin_rule table.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runInitMenuApiMap()
		},
	}
)

func init() {
	// 可以在这里注册其他初始化相关的子命令
}

// runInitApiRoute 执行API路由表初始化
func runInitApiRoute() error {
	// 用户确认
	if !confirmOperation("This command is used to obtain the defined API in the system and store it in the a_api table. Are you sure to perform the operation? [Y/N]: ") {
		fmt.Println("Operation cancelled.")
		return nil
	}

	// 调用服务层方法
	initService := system.NewInitService()
	if err := initService.InitApiRoutes(); err != nil {
		log.Logger.Error(msgFailedToSaveRoute, zap.Error(err))
		fmt.Println(msgFailedToSaveRoute)
		return err
	}

	fmt.Println(msgProcessingComplete)
	return nil
}

// runInitMenuApiMap 执行菜单API映射初始化
func runInitMenuApiMap() error {
	// 用户确认
	if !confirmOperation("This command is used to initialize the menu-API mapping table from casbin_rule table. Are you sure to perform the operation? [Y/N]: ") {
		fmt.Println("Operation cancelled.")
		return nil
	}

	// 调用服务层方法
	initService := system.NewInitService()
	if err := initService.InitMenuApiMap(); err != nil {
		log.Logger.Error(msgFailedToInitMenuApiMap, zap.Error(err))
		fmt.Println(msgFailedToInitMenuApiMap)
		return err
	}

	fmt.Println(msgMenuApiMapComplete)
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
