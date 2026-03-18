package model

import (
	"errors"
	"fmt"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/plugin/soft_delete"

	"github.com/wannanbigpig/gin-layout/data"
	"github.com/wannanbigpig/gin-layout/internal/global"
	"github.com/wannanbigpig/gin-layout/internal/pkg/utils"
)

// BaseModel 提供模型通用字段与基础 CRUD 能力。
type BaseModel struct {
	dbInstance *gorm.DB
	owner      any
	ID         uint             `gorm:"column:id;type:int(11) unsigned AUTO_INCREMENT;not null;primarykey" json:"id"`
	CreatedAt  utils.FormatDate `gorm:"column:created_at;type:datetime;<-:create" json:"created_at"`
	UpdatedAt  utils.FormatDate `gorm:"column:updated_at;type:datetime" json:"updated_at"`
}

// ErrDBUninitialized 表示数据库连接尚未初始化。
var ErrDBUninitialized = errors.New("database connection is not initialized")

// ErrModelPtrNotImplemented 表示模型尚未完成 owner 绑定。
var ErrModelPtrNotImplemented = errors.New("model owner binding is not initialized")

type ownerBinder interface {
	bindOwner(any)
}

// SetDB 为当前模型绑定事务或指定数据库连接。
func (m *BaseModel) SetDB(tx *gorm.DB) *BaseModel {
	m.dbInstance = tx
	return m
}

func (m *BaseModel) bindOwner(owner any) {
	m.owner = owner
}

// BindModel 为嵌入 BaseModel 的模型绑定自身实例，供通用方法回写使用。
func BindModel[T any](m T) T {
	if binder, ok := any(m).(ownerBinder); ok {
		binder.bindOwner(m)
	}
	return m
}

func (m *BaseModel) self() (any, error) {
	if m.owner == nil {
		return nil, ErrModelPtrNotImplemented
	}
	return m.owner, nil
}

// GetDB 返回当前模型可用的数据库实例，传入 model 时会附带 Model 上下文。
func (m *BaseModel) GetDB(model ...any) (*gorm.DB, error) {
	if m.dbInstance != nil {
		if len(model) > 0 {
			return m.dbInstance.Model(model[0]), nil
		}
		return m.dbInstance, nil
	}
	return GetDB(model...)
}

// Paginate 返回 GORM 分页作用域，页码小于 1 时会自动修正为第 1 页。
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

// Count 按条件统计当前模型记录总数。
func (m *BaseModel) Count(condition string, args ...any) (count int64, err error) {
	self, err := m.self()
	if err != nil {
		return 0, err
	}
	query, err := m.GetDB(self)
	if err != nil {
		return 0, err
	}
	if condition != "" {
		query = query.Where(condition, args...)
	}
	err = query.Count(&count).Error
	if err != nil {
		return 0, err
	}
	return
}

// GetById 根据 ID 获取当前模型信息。
func (m *BaseModel) GetById(id uint) error {
	self, err := m.self()
	if err != nil {
		return err
	}
	db, err := m.GetDB()
	if err != nil {
		return err
	}
	return db.First(self, id).Error
}

// GetAllById 根据 ID 获取当前模型及全部关联表信息。
func (m *BaseModel) GetAllById(id uint) error {
	self, err := m.self()
	if err != nil {
		return err
	}
	db, err := m.GetDB()
	if err != nil {
		return err
	}
	return db.Preload(clause.Associations).First(self, id).Error
}

// GetDetail 按条件查询当前模型的单条详情记录。
//
// condition 使用 GORM 的条件表达式写法，val 需要与其中的占位符一一对应。
// 示例：
//
//	GetDetail("username = ? AND status = ?", "admin", 1)
func (m *BaseModel) GetDetail(condition string, val ...any) error {
	self, err := m.self()
	if err != nil {
		return err
	}
	db, err := m.GetDB()
	if err != nil {
		return err
	}
	return db.Where(condition, val...).First(self).Error
}

// ExistsById 判断指定 ID 的记录是否存在。
func (m *BaseModel) ExistsById(id uint) (bool, error) {
	if id == 0 {
		return false, nil
	}
	return m.Exists("id = ?", id)
}

// Exists 判断是否存在满足条件的记录。
func (m *BaseModel) Exists(condition string, args ...any) (bool, error) {
	self, err := m.self()
	if err != nil {
		return false, err
	}

	db, err := m.GetDB()
	if err != nil {
		return false, err
	}
	var count int64
	err = db.Model(self).Where(condition, args...).Limit(1).Count(&count).Error
	return count > 0, err
}

