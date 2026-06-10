package gen

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	"gopkg.in/yaml.v3"
)

// Generator 代码生成器。
type Generator struct {
	schema       *TableSchema
	goModule     string
	projectRoot  string
	dryRun       bool
	frontendDir  string
	withFrontend bool
}

// NewGenerator 创建生成器实例。
func NewGenerator(schema *TableSchema, goModule, projectRoot string, dryRun, withFrontend bool, frontendDir string) *Generator {
	return &Generator{
		schema:       schema,
		goModule:     goModule,
		projectRoot:  projectRoot,
		dryRun:       dryRun,
		frontendDir:  frontendDir,
		withFrontend: withFrontend,
	}
}

// Generate 执行代码生成。
func (g *Generator) Generate() error {
	data := buildTemplateData(g.schema, g.goModule)

	// 后端文件
	backendFiles := []struct {
		template string
		output   string
	}{
		{"model.go.tmpl", filepath.Join("internal", "model", data.ModuleSnake+".go")},
		{"service.go.tmpl", filepath.Join("internal", "service", data.ModuleSnake, data.ModuleSnake+".go")},
		{"form.go.tmpl", filepath.Join("internal", "validator", "form", data.ModuleSnake+".go")},
		{"controller.go.tmpl", filepath.Join("internal", "controller", "admin_v1", "auth_"+data.ModuleSnake+".go")},
		{"resource.go.tmpl", filepath.Join("internal", "resources", data.ModuleSnake+".go")},
	}

	for _, f := range backendFiles {
		if err := g.renderAndWrite(f.template, f.output, data); err != nil {
			return fmt.Errorf("generate %s: %w", f.output, err)
		}
	}

	// Migration 文件
	if err := g.generateMigration(data); err != nil {
		return fmt.Errorf("generate migration: %w", err)
	}

	// 路由片段（打印到终端）
	g.printRouteSnippet(data)
	g.printDepsSnippet(data)
	g.printMenuData(data)
	g.printDefaultMenuAPIBinding(data)

	// 前端文件
	if g.withFrontend {
		if err := g.generateFrontend(data); err != nil {
			return fmt.Errorf("generate frontend: %w", err)
		}
	}

	return nil
}

// renderAndWrite 渲染模板并写入文件。
func (g *Generator) renderAndWrite(tmplName, outputPath string, data any) error {
	fullOutputPath := filepath.Join(g.projectRoot, outputPath)

	// 检查文件是否存在，存在则跳过
	if _, err := os.Stat(fullOutputPath); err == nil {
		fmt.Printf("[SKIP] %s (already exists)\n", outputPath)
		return nil
	}

	if g.dryRun {
		fmt.Printf("[DRY-RUN] %s\n", outputPath)
		return nil
	}

	// 读取模板
	tmplPath := filepath.Join(g.projectRoot, "internal", "console", "gen", "templates", tmplName)
	tmplContent, err := os.ReadFile(tmplPath)
	if err != nil {
		return fmt.Errorf("read template %s: %w", tmplName, err)
	}

	// 解析模板
	tmpl, err := template.New(tmplName).Parse(string(tmplContent))
	if err != nil {
		return fmt.Errorf("parse template %s: %w", tmplName, err)
	}

	// 创建输出目录
	if err := os.MkdirAll(filepath.Dir(fullOutputPath), 0o755); err != nil {
		return fmt.Errorf("create dir: %w", err)
	}

	// 渲染
	f, err := os.Create(fullOutputPath)
	if err != nil {
		return fmt.Errorf("create file: %w", err)
	}
	defer f.Close()

	if err := tmpl.Execute(f, data); err != nil {
		return fmt.Errorf("execute template: %w", err)
	}

	fmt.Printf("[OK] %s\n", outputPath)
	return nil
}

// generateMigration 生成迁移文件。
func (g *Generator) generateMigration(data *TemplateData) error {
	ts := time.Now().Format("20060102150405")

	upFile := fmt.Sprintf("%s_create_%s.up.sql", ts, data.TableName)
	downFile := fmt.Sprintf("%s_create_%s.down.sql", ts, data.TableName)

	migrationData := struct {
		*TemplateData
		Comment string
	}{
		TemplateData: data,
		Comment:      g.schema.Comment,
	}

	for _, f := range []struct {
		template string
		output   string
	}{
		{"migration.up.sql.tmpl", upFile},
		{"migration.down.sql.tmpl", downFile},
	} {
		outputPath := filepath.Join("data", "migrations", f.output)
		if err := g.renderAndWrite(f.template, outputPath, migrationData); err != nil {
			return err
		}
	}

	return nil
}

