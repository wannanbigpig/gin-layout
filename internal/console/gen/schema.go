package gen

import (
	"fmt"
	"strings"
)

// TableSchema 描述一张表的完整结构。
type TableSchema struct {
	TableName   string        `yaml:"table_name"`
	Module      string        `yaml:"module"`
	Comment     string        `yaml:"comment"`
	MenuPID     uint          `yaml:"menu_pid"`
	MenuTitle   string        `yaml:"menu_title"`
	MenuTitleI18n string      `yaml:"menu_title_i18n"`
	Fields      []FieldSchema `yaml:"fields"`
}

// FieldSchema 描述一个字段的结构。
type FieldSchema struct {
	Name       string `yaml:"name"`
	ColumnName string `yaml:"column_name"`
	ColumnType string `yaml:"column_type"`
	GoType     string `yaml:"go_type"`
	GoTag      string `yaml:"go_tag"`
	Comment    string `yaml:"comment"`
	IsNullable bool   `yaml:"is_nullable"`
	IsPrimary  bool   `yaml:"is_primary"`
	MaxLength  int    `yaml:"max_length"`
	EnumValues string `yaml:"enum_values"`
	DefaultVal string `yaml:"default_val"`
	IsIndex    bool   `yaml:"is_index"`
}

// TemplateData 传递给模板的完整数据结构。
type TemplateData struct {
	// 命名
	TableName     string // 数据库表名，如 product
	ModuleSnake   string // 模块蛇形名，如 product
	ModelName     string // Go 模型名，如 Product
	ServiceName   string // 服务名，如 ProductService
	ControllerName string // 控制器名，如 ProductController
	ResourceName  string // 资源名，如 ProductResources
	VarName       string // 局部变量名，如 product
	ComponentKey  string // 组件 key，如 product:index
	MenuTitle     string // 菜单标题
	MenuTitleI18n string // 菜单标题 i18n JSON
	MenuPID       uint   // 父菜单 ID

	// 字段
	Fields      []FieldData
	ListFields  []FieldData // 可搜索字段
	FormFields  []FieldData // 表单字段（排除 id/created_at/updated_at/deleted_at）
	HasStatus   bool
	HasDeptID   bool
	HasCreatedBy bool

	// 包路径
	GoModule string // Go 模块名，如 github.com/wannanbigpig/gin-layout
}

// FieldData 传递给模板的字段数据。
type FieldData struct {
	Name         string // Go 字段名，如 Name
	ColumnName   string // 数据库列名，如 name
	GoType       string // Go 类型，如 string
	JsonTag      string // json tag，如 name
	GormTag      string // gorm tag
	BindingTag   string // binding tag
	Comment      string // 注释
	IsSearchable bool   // 是否可搜索
	IsForm       bool   // 是否表单字段
	FormType     string // 前端控件类型，如 el-input
	EnumValues   string // 枚举值，如 "0 1"
	MaxLength    int    // 最大长度
}

// buildTemplateData 从 TableSchema 构建 TemplateData。
func buildTemplateData(schema *TableSchema, goModule string) *TemplateData {
	data := &TemplateData{
		TableName:      schema.TableName,
		ModuleSnake:    schema.Module,
		ModelName:      toPascalCase(schema.Module),
		ServiceName:    toPascalCase(schema.Module) + "Service",
		ControllerName: toPascalCase(schema.Module) + "Controller",
		ResourceName:   toPascalCase(schema.Module) + "Resources",
		VarName:        toCamelCase(schema.Module),
		ComponentKey:   schema.Module + ":index",
		MenuTitle:      schema.MenuTitle,
		MenuTitleI18n:  schema.MenuTitleI18n,
		MenuPID:        schema.MenuPID,
		GoModule:       goModule,
	}

	for _, f := range schema.Fields {
		fd := buildFieldData(f)
		data.Fields = append(data.Fields, fd)

		if fd.IsSearchable {
			data.ListFields = append(data.ListFields, fd)
		}
		if fd.IsForm {
			data.FormFields = append(data.FormFields, fd)
		}
		if f.ColumnName == "status" {
			data.HasStatus = true
		}
		if f.ColumnName == "dept_id" {
			data.HasDeptID = true
		}
		if f.ColumnName == "created_by" {
			data.HasCreatedBy = true
		}
	}

	return data
}