// UpdateById 根据 ID 更新当前模型记录。
func (m *BaseModel) UpdateById(id uint, data map[string]any) error {
	self, err := m.self()
	if err != nil {
		return err
	}
	db, err := m.GetDB()
	if err != nil {
		return err
	}
	return db.Model(self).Where("id = ?", id).Updates(data).Error
}

// DeleteByID 根据 ID 删除当前模型记录。
func (m *BaseModel) DeleteByID(id uint) (int64, error) {
	self, err := m.self()
	if err != nil {
		return 0, err
	}
	db, err := m.GetDB()
	if err != nil {
		return 0, err
	}
	result := db.Delete(self, id)
	return result.RowsAffected, result.Error
}

// DeleteWhere 按条件删除当前模型记录，空条件会被拒绝以防误删全表。
func (m *BaseModel) DeleteWhere(condition string, args ...any) error {
	if condition == "" {
		return fmt.Errorf("delete condition is empty, operation refused to prevent full table deletion")
	}
	self, err := m.self()
	if err != nil {
		return err
	}
	db, err := m.GetDB()
	if err != nil {
		return err
	}
	return db.Where(condition, args...).Delete(self).Error
}

// Create 使用字段映射创建一条当前模型记录。
func (m *BaseModel) Create(data map[string]any) error {
	self, err := m.self()
	if err != nil {
		return err
	}
	db, err := m.GetDB(self)
	if err != nil {
		return err
	}
	return db.Create(data).Error
}

// CreateBatch 使用字段映射批量创建当前模型记录。
func (m *BaseModel) CreateBatch(data []map[string]any) error {
	self, err := m.self()
	if err != nil {
		return err
	}
	db, err := m.GetDB(self)
	if err != nil {
		return err
	}
	return db.Create(data).Error
}

// Save 保存当前模型实例，存在主键时会执行更新。
func (m *BaseModel) Save() error {
	self, err := m.self()
	if err != nil {
		return err
	}
	db, err := m.GetDB()
	if err != nil {
		return err
	}
	return db.Save(self).Error
}

// ContainsDeleteBaseModel 在 BaseModel 基础上增加软删除字段。
type ContainsDeleteBaseModel struct {
	BaseModel
	DeletedAt soft_delete.DeletedAt `gorm:"column:deleted_at;type:int(11) unsigned;not null;default:0;index;" json:"-"`
}

// GetDB 返回全局数据库实例，传入 model 时会附带 Model 上下文。
func GetDB(model ...any) (*gorm.DB, error) {
	db := data.MysqlDB()
	if db == nil {
		return nil, ErrDBUninitialized
	}
	if len(model) > 0 && model[0] != nil {
		return db.Model(model[0]), nil
	}
	return db, nil
}

// BaseModelInterface 定义一个接口约束，包含所需的方法
type BaseModelInterface[T any] interface {
	Count(condition string, args ...any) (int64, error)
	Paginate(page, perPage int) func(*gorm.DB) *gorm.DB
	TableName() string
	*T
}

// ListOptionalParams 定义列表查询的可选参数。
//
// 该结构通常作为 ListPageE 或 ListE 的最后一个参数传入；
// 如果传入多个，仅会使用第一个。
type ListOptionalParams struct {
	// SelectFields 指定查询字段；为空时默认查询全部字段。
	SelectFields []string
	// Preload 指定需要预加载的关联关系。
	// key 为关联名，value 为可选的预加载回调；value 为 nil 时表示默认预加载。
	Preload map[string]func(db *gorm.DB) *gorm.DB
	// AllPreLoad 为 true 时会预加载全部关联，优先级高于 Preload。
	AllPreLoad bool
	// OrderBy 指定排序语句，例如："sort asc,id desc"；为空时默认按 id desc。
	OrderBy string
	// Joins 指定 JOIN 子句，例如：["INNER JOIN dept d ON d.id = admin_user.dept_id"]。
	Joins []string
	// Distinct 指定查询结果去重字段，常用于 JOIN 后避免列表数据重复，例如："admin_user.id"。
	Distinct string
	// CountDistinct 指定分页总数统计时使用的去重字段。
	// 当列表查询包含 JOIN 且主表可能重复时，应与 Distinct 配合使用，确保 total 正确。
	CountDistinct string
}

