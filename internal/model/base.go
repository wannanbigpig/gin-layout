package model

import (
	"fmt"

	"go.uber.org/zap"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/schema"
	"gorm.io/plugin/soft_delete"

	"github.com/wannanbigpig/gin-layout/data"
	"github.com/wannanbigpig/gin-layout/internal/global"
	log "github.com/wannanbigpig/gin-layout/internal/pkg/logger"
	"github.com/wannanbigpig/gin-layout/internal/pkg/utils"
)

type BaseModel struct {
	dbInstance *gorm.DB
	ID         uint             `gorm:"column:id;type:int(11) unsigned AUTO_INCREMENT;not null;primarykey" json:"id"`
	CreatedAt  utils.FormatDate `gorm:"column:created_at;type:datetime;<-:create" json:"created_at"`
	UpdatedAt  utils.FormatDate `gorm:"column:updated_at;type:datetime" json:"updated_at"`
}

func (m *BaseModel) SetDB(tx *gorm.DB) *BaseModel {
	m.dbInstance = tx
	return m
}

func (m *BaseModel) DB(model ...any) *gorm.DB {
	if m.dbInstance != nil {
		if len(model) > 0 {
			return m.dbInstance.Model(model[0])
		}
		return m.dbInstance
	}
	return DB(model...)
}

func (m *BaseModel) Paginate(page, pageSize int) func(db *gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		// 确保页码从 1 开始
		if page < 1 {
			page = 1
		}

		// 如果 pageSize 大于 0，则使用 pageSize，否则使用默认的 global.PerPage
		limit := global.PerPage
		if pageSize > 0 {
			limit = pageSize
		}

		// 计算 offset，确保从正确的位置开始分页
		offset := (page - 1) * limit

		// 设置 Offset 和 Limit
		return db.Offset(offset).Limit(limit)
	}
}

func (m *BaseModel) Count(model any, condition string, args []any) (count int64, err error) {
	query := m.DB(model)
	if condition != "" {
		query = query.Where(condition, args...)
	}
	err = query.Count(&count).Error
	if err != nil {
		return 0, err
	}
	return
}

// GetById  根据id获取信息
func (m *BaseModel) GetById(model any, id uint) error {
	return m.DB().First(model, id).Error
}

// GetAllById  根据id获取信息,同时获取全部关联表信息
func (m *BaseModel) GetAllById(model any, id uint) error {
	return m.DB().Preload(clause.Associations).First(model, id).Error
}

func (m *BaseModel) GetDetail(model any, condition string, val ...any) error {
	return m.DB().Where(condition, val...).First(model).Error
}

// ExistsById checks if a record exists with the given ID
func (m *BaseModel) ExistsById(model schema.Tabler, id uint) bool {
	if id == 0 {
		return false
	}
	return m.Exists(model, "id = ?", id)
}

// Exists checks if any record exists matching the given conditions
// Returns false if error occurs or no record found
func (m *BaseModel) Exists(model schema.Tabler, condition string, args ...any) bool {
	if model == nil {
		return false
	}

	var exists bool
	err := m.DB().Model(model).
		Select("1").
		Where(condition, args...).
		Limit(1).
		Scan(&exists).Error

	return err == nil && exists
}

// Update 更新
func (m *BaseModel) Update(model any, id uint, data map[string]any) error {
	return m.DB().Model(model).Where("id = ?", id).Updates(data).Error
}

// Delete 根据ID删除
func (m *BaseModel) Delete(model any, conds ...any) (int64, error) {
	result := m.DB().Delete(model, conds...)
	return result.RowsAffected, result.Error
}

// DeleteWithCondition 根据条件删除
func (m *BaseModel) DeleteWithCondition(model any, condition string, args ...any) error {
	if condition == "" {
		return fmt.Errorf("delete condition is empty, operation refused to prevent full table deletion")
	}
	return m.DB().Where(condition, args...).Delete(model).Error
}

