package product

import (
	"context"
	"errors"
	"strings"
	"unicode/utf8"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/wannanbigpig/gin-layout/internal/global"
	"github.com/wannanbigpig/gin-layout/internal/model"
	e "github.com/wannanbigpig/gin-layout/internal/pkg/errors"
	"github.com/wannanbigpig/gin-layout/internal/pkg/query_builder"
	"github.com/wannanbigpig/gin-layout/internal/resources"
	"github.com/wannanbigpig/gin-layout/internal/service"
	"github.com/wannanbigpig/gin-layout/internal/service/access"
	"github.com/wannanbigpig/gin-layout/internal/service/auth"
	"github.com/wannanbigpig/gin-layout/internal/validator/form"
)

// ProductService 产品服务（示例：演示数据权限接入）。
type ProductService struct {
	service.Base
	db *gorm.DB
}

type productDataScopeFilter interface {
	Build(ctx context.Context, input access.DataScopeInput, deptCol, ownerCol string) (string, []any)
}

var newProductDataScopeFilter = func() productDataScopeFilter {
	return access.NewDataScopeFilter()
}

// NewProductService 创建产品服务实例。
func NewProductService() *ProductService {
	return &ProductService{}
}

// List 分页查询产品列表（接入数据权限）。
func (s *ProductService) List(params *form.ProductList) (interface{}, error) {
	condition, args := query_builder.New().
		AddLike("name", params.Name).
		AddEq("status", params.Status).
		Build()

	// 接入数据权限过滤
	dataScopeCond, dataScopeArgs := s.buildDataScopeCondition()
	if dataScopeCond != "" {
		if condition != "" {
			condition = "(" + condition + ") AND " + dataScopeCond
		} else {
			condition = dataScopeCond
		}
		args = append(args, dataScopeArgs...)
	}

	productModel := model.NewProduct()
	db, err := s.dbOrDefault()
	if err != nil {
		return nil, err
	}
	productModel.SetDB(db)
	listOptions := model.ListOptionalParams{
		OrderBy: "id desc",
		Preload: map[string]func(db *gorm.DB) *gorm.DB{
			"Department": nil,
		},
	}
	total, collection, err := model.ListPageE(
		productModel,
		params.Page,
		params.PerPage,
		condition,
		args,
		listOptions,
	)
	if err != nil {
		return nil, err
	}

	return resources.NewProductTransformer().ToCollection(params.Page, params.PerPage, total, collection), nil
}

// buildDataScopeCondition 构建数据权限过滤条件。
// 展示如何在业务 service 中接入 DataScopeFilter。
func (s *ProductService) buildDataScopeCondition() (string, []any) {
	// 获取当前用户信息
	uid := s.GetAdminUserId()

	// 从上下文获取 AuthPrincipal 判断是否超管
	isSuperAdmin := false
	if ctx := s.GetCtx(); ctx != nil {
		if principal := auth.GetAuthPrincipal(ctx); principal != nil {
			isSuperAdmin = principal.IsSuperAdmin == global.Yes
		}
	}

	// 构建数据权限输入
	input := access.DataScopeInput{
		UID:          uid,
		IsSuperAdmin: isSuperAdmin,
	}

	// 使用 DataScopeFilter 构建过滤条件
	// deptCol: 业务表的部门列名
	// ownerCol: 业务表的创建者列名
	filter := newProductDataScopeFilter()
	return filter.Build(s.context(), input, "dept_id", "created_by")
}

// Create 新增产品（写入 created_by 和 dept_id）。
func (s *ProductService) Create(params *form.CreateProduct) error {
	_, err := s.CreateWithAuditDiff(params)
	return err
}

// Update 更新产品（校验行级权限）。
func (s *ProductService) Update(params *form.UpdateProduct) error {
	_, err := s.UpdateWithAuditDiff(params)
	return err
}

// Delete 删除产品（校验行级权限）。
func (s *ProductService) Delete(id uint) error {
	_, err := s.DeleteWithAuditDiff(id)
	return err
}

// Detail 获取产品详情（校验行级权限）。
func (s *ProductService) Detail(id uint) (any, error) {
	product := model.NewProduct()
	// 查询产品时加上数据权限条件，确保只能查看有权限的数据
	condition, args := s.buildDetailCondition(id)
	db, err := s.dbOrDefault()
	if err != nil {
		return nil, err
	}
	if err := db.Preload("Department").Where(condition, args...).First(product).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, e.NewBusinessError(e.NotFound)
		}
		return nil, err
	}

	return resources.NewProductTransformer().ToStruct(product), nil
}

