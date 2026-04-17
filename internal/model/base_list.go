package model

import (
	"fmt"
	"regexp"
	"strings"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/wannanbigpig/gin-layout/internal/pkg/utils"
)

// BaseModelInterface 定义一个接口约束，包含所需的方法。
type BaseModelInterface[T any] interface {
	Count(condition string, args ...any) (int64, error)
	Paginate(page, perPage int) func(*gorm.DB) *gorm.DB
	TableName() string
	*T
}

// ListOptionalParams 定义列表查询的可选参数。
type ListOptionalParams struct {
	SelectFields  []string
	Preload       map[string]func(db *gorm.DB) *gorm.DB
	AllPreLoad    bool
	OrderBy       string
	OrderAllowMap map[string]struct{}
	Joins         []string
	Distinct      string
	CountDistinct string
}

var orderByPattern = regexp.MustCompile(`(?i)^([a-z_][a-z0-9_]*)(?:\.([a-z_][a-z0-9_]*))?(?:\s+(asc|desc))?$`)
var selectFieldPattern = regexp.MustCompile(`(?i)^([a-z_][a-z0-9_]*)(?:\.([a-z_][a-z0-9_]*))?$`)

func normalizeOrderBy(orderBy string, allowed map[string]struct{}) (string, error) {
	orderBy = strings.TrimSpace(orderBy)
	if orderBy == "" {
		return "", nil
	}

	parts := strings.Split(orderBy, ",")
	res := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		matches := orderByPattern.FindStringSubmatch(part)
		if len(matches) == 0 {
			return "", fmt.Errorf("invalid order by clause: %s", part)
		}

		field := matches[1]
		column := field
		if matches[2] != "" {
			column = matches[2]
		}

		if len(allowed) > 0 {
			if _, ok := allowed[column]; !ok {
				return "", fmt.Errorf("order field not allowed: %s", column)
			}
		}

		direction := "ASC"
		if strings.EqualFold(matches[3], "desc") {
			direction = "DESC"
		}
		res = append(res, fmt.Sprintf("%s %s", field, direction))
	}

	if len(res) == 0 {
		return "", nil
	}
	return strings.Join(res, ", "), nil
}

func normalizeSelectFields(fields string) (string, error) {
	fields = strings.TrimSpace(fields)
	if fields == "" {
		return "", nil
	}

	parts := strings.Split(fields, ",")
	res := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		matches := selectFieldPattern.FindStringSubmatch(part)
		if len(matches) == 0 {
			return "", fmt.Errorf("invalid select field: %s", part)
		}
		if matches[2] != "" {
			res = append(res, fmt.Sprintf("%s.%s", matches[1], matches[2]))
			continue
		}
		res = append(res, matches[1])
	}
	if len(res) == 0 {
		return "", nil
	}
	return strings.Join(res, ", "), nil
}

func buildListQuery[T any, M BaseModelInterface[T]](model M, condition string, args []any, listParams ListOptionalParams) (*gorm.DB, error) {
	query, err := GetDB(model)
	if err != nil {
		return nil, err
	}

	if len(listParams.Joins) > 0 {
		for _, join := range listParams.Joins {
			query = query.Joins(join)
		}
	}

	if condition != "" {
		query = query.Where(condition, args...)
	}

	if listParams.Distinct != "" {
		distinctFields, err := normalizeSelectFields(listParams.Distinct)
		if err != nil {
			return nil, err
		}
		if distinctFields != "" {
			query = query.Distinct(distinctFields)
		}
	}

	if len(listParams.SelectFields) > 0 {
		query = query.Select(listParams.SelectFields)
	}

	if listParams.OrderBy != "" {
		safeOrderBy, err := normalizeOrderBy(listParams.OrderBy, listParams.OrderAllowMap)
		if err != nil {
			return nil, err
		}
		if safeOrderBy != "" {
			query = query.Order(safeOrderBy)
		} else {
			query = query.Order("id desc")
		}
	} else {
		query = query.Order("id desc")
	}

	if listParams.AllPreLoad {
		query = query.Preload(clause.Associations)
	} else if len(listParams.Preload) > 0 {
		for key, value := range listParams.Preload {
			if value == nil {
				query = query.Preload(key)
			} else {
				query = query.Preload(key, value)
			}
		}
	}

	return query, nil
}