// buildListQuery 构建列表查询的公共方法
func buildListQuery[T any, M BaseModelInterface[T]](model M, condition string, args []any, listParams ListOptionalParams) (*gorm.DB, error) {
	// 构建基础查询
	query, err := GetDB(model)
	if err != nil {
		return nil, err
	}

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

	return query, nil
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
		return model.Count(condition, args...)
	}
	return model.Count("")
}

// ListPageE 按条件分页查询列表，并返回总数与结果集。
//
// 参数约定：
//   - condition 为 SQL 条件片段，可省略 where 关键字，例如："status = ? AND dept_id = ?"
//   - args 需要按顺序对应 condition 中的占位符
//   - optional 通常只传一个 ListOptionalParams，用于控制 JOIN、预加载、排序和字段选择
//
// 使用 JOIN 查询时，如果主表记录可能因关联表而重复，建议同时设置：
//   - Distinct：用于结果去重
//   - CountDistinct：用于 total 去重计数
//
// 返回值依次为总数、当前页数据、错误。
func ListPageE[T any, M BaseModelInterface[T]](model M, page, perPage int, condition string, args []any, optional ...ListOptionalParams) (int64, []*T, error) {
	if condition != "" {
		condition = utils.TrimPrefixAndSuffixAND(condition)
	}

	var listParams ListOptionalParams
	if len(optional) > 0 {
		listParams = optional[0]
	}

	// 构建基础查询（用于计数）
	baseQuery, err := GetDB(model)
	if err != nil {
		return 0, nil, err
	}
	if len(listParams.Joins) > 0 {
		for _, join := range listParams.Joins {
			baseQuery = baseQuery.Joins(join)
		}
	}

	// 计算总数
	total, err := countListTotal(model, baseQuery, condition, args, listParams)
	if err != nil || total == 0 {
		return total, nil, err
	}

	// 构建查询并应用分页
	query, err := buildListQuery(model, condition, args, listParams)
	if err != nil {
		return total, nil, err
	}
	query = query.Scopes(model.Paginate(page, perPage))

	// 执行查询
	res := make([]*T, 0, perPage)
	err = query.Find(&res).Error
	if err != nil {
		return total, nil, err
	}
	return total, res, nil
}

// ListE 按条件查询列表，支持预加载、排序、字段选择等可选参数。
//
// 该方法与 ListPageE 的查询约定一致，但不做分页。
// condition 与 args 的对应关系、optional 的使用方式与 ListPageE 相同。
func ListE[T any, M BaseModelInterface[T]](model M, condition string, args []any, optional ...ListOptionalParams) ([]*T, error) {
	if condition != "" {
		condition = utils.TrimPrefixAndSuffixAND(condition)
	}

	var listParams ListOptionalParams
	if len(optional) > 0 {
		listParams = optional[0]
	}

	// 构建查询
	query, err := buildListQuery(model, condition, args, listParams)
	if err != nil {
		return nil, err
	}

	// 执行查询
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
//
// column 应传数据库字段名，condition 与 args 的写法和 GetDetail 一致。
// 泛型参数 R 需要与目标列类型兼容，例如提取 id 可使用 []uint，提取 name 可使用 []string。
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

// HasChildren 判断指定父节点是否存在子节点。
func HasChildren[T any, M BaseModelInterface[T]](model M, pid uint) (bool, error) {
	count, err := model.Count("pid = ?", pid)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// UpdateChildrenNum 统计并更新父节点的 children_num 字段。
//
// pid 为 0 时直接跳过更新；tx 不为 nil 时会优先使用传入事务，
// 以便在创建、删除或移动子节点的事务中保持 children_num 一致。
func UpdateChildrenNum[T any, M BaseModelInterface[T]](model M, pid uint, tx *gorm.DB) error {
	if pid == 0 {
		return nil
	}

	// 获取查询对象
	getDB := func() (*gorm.DB, error) {
		if tx != nil {
			return tx.Model(model), nil
		}
		return GetDB(model)
	}

	// 统计子节点数量
	var count int64
	queryDB, err := getDB()
	if err != nil {
		return err
	}
	if err := queryDB.Where("pid = ?", pid).Count(&count).Error; err != nil {
		return err
	}

	// 更新父节点的 children_num（使用新的查询对象避免条件叠加）
	updateDB, err := getDB()
	if err != nil {
		return err
	}
	return updateDB.Where("id = ?", pid).Update("children_num", count).Error
}
