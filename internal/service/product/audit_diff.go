package product

import (
	"fmt"

	"gorm.io/gorm"

	"github.com/wannanbigpig/gin-layout/internal/model"
	"github.com/wannanbigpig/gin-layout/internal/pkg/auditdiff"
	e "github.com/wannanbigpig/gin-layout/internal/pkg/errors"
	"github.com/wannanbigpig/gin-layout/internal/service"
	"github.com/wannanbigpig/gin-layout/internal/service/access"
	"github.com/wannanbigpig/gin-layout/internal/service/todo"
	"github.com/wannanbigpig/gin-layout/internal/validator/form"
)

const productActivationTodoSource = "product_activation"

var productDiffRules = []auditdiff.FieldRule{
	{Field: "id", Label: "产品ID"},
	{Field: "name", Label: "产品名称"},
	{Field: "description", Label: "产品描述"},
	{Field: "price", Label: "价格（分）"},
	{
		Field: "status",
		Label: "状态",
		ValueLabels: map[string]string{
			"0": "禁用",
			"1": "启用",
		},
	},
	{Field: "dept_id", Label: "归属部门ID"},
	{Field: "created_by", Label: "创建者ID"},
}

// CreateWithAuditDiff 创建产品并返回精确 change_diff。
func (s *ProductService) CreateWithAuditDiff(params *form.CreateProduct) (string, error) {
	if params == nil {
		return "", e.NewBusinessError(e.InvalidParameter)
	}
	name, err := normalizeProductName(params.Name)
	if err != nil {
		return "", err
	}
	db, err := s.dbOrDefault()
	if err != nil {
		return "", err
	}

	var changeDiff string
	err = access.RunInTransaction(db, func(tx *gorm.DB) error {
		deptID, err := s.resolveWritableDeptID(tx, params.DeptId)
		if err != nil {
			return err
		}
		product := model.NewProduct()
		product.Name = name
		product.Description = params.Description
		product.Price = params.Price
		product.Status = params.Status
		product.DeptId = deptID
		product.CreatedBy = s.GetAdminUserId()
		if err := service.CreateWithExplicitFields(tx, product, map[string]any{"status": params.Status}); err != nil {
			return err
		}
		if product.Status == 0 {
			if err := s.upsertActivationTodo(tx, product, false); err != nil {
				return err
			}
		}
		changeDiff = buildProductDiff(nil, snapshotProduct(product))
		return nil
	})
	return changeDiff, err
}

// UpdateWithAuditDiff 原子更新有数据权限的产品并返回精确 change_diff。
func (s *ProductService) UpdateWithAuditDiff(params *form.UpdateProduct) (string, error) {
	if params == nil {
		return "", e.NewBusinessError(e.InvalidParameter)
	}
	name, err := normalizeProductName(params.Name)
	if err != nil {
		return "", err
	}
	db, err := s.dbOrDefault()
	if err != nil {
		return "", err
	}
	condition, args := s.buildDetailCondition(params.Id)

	var changeDiff string
	err = access.RunInTransaction(db, func(tx *gorm.DB) error {
		product, err := findProductForUpdate(tx, condition, args...)
		if err != nil {
			return err
		}
		before := snapshotProduct(product)
		updates := map[string]any{
			"name":        name,
			"description": params.Description,
			"price":       params.Price,
			"status":      params.Status,
		}
		product.Name = name
		product.Description = params.Description
		product.Price = params.Price
		product.Status = params.Status
		if params.DeptId > 0 {
			deptID, err := s.resolveWritableDeptID(tx, params.DeptId)
			if err != nil {
				return err
			}
			updates["dept_id"] = deptID
			product.DeptId = deptID
		}

		result := tx.Model(&model.Product{}).Where(condition, args...).Updates(updates)
		if result.Error != nil {
			return result.Error
		}
		// 行已在同一事务中按相同权限条件锁定；SQL 再次携带条件，避免校验与写入脱节。
		if result.RowsAffected == 0 {
			var count int64
			if err := tx.Model(&model.Product{}).Where(condition, args...).Count(&count).Error; err != nil {
				return err
			}
			if count == 0 {
				return e.NewBusinessError(e.NotFound)
			}
		}
		if product.Status == 0 {
			if err := s.upsertActivationTodo(tx, product, before["status"] == uint8(1)); err != nil {
				return err
			}
		} else if before["status"] == uint8(0) {
			if err := s.closeActivationTodo(tx, product, model.TodoStatusCompleted); err != nil {
				return err
			}
		}
		changeDiff = buildProductDiff(before, snapshotProduct(product))
		return nil
	})
	return changeDiff, err
}

// DeleteWithAuditDiff 原子删除有数据权限的产品并返回精确 change_diff。
func (s *ProductService) DeleteWithAuditDiff(id uint) (string, error) {
	db, err := s.dbOrDefault()
	if err != nil {
		return "", err
	}
	condition, args := s.buildDetailCondition(id)

	var changeDiff string
	err = access.RunInTransaction(db, func(tx *gorm.DB) error {
		product, err := findProductForUpdate(tx, condition, args...)
		if err != nil {
			return err
		}
		before := snapshotProduct(product)
		result := tx.Where(condition, args...).Delete(&model.Product{})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return e.NewBusinessError(e.NotFound)
		}
		if product.Status == 0 {
			if err := s.closeActivationTodo(tx, product, model.TodoStatusCanceled); err != nil {
				return err
			}
		}
		changeDiff = buildProductDiff(before, nil)
		return nil
	})
	return changeDiff, err
}

func (s *ProductService) upsertActivationTodo(tx *gorm.DB, product *model.Product, startNewCycle bool) error {
	_, err := todo.NewServiceWithDB(tx).UpsertBusinessTodo(s.context(), todo.BusinessTodoInput{
		AssigneeID:    product.CreatedBy,
		CreatorID:     s.GetAdminUserId(),
		SourceType:    productActivationTodoSource,
		SourceKey:     productActivationTodoKey(product.ID),
		Title:         "启用产品：" + product.Name,
		Description:   "产品当前处于禁用状态，请检查产品信息后启用。",
		Priority:      model.TodoPriorityNormal,
		ActionURL:     "/business/product",
		StartNewCycle: startNewCycle,
	})
	return err
}

func (s *ProductService) closeActivationTodo(tx *gorm.DB, product *model.Product, status string) error {
	_, err := todo.NewServiceWithDB(tx).CloseBusinessTodo(s.context(), todo.BusinessTodoCloseInput{
		AssigneeID: product.CreatedBy,
		SourceType: productActivationTodoSource,
		SourceKey:  productActivationTodoKey(product.ID),
		Status:     status,
	})
	return err
}

func productActivationTodoKey(productID uint) string {
	return fmt.Sprintf("product:%d", productID)
}

func snapshotProduct(product *model.Product) map[string]any {
	if product == nil || product.ID == 0 {
		return nil
	}
	return map[string]any{
		"id":          product.ID,
		"name":        product.Name,
		"description": product.Description,
		"price":       product.Price,
		"status":      product.Status,
		"dept_id":     product.DeptId,
		"created_by":  product.CreatedBy,
	}
}

func buildProductDiff(before, after map[string]any) string {
	return auditdiff.Marshal(auditdiff.BuildFieldDiff(before, after, productDiffRules))
}