// Create 创建
func (m *BaseModel) Create(model any, data map[string]any) error {
	return m.DB(model).Create(data).Error
}

// CreateBatch 批量创建
func (m *BaseModel) CreateBatch(model any, data []map[string]any) error {
	return m.DB(model).Create(data).Error
}

// Save 保存
func (m *BaseModel) Save(data any) error {
	return m.DB().Save(data).Error
}

type ContainsDeleteBaseModel struct {
	BaseModel
	DeletedAt soft_delete.DeletedAt `gorm:"column:deleted_at;type:int(11) unsigned;not null;default:0;index;" json:"-"`
}

func DB(model ...any) *gorm.DB {
	db := data.MysqlDB()
	if db == nil {
		// 如果数据库连接为 nil，记录错误并返回 nil
		// 调用方需要检查返回值
		log.Logger.Error("数据库连接未初始化")
		return nil
	}
	if len(model) > 0 && model[0] != nil {
		return db.Model(model[0])
	}
	return db
}

// BaseModelInterface 定义一个接口约束，包含所需的方法
type BaseModelInterface[T any] interface {
	Count(model any, condition string, args []any) (int64, error)
	DB(model ...any) *gorm.DB
	Paginate(page, perPage int) func(*gorm.DB) *gorm.DB
	TableName() string
	*T
}

// ListOptionalParams 定义可选参数结构体
type ListOptionalParams struct {
	SelectFields  []string
	Preload       map[string]func(db *gorm.DB) *gorm.DB
	AllPreLoad    bool
	OrderBy       string
	Joins         []string // JOIN 子句，例如：["INNER JOIN table ON condition"]
	Distinct      string   // DISTINCT 字段，用于 JOIN 查询去重，例如："a_admin_user.id"
	CountDistinct string   // COUNT DISTINCT 字段，用于 JOIN 查询计数，例如："a_admin_user.id"
}

// buildListQuery 构建列表查询的公共方法
func buildListQuery[T any, M BaseModelInterface[T]](model M, condition string, args []any, listParams ListOptionalParams) *gorm.DB {
	// 构建基础查询
	query := model.DB(model)

	// 应用 JOIN
	if len(listParams.Joins) > 0 {
		for _, join := range listParams.Joins {
			query = query.Joins(join)
		}
	}

	// 应用条件
	if condition != "" {
		query = query.Where(condition, args...)
	}

	// 应用 DISTINCT（用于 JOIN 查询去重）
	if listParams.Distinct != "" {
		query = query.Distinct(listParams.Distinct)
	}

	// 应用 SelectFields
	if len(listParams.SelectFields) > 0 {
		query = query.Select(listParams.SelectFields)
	}

	// 应用 OrderBy
	if listParams.OrderBy != "" {
		query = query.Order(listParams.OrderBy)
	} else {
		query = query.Order("id desc")
	}

	// 应用 Preload
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

	return query
}

// countListTotal 计算列表总数的公共方法
func countListTotal[T any, M BaseModelInterface[T]](model M, baseQuery *gorm.DB, condition string, args []any, listParams ListOptionalParams) (int64, error) {
	if listParams.CountDistinct != "" {
		// 使用 DISTINCT COUNT 方式计数（适用于 JOIN 查询）
		countQuery := baseQuery
		if condition != "" {
			countQuery = countQuery.Where(condition, args...)
		}
		var total int64
		err := countQuery.Model(model).
			Select(fmt.Sprintf("COUNT(DISTINCT %s)", listParams.CountDistinct)).
			Scan(&total).Error
		return total, err
	}
	// 如果有 JOIN，必须在 baseQuery 上计数，不能使用 model.Count
	if len(listParams.Joins) > 0 {
		countQuery := baseQuery
		if condition != "" {
			countQuery = countQuery.Where(condition, args...)
		}
		var total int64
		err := countQuery.Model(model).Count(&total).Error
		return total, err
	}
	// 普通计数方式（无 JOIN）
	if condition != "" {
		return model.Count(model, condition, args)
	}
	return model.Count(model, "", nil)
}

