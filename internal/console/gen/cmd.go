package gen

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/wannanbigpig/gin-layout/data"
)

var (
	tableName     string
	yamlFile      string
	moduleName    string
	menuPID       uint
	menuTitle     string
	menuTitleI18n string
	dryRun        bool
	withFrontend  bool
	frontendDir   string

	// Cmd gen 命令入口。
	Cmd = &cobra.Command{
		Use:   "gen",
		Short: "Generate CRUD code from table or YAML spec",
		Example: `go-layout command gen --table=product --menu-pid=10 --menu-title=商品管理
go-layout command gen --from=gen/specs/product.yaml
go-layout command gen --table=product --dry-run`,
		RunE: runGen,
	}
)

func init() {
	Cmd.Flags().StringVar(&tableName, "table", "", "MySQL table name (reverse engineer from INFORMATION_SCHEMA)")
	Cmd.Flags().StringVar(&yamlFile, "from", "", "YAML spec file path")
	Cmd.Flags().StringVar(&moduleName, "module", "", "Module name (default: table name)")
	Cmd.Flags().UintVar(&menuPID, "menu-pid", 0, "Parent menu ID for menu insertion")
	Cmd.Flags().StringVar(&menuTitle, "menu-title", "", "Menu title")
	Cmd.Flags().StringVar(&menuTitleI18n, "menu-title-i18n", "", "Menu title i18n JSON, e.g. {\"zh-CN\":\"商品管理\",\"en-US\":\"Product\"}")
	Cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Only print generated files, do not write")
	Cmd.Flags().BoolVar(&withFrontend, "with-frontend", false, "Also generate frontend files")
	Cmd.Flags().StringVar(&frontendDir, "frontend-dir", "", "Frontend project directory (default: auto detect)")
}

func runGen(cmd *cobra.Command, args []string) error {
	if tableName == "" && yamlFile == "" {
		return fmt.Errorf("either --table or --from is required")
	}

	// 获取项目根目录
	projectRoot, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	// 获取 Go 模块名
	goModule, err := GetGoModule(projectRoot)
	if err != nil {
		return fmt.Errorf("get go module: %w", err)
	}

	// 加载表结构
	var schema *TableSchema
	if yamlFile != "" {
		schema, err = LoadSchemaFromYAML(yamlFile)
		if err != nil {
			return fmt.Errorf("load yaml: %w", err)
		}
	} else {
		db := data.MysqlDB()
		if db == nil {
			if initErr := data.MysqlInitError(); initErr != nil {
				return fmt.Errorf("--table mode requires MySQL connection: %w", initErr)
			}
			return fmt.Errorf("--table mode requires MySQL connection: mysql is disabled or not configured")
		}
		schema, err = ReadTableSchemaFromDB(db, tableName)
		if err != nil {
			return fmt.Errorf("read table schema: %w", err)
		}
	}

	// 覆盖命令行参数
	if moduleName != "" {
		schema.Module = moduleName
	}
	if menuPID > 0 {
		schema.MenuPID = menuPID
	}
	if menuTitle != "" {
		schema.MenuTitle = menuTitle
	}
	if menuTitleI18n != "" {
		schema.MenuTitleI18n = menuTitleI18n
	}

	// 设置默认前端目录
	if frontendDir == "" {
		frontendDir = filepath.Join(filepath.Dir(projectRoot), "x-l-admin-vue3")
	}

	// 创建生成器
	gen := NewGenerator(schema, goModule, projectRoot, dryRun, withFrontend, frontendDir)

	// 执行生成
	fmt.Printf("Generating CRUD for module: %s (table: %s)\n", schema.Module, schema.TableName)
	fmt.Println("---")

	if err := gen.Generate(); err != nil {
		return fmt.Errorf("generate: %w", err)
	}

	fmt.Println("\nDone! Please:")
	fmt.Println("1. Add the controller to internal/routers/deps.go")
	fmt.Printf("2. Add %sGroup(deps) to adminAuthGroup in admin_router.go\n", schema.Module)
	fmt.Println("3. Run: go run main.go -c ./config.yaml command api-route -y")
	fmt.Println("4. Run: go run main.go -c ./config.yaml command rebuild-user-permissions -y")
	fmt.Println("5. Run the migration: go run main.go -c ./config.yaml command migrate up")

	return nil
}
