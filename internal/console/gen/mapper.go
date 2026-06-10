package gen

import (
	"regexp"
	"strconv"
	"strings"
)

var (
	varcharPattern = regexp.MustCompile(`(?i)varchar\((\d+)\)`)
	charPattern    = regexp.MustCompile(`(?i)char\((\d+)\)`)
)

// MapMySQLToGo 将 MySQL 列类型映射为 Go 类型。
func MapMySQLToGo(colType string, enumValues string) string {
	ct := strings.ToLower(strings.TrimSpace(colType))

	// 去掉 unsigned 后缀用于类型判断
	base := strings.ReplaceAll(ct, " unsigned", "")

	switch {
	case strings.Contains(ct, "json"):
		return "datatypes.JSON"
	case strings.Contains(ct, "datetime") || strings.Contains(ct, "timestamp"):
		return "utils.FormatDate"
	case strings.Contains(ct, "text") || strings.Contains(ct, "longtext") || strings.Contains(ct, "mediumtext"):
		return "string"
	case strings.Contains(base, "bigint"):
		if strings.Contains(ct, "unsigned") {
			return "uint64"
		}
		return "int64"
	case strings.Contains(base, "int") && !strings.Contains(base, "tinyint") && !strings.Contains(base, "smallint") && !strings.Contains(base, "mediumint"):
		if strings.Contains(ct, "unsigned") {
			return "uint"
		}
		return "int"
	case strings.Contains(base, "tinyint"):
		if enumValues != "" {
			return "uint8"
		}
		return "uint8"
	case strings.Contains(base, "smallint"):
		if strings.Contains(ct, "unsigned") {
			return "uint16"
		}
		return "int16"
	case strings.Contains(base, "mediumint"):
		if strings.Contains(ct, "unsigned") {
			return "uint"
		}
		return "int"
	case strings.Contains(ct, "decimal") || strings.Contains(ct, "float") || strings.Contains(ct, "double"):
		return "string"
	case strings.Contains(ct, "varchar") || strings.Contains(ct, "char"):
		return "string"
	default:
		return "string"
	}
}

// ExtractMaxLength 从列类型中提取最大长度。
func ExtractMaxLength(colType string) int {
	ct := strings.ToLower(colType)

	if matches := varcharPattern.FindStringSubmatch(ct); len(matches) > 1 {
		if n, err := strconv.Atoi(matches[1]); err == nil {
			return n
		}
	}
	if matches := charPattern.FindStringSubmatch(ct); len(matches) > 1 {
		if n, err := strconv.Atoi(matches[1]); err == nil {
			return n
		}
	}
	return 0
}

// IsSearchableColumn 判断列是否可作为搜索条件。
func IsSearchableColumn(colName string) bool {
	searchable := map[string]bool{
		"name":     true,
		"title":    true,
		"code":     true,
		"status":   true,
		"keyword":  true,
		"username": true,
		"nickname": true,
	}
	return searchable[colName]
}

// IsFormExcludedColumn 判断列是否排除出表单生成。
func IsFormExcludedColumn(colName string) bool {
	excluded := map[string]bool{
		"id":         true,
		"created_at": true,
		"updated_at": true,
		"deleted_at": true,
		"created_by": true,
		"dept_id":    true,
	}
	return excluded[colName]
}