// ListPage 获取分页列表
func ListPage[T any, M BaseModelInterface[T]](model M, page, perPage int, condition string, args []any, optional ...ListOptionalParams) (int64, []*T) {
	if condition != "" {
		condition = utils.TrimPrefixAndSuffixAND(condition)
	}

	var listParams ListOptionalParams
	if len(optional) > 0 {
		listParams = optional[0]
	}

	// 构建基础查询（用于计数）
	baseQuery := model.DB(model)
	if len(listParams.Joins) > 0 {
		for _, join := range listParams.Joins {
			baseQuery = baseQuery.Joins(join)
		}
	}

	// 计算总数
	total, err := countListTotal(model, baseQuery, condition, args, listParams)
	if err != nil || total == 0 {
		return total, nil
	}

	// 构建查询并应用分页
	query := buildListQuery(model, condition, args, listParams)
	query = query.Scopes(model.Paginate(page, perPage))

	// 执行查询
	res := make([]*T, 0, perPage)
	err = query.Find(&res).Error
	if err != nil {
		log.Logger.Error("An error occurred when querying the paginated list", zap.Error(err))
		return total, nil
	}
	return total, res
}

// List 获取列表
func List[T any, M BaseModelInterface[T]](model M, condition string, args []any, optional ...ListOptionalParams) []*T {
	if condition != "" {
		condition = utils.TrimPrefixAndSuffixAND(condition)
	}

	var listParams ListOptionalParams
	if len(optional) > 0 {
		listParams = optional[0]
	}

	// 构建查询
	query := buildListQuery(model, condition, args, listParams)

	// 执行查询
	var res []*T
	err := query.Find(&res).Error
	if err != nil {
		log.Logger.Error("An error occurred when querying the list", zap.Error(err))
		return nil
	}
	return res
}

// VerifyExistingIDs 验证ID是否存在
func VerifyExistingIDs[T any, M BaseModelInterface[T]](model M, ids []uint) ([]uint, error) {
	if len(ids) == 0 {
		return ids, nil
	}

	var existIds []uint
	if err := model.DB(model).Where("id IN (?)", ids).Pluck("id", &existIds).Error; err != nil {
		return nil, err
	}

	return existIds, nil
}

// ExtractColumnsByCondition 获取符合条件的列
func ExtractColumnsByCondition[T any, M BaseModelInterface[T], R any](model M, column string, condition string, args ...any) ([]R, error) {
	var columns []R
	if condition == "" {
		return nil, fmt.Errorf("condition is required")
	}

	if column == "" {
		return nil, fmt.Errorf("column name is required")
	}
	if err := model.DB(model).Where(condition, args...).Pluck(column, &columns).Error; err != nil {
		return nil, err
	}

	return columns, nil
}

// Save 保存
func Save[T any, M BaseModelInterface[T]](model M) error {
	return model.DB().Save(model).Error
}

// HasChildren 判断是否有子菜单
func HasChildren[T any, M BaseModelInterface[T]](model M, pid uint) (bool, error) {
	count, err := model.Count(model, "pid =?", []any{pid})
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// UpdateChildrenNum 更新子集数量
func UpdateChildrenNum[T any, M BaseModelInterface[T]](model M, pid uint, tx *gorm.DB) error {
	if pid == 0 {
		return nil
	}

	// 获取查询对象
	getDB := func() *gorm.DB {
		if tx != nil {
			return tx.Model(model)
		}
		return model.DB(model)
	}

	// 统计子节点数量
	var count int64
	if err := getDB().Where("pid = ?", pid).Count(&count).Error; err != nil {
		return err
	}

	// 更新父节点的 children_num（使用新的查询对象避免条件叠加）
	return getDB().Where("id = ?", pid).Update("children_num", count).Error
}