// buildDetailCondition 构建详情查询条件（包含数据权限）。
func (s *ProductService) buildDetailCondition(id uint) (string, []any) {
	// 基础条件：ID
	condition := "id = ?"
	args := []any{id}

	// 接入数据权限过滤
	dataScopeCond, dataScopeArgs := s.buildDataScopeCondition()
	if dataScopeCond != "" {
		condition = "(" + condition + ") AND " + dataScopeCond
		args = append(args, dataScopeArgs...)
	}

	return condition, args
}

// getUserDeptID 获取当前用户的默认部门 ID（取第一个部门）。
func (s *ProductService) getUserDeptID(db *gorm.DB) (uint, error) {
	uid := s.GetAdminUserId()
	if uid == 0 {
		return 0, nil
	}
	var deptIDs []uint
	if err := db.Model(&model.AdminUserDeptMap{}).
		Where("uid = ?", uid).
		Order("id ASC").
		Limit(1).
		Pluck("dept_id", &deptIDs).Error; err != nil {
		return 0, err
	}
	if len(deptIDs) == 0 {
		return 0, nil
	}
	return deptIDs[0], nil
}

func (s *ProductService) resolveWritableDeptID(db *gorm.DB, requested uint) (uint, error) {
	if requested > 0 {
		if err := s.ensureDeptExists(db, requested); err != nil {
			return 0, err
		}
		if s.isSuperAdmin() {
			return requested, nil
		}
		hasDept, err := s.userHasDirectDept(db, requested)
		if err != nil {
			return 0, err
		}
		if hasDept {
			return requested, nil
		}
		return 0, e.NewBusinessError(e.InvalidParameter, "无权使用该部门")
	}

	deptID, err := s.getUserDeptID(db)
	if err != nil {
		return 0, err
	}
	if deptID == 0 {
		return 0, e.NewBusinessError(e.DepartmentNotFound)
	}
	if err := s.ensureDeptExists(db, deptID); err != nil {
		return 0, err
	}
	return deptID, nil
}

func (s *ProductService) ensureDeptExists(db *gorm.DB, deptID uint) error {
	var department model.Department
	err := db.Select("id").Where("id = ?", deptID).Take(&department).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return e.NewBusinessError(e.DepartmentNotFound)
	}
	return err
}

func (s *ProductService) userHasDirectDept(db *gorm.DB, deptID uint) (bool, error) {
	uid := s.GetAdminUserId()
	if uid == 0 {
		return false, nil
	}
	var count int64
	err := db.Model(&model.AdminUserDeptMap{}).
		Where("uid = ? AND dept_id = ?", uid, deptID).
		Limit(1).
		Count(&count).Error
	return count > 0, err
}

func (s *ProductService) isSuperAdmin() bool {
	if ctx := s.GetCtx(); ctx != nil {
		if principal := auth.GetAuthPrincipal(ctx); principal != nil {
			return principal.IsSuperAdmin == global.Yes
		}
	}
	return false
}

func (s *ProductService) context() context.Context {
	if ctx := s.GetCtx(); ctx != nil && ctx.Request != nil {
		return ctx.Request.Context()
	}
	return context.Background()
}

func (s *ProductService) dbOrDefault() (*gorm.DB, error) {
	if s.db != nil {
		return s.db.WithContext(s.context()), nil
	}
	db, err := model.GetDB()
	if err != nil {
		return nil, err
	}
	return db.WithContext(s.context()), nil
}

func normalizeProductName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", e.NewBusinessError(e.InvalidParameter, "产品名称不能为空")
	}
	if utf8.RuneCountInString(name) > 200 {
		return "", e.NewBusinessError(e.InvalidParameter, "产品名称不能超过200个字符")
	}
	return name, nil
}

func findProductForUpdate(tx *gorm.DB, condition string, args ...any) (*model.Product, error) {
	product := model.NewProduct()
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where(condition, args...).First(product).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, e.NewBusinessError(e.NotFound)
	}
	if err != nil {
		return nil, err
	}
	return product, nil
}
