package gen

import (
	"fmt"
	"strings"

	"gorm.io/gorm"
)

// ReadTableSchemaFromDB 从 MySQL INFORMATION_SCHEMA 读取表结构。
func ReadTableSchemaFromDB(db *gorm.DB, tableName string) (*TableSchema, error) {
	var columns []struct {
		ColumnName    string `gorm:"column:COLUMN_NAME"`
		ColumnType    string `gorm:"column:COLUMN_TYPE"`
		ColumnComment string `gorm:"column:COLUMN_COMMENT"`
		IsNullable    string `gorm:"column:IS_NULLABLE"`
		ColumnKey     string `gorm:"column:COLUMN_KEY"`
		ColumnDefault *string `gorm:"column:COLUMN_DEFAULT"`
		Extra         string `gorm:"column:EXTRA"`
	}

	// 查询列信息
	err := db.Raw(`
		SELECT COLUMN_NAME, COLUMN_TYPE, COLUMN_COMMENT, IS_NULLABLE, 
		       COLUMN_KEY, COLUMN_DEFAULT, EXTRA
		FROM INFORMATION_SCHEMA.COLUMNS 
		WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = ?
		ORDER BY ORDINAL_POSITION
	`, tableName).Scan(&columns).Error
	if err != nil {
		return nil, fmt.Errorf("query columns: %w", err)
	}

	if len(columns) == 0 {
		return nil, fmt.Errorf("table %s not found or has no columns", tableName)
	}

	// 查询表注释
	var tableComment string
	err = db.Raw(`
		SELECT TABLE_COMMENT 
		FROM INFORMATION_SCHEMA.TABLES 
		WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = ?
	`, tableName).Scan(&tableComment).Error
	if err != nil {
		return nil, fmt.Errorf("query table comment: %w", err)
	}

	// 查询索引信息
	indexColumns := make(map[string]bool)
	var indexes []struct {
		ColumnName string `gorm:"column:COLUMN_NAME"`
	}
	err = db.Raw(`
		SELECT DISTINCT COLUMN_NAME
		FROM INFORMATION_SCHEMA.STATISTICS
		WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = ?
		AND INDEX_NAME != 'PRIMARY'
	`, tableName).Scan(&indexes).Error
	if err != nil {
		return nil, fmt.Errorf("query indexes: %w", err)
	}
	for _, idx := range indexes {
		indexColumns[idx.ColumnName] = true
	}

	// 构建 schema
	schema := &TableSchema{
		TableName: tableName,
		Module:    tableName,
		Comment:   tableComment,
	}

	for _, col := range columns {
		// 跳过系统字段
		if col.ColumnName == "id" || col.ColumnName == "created_at" ||
			col.ColumnName == "updated_at" || col.ColumnName == "deleted_at" {
			continue
		}

		field := FieldSchema{
			ColumnName: col.ColumnName,
			Name:       toPascalCase(col.ColumnName),
			ColumnType: col.ColumnType,
			Comment:    col.ColumnComment,
			IsNullable: col.IsNullable == "YES",
			IsPrimary:  col.ColumnKey == "PRI",
			MaxLength:  ExtractMaxLength(col.ColumnType),
			IsIndex:    indexColumns[col.ColumnName],
		}

		if col.ColumnDefault != nil {
			field.DefaultVal = *col.ColumnDefault
		}

		// 提取枚举值
		field.EnumValues = extractEnumValues(col.ColumnType)

		// 映射 Go 类型
		field.GoType = MapMySQLToGo(col.ColumnType, field.EnumValues)

		// 构建 gorm tag
		field.GoTag = buildGormTag(col.ColumnName, col.ColumnType, col.ColumnComment, col.IsNullable == "YES", field.DefaultVal)

		schema.Fields = append(schema.Fields, field)
	}

	return schema, nil
}

// extractEnumValues 从列类型中提取枚举值。
func extractEnumValues(colType string) string {
	ct := strings.ToLower(colType)
	if !strings.Contains(ct, "tinyint") {
		return ""
	}

	// 常见的 0/1 状态字段
	if strings.Contains(ct, "tinyint") {
		return "0 1"
	}
	return ""
}

// buildGormTag 构建 gorm tag。
func buildGormTag(colName, colType, comment string, isNullable bool, defaultVal string) string {
	parts := []string{
		fmt.Sprintf("column:%s", colName),
		fmt.Sprintf("type:%s", colType),
	}

	if !isNullable {
		parts = append(parts, "not null")
	}

	if defaultVal != "" {
		parts = append(parts, fmt.Sprintf("default:'%s'", defaultVal))
	}

	if comment != "" {
		parts = append(parts, fmt.Sprintf("comment:%s", comment))
	}

	return strings.Join(parts, ";")
}