// printRouteSnippet 打印路由片段。
func (g *Generator) printRouteSnippet(data *TemplateData) {
	tmplPath := filepath.Join(g.projectRoot, "internal", "console", "gen", "templates", "route_group.go.tmpl")
	tmplContent, err := os.ReadFile(tmplPath)
	if err != nil {
		fmt.Printf("[WARN] read route template failed: %v\n", err)
		return
	}

	tmpl, err := template.New("route").Parse(string(tmplContent))
	if err != nil {
		fmt.Printf("[WARN] parse route template failed: %v\n", err)
		return
	}

	var buf strings.Builder
	if err := tmpl.Execute(&buf, data); err != nil {
		fmt.Printf("[WARN] execute route template failed: %v\n", err)
		return
	}

	fmt.Println("\n========== 路由片段 ==========")
	fmt.Println("// 将以下代码添加到 internal/routers/admin_router.go")
	fmt.Println(buf.String())
	fmt.Println("// 将以下行添加到 adminAuthGroup 的 Children 中:")
	fmt.Printf("// %sGroup(deps),\n", data.ModuleSnake)
	fmt.Println("================================")
}

// printDepsSnippet 打印依赖片段。
func (g *Generator) printDepsSnippet(data *TemplateData) {
	fmt.Println("\n========== deps.go 片段 ==========")
	fmt.Println("// 1. 在 ControllerDeps 结构体中添加:")
	fmt.Printf("// %s *admin_v1.%s\n", data.ControllerName, data.ControllerName)
	fmt.Println("//")
	fmt.Println("// 2. 在 DefaultControllerDeps() 中添加:")
	fmt.Printf("// %s: admin_v1.New%s(),\n", data.ControllerName, data.ControllerName)
	fmt.Println("//")
	fmt.Println("// 3. 在 normalizeControllerDeps() 中添加:")
	fmt.Printf("// if deps.%s == nil {\n", data.ControllerName)
	fmt.Printf("//     deps.%s = defaultDeps.%s\n", data.ControllerName, data.ControllerName)
	fmt.Println("// }")
	fmt.Println("===================================")
}

// printMenuData 打印菜单数据。
func (g *Generator) printMenuData(data *TemplateData) {
	fmt.Println("\n========== 菜单数据 ==========")
	fmt.Println("// 以下 SQL 用于在 menu 表中插入菜单和按钮记录")
	fmt.Printf("// 父菜单 ID: %d\n", data.MenuPID)
	fmt.Printf("// 菜单标题: %s\n", data.MenuTitle)
	if data.MenuTitleI18n != "" {
		fmt.Printf("// 菜单标题 i18n: %s\n", data.MenuTitleI18n)
	}
	fmt.Println()
	fmt.Printf("-- 菜单记录 (Type=2)\n")
	fmt.Printf("INSERT INTO `menu` (`pid`, `type`, `title`, `title_i18n`, `path`, `component_key`, `sort`, `is_auth`, `is_show`, `is_cache`) VALUES\n")
	titleI18n := data.MenuTitleI18n
	if titleI18n == "" {
		titleI18n = fmt.Sprintf(`{"zh-CN":"%s","en-US":"%s"}`, data.MenuTitle, toPascalCase(data.ModuleSnake))
	}
	fmt.Printf("(%d, 2, '%s', '%s', '/%s', '%s', 0, 1, 1, 0);\n",
		data.MenuPID, data.MenuTitle, titleI18n, data.ModuleSnake, data.ComponentKey)
	fmt.Println()
	fmt.Printf("-- 按钮记录 (Type=3)\n")
	for _, btn := range []struct {
		code  string
		title string
	}{
		{data.ModuleSnake + ":list", data.MenuTitle + "列表"},
		{data.ModuleSnake + ":create", "新增" + data.MenuTitle},
		{data.ModuleSnake + ":update", "编辑" + data.MenuTitle},
		{data.ModuleSnake + ":delete", "删除" + data.MenuTitle},
	} {
		fmt.Printf("INSERT INTO `menu` (`pid`, `type`, `title`, `code`, `sort`, `is_auth`) VALUES\n")
		fmt.Printf("((SELECT id FROM (SELECT id FROM `menu` WHERE `component_key` = '%s' AND `type` = 2 LIMIT 1) AS tmp), 3, '%s', '%s', 0, 1);\n",
			data.ComponentKey, btn.title, btn.code)
	}
	fmt.Println("================================")
}