func countListTotal[T any, M BaseModelInterface[T]](model M, baseQuery *gorm.DB, condition string, args []any, listParams ListOptionalParams) (int64, error) {
	if listParams.CountDistinct != "" {
		countDistinctFields, err := normalizeSelectFields(listParams.CountDistinct)
		if err != nil {
			return 0, err
		}
		if countDistinctFields == "" {
			return 0, fmt.Errorf("count distinct fields are required")
		}
		countQuery := baseQuery
		if condition != "" {
			countQuery = countQuery.Where(condition, args...)
		}
		var total int64
		err = countQuery.Model(model).
			Select(fmt.Sprintf("COUNT(DISTINCT %s)", countDistinctFields)).
			Scan(&total).Error
		return total, err
	}

	if len(listParams.Joins) > 0 {
		countQuery := baseQuery
		if condition != "" {
			countQuery = countQuery.Where(condition, args...)
		}
		var total int64
		err := countQuery.Model(model).Count(&total).Error
		return total, err
	}

	if condition != "" {
		return model.Count(condition, args...)
	}
	return model.Count("")
}

// ListPageE 按条件分页查询列表，并返回总数与结果集。
func ListPageE[T any, M BaseModelInterface[T]](model M, page, perPage int, condition string, args []any, optional ...ListOptionalParams) (int64, []*T, error) {
	if condition != "" {
		condition = utils.TrimPrefixAndSuffixAND(condition)
	}

	var listParams ListOptionalParams
	if len(optional) > 0 {
		listParams = optional[0]
	}

	baseQuery, err := GetDB(model)
	if err != nil {
		return 0, nil, err
	}
	if len(listParams.Joins) > 0 {
		for _, join := range listParams.Joins {
			baseQuery = baseQuery.Joins(join)
		}
	}

	total, err := countListTotal(model, baseQuery, condition, args, listParams)
	if err != nil || total == 0 {
		return total, nil, err
	}

	query, err := buildListQuery(model, condition, args, listParams)
	if err != nil {
		return total, nil, err
	}
	query = query.Scopes(model.Paginate(page, perPage))

	res := make([]*T, 0, perPage)
	err = query.Find(&res).Error
	if err != nil {
		return total, nil, err
	}
	return total, res, nil
}

// ListE 按条件查询列表，支持预加载、排序、字段选择等可选参数。
func ListE[T any, M BaseModelInterface[T]](model M, condition string, args []any, optional ...ListOptionalParams) ([]*T, error) {
	if condition != "" {
		condition = utils.TrimPrefixAndSuffixAND(condition)
	}

	var listParams ListOptionalParams
	if len(optional) > 0 {
		listParams = optional[0]
	}

	query, err := buildListQuery(model, condition, args, listParams)
	if err != nil {
		return nil, err
	}

	var res []*T
	err = query.Find(&res).Error
	if err != nil {
		return nil, err
	}
	return res, nil
}

// VerifyExistingIDs 返回输入 ID 中数据库实际存在的 ID 列表。
func VerifyExistingIDs[T any, M BaseModelInterface[T]](model M, ids []uint) ([]uint, error) {
	if len(ids) == 0 {
		return ids, nil
	}

	var existIds []uint
	db, err := GetDB(model)
	if err != nil {
		return nil, err
	}
	if err := db.Where("id IN (?)", ids).Pluck("id", &existIds).Error; err != nil {
		return nil, err
	}

	return existIds, nil
}

// ExtractColumnsByCondition 按条件提取指定列的值列表。
func ExtractColumnsByCondition[T any, M BaseModelInterface[T], R any](model M, column string, condition string, args ...any) ([]R, error) {
	var columns []R
	if condition == "" {
		return nil, fmt.Errorf("condition is required")
	}
	if column == "" {
		return nil, fmt.Errorf("column name is required")
	}

	db, err := GetDB(model)
	if err != nil {
		return nil, err
	}
	if err := db.Where(condition, args...).Pluck(column, &columns).Error; err != nil {
		return nil, err
	}

	return columns, nil
}
