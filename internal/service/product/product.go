package product

import (
	"context"

	"gorm.io/gorm"

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
}

// NewProductService 创建产品服务实例。
func NewProductService() *ProductService {
	return &ProductService{}
}

// List 分页查询产品列表（接入数据权限）。
func (s *ProductService) List(params *form.ProductList) interface{} {
	condition, args := s.buildListCondition(params)

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
		return resources.NewProductTransformer().ToCollection(params.Page, params.PerPage, 0, []*model.Product{})
	}

	return resources.NewProductTransformer().ToCollection(params.Page, params.PerPage, total, collection)
}

func (s *ProductService) buildListCondition(params *form.ProductList) (string, []any) {
	return query_builder.New().
		AddLike("name", params.Name).
		AddEq("status", params.Status).
		Build()
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
	filter := access.NewDataScopeFilter()
	return filter.Build(s.context(), input, "dept_id", "created_by")
}

// Create 新增产品（写入 created_by 和 dept_id）。
func (s *ProductService) Create(params *form.CreateProduct) error {
	deptID, err := s.resolveWritableDeptID(params.DeptId)
	if err != nil {
		return err
	}

	product := model.NewProduct()
	product.Name = params.Name
	product.Description = params.Description
	product.Price = params.Price
	product.Status = params.Status
	// 写入创建者
	product.CreatedBy = s.GetAdminUserId()
	product.DeptId = deptID

	if err := product.Create(); err != nil {
		return e.NewBusinessError(e.FAILURE)
	}
	return nil
}

// Update 更新产品（校验行级权限）。
func (s *ProductService) Update(params *form.UpdateProduct) error {
	product := model.NewProduct()
	// 查询产品时加上数据权限条件，确保只能更新有权限的数据
	condition, args := s.buildDetailCondition(params.Id)
	if err := product.GetDetail(condition, args...); err != nil || product.ID == 0 {
		return e.NewBusinessError(e.NotFound)
	}

	// 更新字段
	product.Name = params.Name
	product.Description = params.Description
	product.Price = params.Price
	product.Status = params.Status
	// 更新部门：前端传入 dept_id 时更新，否则保持原值
	if params.DeptId > 0 {
		deptID, err := s.resolveWritableDeptID(params.DeptId)
		if err != nil {
			return err
		}
		product.DeptId = deptID
	}

	if err := product.Save(); err != nil {
		return e.NewBusinessError(e.FAILURE)
	}
	return nil
}

// Delete 删除产品（校验行级权限）。
func (s *ProductService) Delete(id uint) error {
	product := model.NewProduct()
	// 查询产品时加上数据权限条件，确保只能删除有权限的数据
	condition, args := s.buildDetailCondition(id)
	if err := product.GetDetail(condition, args...); err != nil || product.ID == 0 {
		return e.NewBusinessError(e.NotFound)
	}

	if _, err := product.DeleteByID(id); err != nil {
		return e.NewBusinessError(e.FAILURE)
	}
	return nil
}

// Detail 获取产品详情（校验行级权限）。
func (s *ProductService) Detail(id uint) (any, error) {
	product := model.NewProduct()
	// 查询产品时加上数据权限条件，确保只能查看有权限的数据
	condition, args := s.buildDetailCondition(id)
	if err := s.loadProductDetail(product, condition, args...); err != nil || product.ID == 0 {
		return nil, e.NewBusinessError(e.NotFound)
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

func (s *ProductService) loadProductDetail(product *model.Product, condition string, args ...any) error {
	db, err := product.GetDB(product)
	if err != nil {
		return err
	}
	return db.Preload("Department").Where(condition, args...).First(product).Error
}

// getUserDeptId 获取当前用户的默认部门 ID（取第一个部门）。
func (s *ProductService) getUserDeptId() uint {
	uid := s.GetAdminUserId()
	if uid == 0 {
		return 0
	}
	deptIds, err := model.NewAdminUserDeptMap().DeptIdsByUid(uid)
	if err != nil || len(deptIds) == 0 {
		return 0
	}
	return deptIds[0]
}

func (s *ProductService) resolveWritableDeptID(requested uint) (uint, error) {
	if requested > 0 {
		if err := s.ensureDeptExists(requested); err != nil {
			return 0, err
		}
		if s.isSuperAdmin() || s.userHasDirectDept(requested) {
			return requested, nil
		}
		return 0, e.NewBusinessError(e.InvalidParameter, "无权使用该部门")
	}

	deptID := s.getUserDeptId()
	if deptID == 0 {
		return 0, e.NewBusinessError(e.DepartmentNotFound)
	}
	if err := s.ensureDeptExists(deptID); err != nil {
		return 0, err
	}
	return deptID, nil
}

func (s *ProductService) ensureDeptExists(deptID uint) error {
	exists, err := model.NewDepartment().Exists("id = ? AND deleted_at = 0", deptID)
	if err != nil {
		return err
	}
	if !exists {
		return e.NewBusinessError(e.DepartmentNotFound)
	}
	return nil
}

func (s *ProductService) userHasDirectDept(deptID uint) bool {
	uid := s.GetAdminUserId()
	if uid == 0 {
		return false
	}
	deptIDs, err := model.NewAdminUserDeptMap().DeptIdsByUid(uid)
	if err != nil {
		return false
	}
	for _, id := range deptIDs {
		if id == deptID {
			return true
		}
	}
	return false
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