// buildFieldData 从 FieldSchema 构建 FieldData。
func buildFieldData(f FieldSchema) FieldData {
	fd := FieldData{
		Name:       f.Name,
		ColumnName: f.ColumnName,
		GoType:     f.GoType,
		JsonTag:    f.ColumnName,
		Comment:    f.Comment,
		MaxLength:  f.MaxLength,
		EnumValues: f.EnumValues,
	}

	// 判断是否可搜索
	searchableColumns := map[string]bool{
		"name":   true,
		"title":  true,
		"code":   true,
		"status": true,
		"keyword": true,
	}
	fd.IsSearchable = searchableColumns[f.ColumnName] || strings.HasSuffix(f.ColumnName, "_name")

	// 判断是否表单字段
	excludedColumns := map[string]bool{
		"id":         true,
		"created_at": true,
		"updated_at": true,
		"deleted_at": true,
		"created_by": true,
		"dept_id":    true,
	}
	fd.IsForm = !excludedColumns[f.ColumnName]

	// 构建 gorm tag
	fd.GormTag = f.GoTag

	// 构建 binding tag
	fd.BindingTag = buildBindingTag(f)

	// 前端控件类型
	fd.FormType = mapToFormControl(f)

	return fd
}

// buildBindingTag 根据字段类型构建 binding tag。
func buildBindingTag(f FieldSchema) string {
	if !isFormExcluded(f.ColumnName) {
		return ""
	}

	parts := []string{}
	if !f.IsNullable && f.DefaultVal == "" {
		parts = append(parts, "required")
	}

	goType := strings.ToLower(f.GoType)
	switch {
	case goType == "string" && f.MaxLength > 0:
		parts = append(parts, fmt.Sprintf("max=%d", f.MaxLength))
	case strings.Contains(goType, "uint") || strings.Contains(goType, "int"):
		parts = append(parts, "gte=0")
	case f.EnumValues != "":
		parts = append(parts, "oneof="+f.EnumValues)
	}

	return strings.Join(parts, ",")
}

// isFormExcluded 判断字段是否排除出表单。
func isFormExcluded(col string) bool {
	excluded := map[string]bool{
		"id":         true,
		"created_at": true,
		"updated_at": true,
		"deleted_at": true,
	}
	return !excluded[col]
}

// mapToFormControl 映射到前端控件类型。
func mapToFormControl(f FieldSchema) string {
	colType := strings.ToLower(f.ColumnType)
	switch {
	case strings.Contains(colType, "text") || strings.Contains(colType, "longtext"):
		return "el-input type=textarea"
	case strings.Contains(colType, "datetime") || strings.Contains(colType, "timestamp"):
		return "el-date-picker"
	case strings.Contains(colType, "json"):
		return "el-input type=textarea"
	case strings.Contains(colType, "decimal"):
		return "el-input-number"
	case strings.Contains(colType, "tinyint") && f.EnumValues != "":
		return "el-select"
	case strings.Contains(colType, "int") || strings.Contains(colType, "bigint"):
		return "el-input-number"
	default:
		return "el-input"
	}
}

// toPascalCase 将蛇形名转为 PascalCase。
func toPascalCase(s string) string {
	parts := strings.Split(s, "_")
	for i, p := range parts {
		if len(p) > 0 {
			parts[i] = strings.ToUpper(p[:1]) + p[1:]
		}
	}
	return strings.Join(parts, "")
}

// toCamelCase 将蛇形名转为 camelCase。
func toCamelCase(s string) string {
	pascal := toPascalCase(s)
	if len(pascal) == 0 {
		return pascal
	}
	return strings.ToLower(pascal[:1]) + pascal[1:]
}