// printDefaultMenuAPIBinding 打印默认菜单 API 绑定。
func (g *Generator) printDefaultMenuAPIBinding(data *TemplateData) {
	fmt.Println("\n========== defaultMenuAPIBinding 片段 ==========")
	fmt.Println("// 将以下绑定添加到 internal/service/access/menu_api_defaults.go 的 builtInDefaultMenuAPIBindings 中")
	fmt.Printf("// %s:list → GET /admin/v1/%s/list, GET /admin/v1/%s/options, GET /admin/v1/%s/detail\n",
		data.ModuleSnake, data.TableName, data.TableName, data.TableName)
	fmt.Printf("// %s:create → POST /admin/v1/%s/create\n", data.ModuleSnake, data.TableName)
	fmt.Printf("// %s:update → POST /admin/v1/%s/update\n", data.ModuleSnake, data.TableName)
	fmt.Printf("// %s:delete → POST /admin/v1/%s/delete\n", data.ModuleSnake, data.TableName)
	fmt.Println("===================================================")
}

// generateFrontend 生成前端文件。
func (g *Generator) generateFrontend(data *TemplateData) error {
	frontendFiles := []struct {
		template string
		output   string
	}{
		{"api.ts.tmpl", filepath.Join("src", "api", data.ModuleSnake+".ts")},
		{"types.d.ts.tmpl", filepath.Join("src", "types", data.ModuleSnake+".d.ts")},
		{"index.vue.tmpl", filepath.Join("src", "views", data.ModuleSnake, "index.vue")},
	}

	for _, f := range frontendFiles {
		fullPath := filepath.Join(g.frontendDir, f.output)

		// 检查文件是否存在
		if _, err := os.Stat(fullPath); err == nil {
			fmt.Printf("[SKIP] frontend/%s (already exists)\n", f.output)
			continue
		}

		if g.dryRun {
			fmt.Printf("[DRY-RUN] frontend/%s\n", f.output)
			continue
		}

		// 读取模板
		tmplPath := filepath.Join(g.projectRoot, "internal", "console", "gen", "templates", f.template)
		tmplContent, err := os.ReadFile(tmplPath)
		if err != nil {
			return err
		}

		// 准备前端模板数据
		frontendData := struct {
			*TemplateData
			TsTypeMap map[string]string
		}{
			TemplateData: data,
		}

		// 解析模板
		funcMap := template.FuncMap{
			"tsType": func(goType string) string {
				return mapGoToTsType(goType)
			},
		}

		tmpl, err := template.New(f.template).Funcs(funcMap).Parse(string(tmplContent))
		if err != nil {
			return err
		}

		// 创建目录
		if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
			return err
		}

		// 写入
		out, err := os.Create(fullPath)
		if err != nil {
			return err
		}

		if err := tmpl.Execute(out, frontendData); err != nil {
			out.Close()
			return err
		}
		out.Close()

		fmt.Printf("[OK] frontend/%s\n", f.output)
	}

	return nil
}

// mapGoToTsType 将 Go 类型映射为 TypeScript 类型。
func mapGoToTsType(goType string) string {
	switch goType {
	case "string", "utils.FormatDate", "datatypes.JSON":
		return "string"
	case "uint8", "uint16", "uint", "uint64", "int", "int16", "int64":
		return "number"
	default:
		return "any"
	}
}

// LoadSchemaFromYAML 从 YAML 文件加载表结构。
func LoadSchemaFromYAML(path string) (*TableSchema, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read yaml: %w", err)
	}

	var schema TableSchema
	if err := yaml.Unmarshal(data, &schema); err != nil {
		return nil, fmt.Errorf("parse yaml: %w", err)
	}

	// 验证必填字段
	if schema.TableName == "" {
		return nil, fmt.Errorf("table_name is required")
	}
	if schema.Module == "" {
		schema.Module = schema.TableName
	}
	if schema.MenuTitle == "" {
		schema.MenuTitle = schema.Module
	}

	return &schema, nil
}

// GetGoModule 从 go.mod 获取模块名。
func GetGoModule(projectRoot string) (string, error) {
	gomodPath := filepath.Join(projectRoot, "go.mod")
	data, err := os.ReadFile(gomodPath)
	if err != nil {
		return "", fmt.Errorf("read go.mod: %w", err)
	}

	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "module ") {
			return strings.TrimPrefix(line, "module "), nil
		}
	}

	return "", fmt.Errorf("module not found in go.mod")
}
