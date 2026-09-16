package product

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/wannanbigpig/gin-layout/internal/global"
	"github.com/wannanbigpig/gin-layout/internal/model"
	"github.com/wannanbigpig/gin-layout/internal/pkg/auditdiff"
	e "github.com/wannanbigpig/gin-layout/internal/pkg/errors"
	"github.com/wannanbigpig/gin-layout/internal/service/access"
	"github.com/wannanbigpig/gin-layout/internal/service/auth"
	"github.com/wannanbigpig/gin-layout/internal/validator/form"
)

func TestUpdateWithAuditDiffUsesExplicitFieldsAndTrimsName(t *testing.T) {
	db, service := newProductTestService(t)
	seedProduct(t, db, model.Product{
		ContainsDeleteBaseModel: model.ContainsDeleteBaseModel{BaseModel: model.BaseModel{ID: 1}},
		Name:                    "旧产品",
		Description:             "原描述",
		Price:                   100,
		Status:                  1,
		DeptId:                  8,
		CreatedBy:               99,
	})

	diffJSON, err := service.UpdateWithAuditDiff(updateProductForm(1, "  新产品  ", "原描述", 250, 0, 0))
	if err != nil {
		t.Fatalf("UpdateWithAuditDiff() error = %v", err)
	}

	var got model.Product
	if err := db.First(&got, 1).Error; err != nil {
		t.Fatalf("query updated product: %v", err)
	}
	if got.Name != "新产品" || got.Price != 250 || got.Status != 0 {
		t.Fatalf("unexpected mutable fields: %#v", got)
	}
	if got.DeptId != 8 || got.CreatedBy != 99 {
		t.Fatalf("update overwrote ownership fields: dept_id=%d created_by=%d", got.DeptId, got.CreatedBy)
	}

	diff := decodeProductDiff(t, diffJSON)
	if len(diff) != 3 {
		t.Fatalf("expected 3 changed fields, got %d: %s", len(diff), diffJSON)
	}
	assertProductDiff(t, diff, "name", "旧产品", "新产品", "")
	assertProductDiff(t, diff, "price", float64(100), float64(250), "")
	assertProductDiff(t, diff, "status", float64(1), float64(0), "禁用")
}

func TestUpdateWithAuditDiffDoesNotRecreateDeletedProduct(t *testing.T) {
	db, service := newProductTestService(t)
	seedProduct(t, db, model.Product{
		ContainsDeleteBaseModel: model.ContainsDeleteBaseModel{BaseModel: model.BaseModel{ID: 2}},
		Name:                    "待删除产品",
		Status:                  1,
		DeptId:                  3,
		CreatedBy:               7,
	})
	if err := db.Model(&model.Product{}).Where("id = ?", 2).Update("deleted_at", time.Now().Unix()).Error; err != nil {
		t.Fatalf("soft delete seed product: %v", err)
	}

	_, err := service.UpdateWithAuditDiff(updateProductForm(2, "复活产品", "", 0, 1, 0))
	assertBusinessErrorCode(t, err, e.NotFound)

	var got model.Product
	if err := db.Unscoped().First(&got, 2).Error; err != nil {
		t.Fatalf("query deleted product: %v", err)
	}
	if got.Name != "待删除产品" || got.DeletedAt == 0 {
		t.Fatalf("deleted product was unexpectedly recreated or changed: %#v", got)
	}
}

func TestUpdateWithAuditDiffKeepsDataScopePredicateOnWrite(t *testing.T) {
	previous := newProductDataScopeFilter
	newProductDataScopeFilter = func() productDataScopeFilter {
		return fixedProductDataScopeFilter{condition: "dept_id = ?", args: []any{uint(8)}}
	}
	defer func() { newProductDataScopeFilter = previous }()

	db, service := newProductTestService(t)
	seedProduct(t, db, model.Product{
		ContainsDeleteBaseModel: model.ContainsDeleteBaseModel{BaseModel: model.BaseModel{ID: 4}},
		Name:                    "范围外产品",
		Status:                  1,
		DeptId:                  9,
		CreatedBy:               7,
	})

	_, err := service.UpdateWithAuditDiff(updateProductForm(4, "越权修改", "", 0, 1, 0))
	assertBusinessErrorCode(t, err, e.NotFound)
	var got model.Product
	if err := db.First(&got, 4).Error; err != nil {
		t.Fatalf("query product outside scope: %v", err)
	}
	if got.Name != "范围外产品" {
		t.Fatalf("product outside scope was updated: %#v", got)
	}
}

func TestDeleteWithAuditDiffSoftDeletesAndCapturesBeforeState(t *testing.T) {
	db, service := newProductTestService(t)
	seedProduct(t, db, model.Product{
		ContainsDeleteBaseModel: model.ContainsDeleteBaseModel{BaseModel: model.BaseModel{ID: 3}},
		Name:                    "下架产品",
		Description:             "删除前快照",
		Price:                   880,
		Status:                  1,
		DeptId:                  6,
		CreatedBy:               12,
	})

	diffJSON, err := service.DeleteWithAuditDiff(3)
	if err != nil {
		t.Fatalf("DeleteWithAuditDiff() error = %v", err)
	}
	var active int64
	if err := db.Model(&model.Product{}).Where("id = ?", 3).Count(&active).Error; err != nil {
		t.Fatalf("count active product: %v", err)
	}
	if active != 0 {
		t.Fatalf("expected soft-deleted product, active count = %d", active)
	}
	diff := decodeProductDiff(t, diffJSON)
	assertProductDiff(t, diff, "name", "下架产品", nil, "")
	assertProductDiff(t, diff, "status", float64(1), nil, "")
}

func TestDeleteWithAuditDiffKeepsDataScopePredicateOnWrite(t *testing.T) {
	previous := newProductDataScopeFilter
	newProductDataScopeFilter = func() productDataScopeFilter {
		return fixedProductDataScopeFilter{condition: "dept_id = ?", args: []any{uint(8)}}
	}
	defer func() { newProductDataScopeFilter = previous }()

	db, service := newProductTestService(t)
	seedProduct(t, db, model.Product{
		ContainsDeleteBaseModel: model.ContainsDeleteBaseModel{BaseModel: model.BaseModel{ID: 5}},
		Name:                    "不可删除产品",
		Status:                  1,
		DeptId:                  9,
		CreatedBy:               7,
	})

	_, err := service.DeleteWithAuditDiff(5)
	assertBusinessErrorCode(t, err, e.NotFound)
	var active int64
	if err := db.Model(&model.Product{}).Where("id = ?", 5).Count(&active).Error; err != nil {
		t.Fatalf("count product outside scope: %v", err)
	}
	if active != 1 {
		t.Fatalf("product outside scope was deleted, active count = %d", active)
	}
}

func TestCreateWithAuditDiffPreservesDisabledStatus(t *testing.T) {
	db, service := newProductTestService(t)
	seedDepartment(t, db, 4)

	diffJSON, err := service.CreateWithAuditDiff(createProductForm("  新建产品  ", "描述", 1200, 0, 4))
	if err != nil {
		t.Fatalf("CreateWithAuditDiff() error = %v", err)
	}
	var got model.Product
	if err := db.First(&got).Error; err != nil {
		t.Fatalf("query created product: %v", err)
	}
	if got.Name != "新建产品" || got.Status != 0 || got.DeptId != 4 || got.CreatedBy != 42 {
		t.Fatalf("unexpected created product: %#v", got)
	}
	diff := decodeProductDiff(t, diffJSON)
	assertProductDiff(t, diff, "name", nil, "新建产品", "")
	assertProductDiff(t, diff, "status", nil, float64(0), "禁用")
}

func TestProductMutationRejectsBlankAndTooLongNameBeforeDatabaseAccess(t *testing.T) {
	service := NewProductService()
	_, err := service.CreateWithAuditDiff(createProductForm(" \t\n ", "", 0, 0, 0))
	assertBusinessErrorCode(t, err, e.InvalidParameter)

	_, err = service.UpdateWithAuditDiff(updateProductForm(1, strings.Repeat("产", 201), "", 0, 0, 0))
	assertBusinessErrorCode(t, err, e.InvalidParameter)
}

func TestUpdateWithAuditDiffPropagatesDatabaseErrors(t *testing.T) {
	db, service := newProductTestService(t)
	if err := db.Exec("DROP TABLE product").Error; err != nil {
		t.Fatalf("drop product table: %v", err)
	}

	_, err := service.UpdateWithAuditDiff(updateProductForm(1, "产品", "", 0, 1, 0))
	assertTechnicalError(t, err)
}

func TestCreateWithAuditDiffPropagatesWriteErrors(t *testing.T) {
	db, service := newProductTestService(t)
	seedDepartment(t, db, 4)
	if err := db.Exec("CREATE TRIGGER reject_product_insert BEFORE INSERT ON product BEGIN SELECT RAISE(ABORT, 'rejected'); END;").Error; err != nil {
		t.Fatalf("create insert rejection trigger: %v", err)
	}

	_, err := service.CreateWithAuditDiff(createProductForm("产品", "", 0, 1, 4))
	assertTechnicalError(t, err)
}

func TestDeleteWithAuditDiffPropagatesWriteErrors(t *testing.T) {
	db, service := newProductTestService(t)
	seedProduct(t, db, model.Product{
		ContainsDeleteBaseModel: model.ContainsDeleteBaseModel{BaseModel: model.BaseModel{ID: 6}},
		Name:                    "删除失败产品",
		Status:                  1,
		DeptId:                  8,
		CreatedBy:               42,
	})
	if err := db.Exec("CREATE TRIGGER reject_product_delete BEFORE UPDATE OF deleted_at ON product BEGIN SELECT RAISE(ABORT, 'rejected'); END;").Error; err != nil {
		t.Fatalf("create delete rejection trigger: %v", err)
	}

	_, err := service.DeleteWithAuditDiff(6)
	assertTechnicalError(t, err)
	var active int64
	if countErr := db.Model(&model.Product{}).Where("id = ?", 6).Count(&active).Error; countErr != nil {
		t.Fatalf("count product after failed delete: %v", countErr)
	}
	if active != 1 {
		t.Fatalf("failed delete changed product state, active count = %d", active)
	}
}

func newProductTestService(t *testing.T) (*gorm.DB, *ProductService) {
	t.Helper()
	dsn := fmt.Sprintf("file:product_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
		Logger:                                   logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get sql db: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = sqlDB.Close() })
	if err := db.Exec(`CREATE TABLE product (
		id integer primary key autoincrement,
		created_at datetime,
		updated_at datetime,
		deleted_at integer not null default 0,
		name text not null default '',
		description text,
		price integer not null default 0,
		status integer not null default 1,
		dept_id integer not null default 0,
		created_by integer not null default 0
	)`).Error; err != nil {
		t.Fatalf("create product table: %v", err)
	}

	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(nil)
	auth.StoreAuthPrincipal(ctx, &auth.AuthPrincipal{UserID: 42, IsSuperAdmin: global.Yes})
	service := NewProductService()
	service.db = db
	service.SetCtx(ctx)
	service.SetAdminUserId(42)
	return db, service
}

func seedProduct(t *testing.T, db *gorm.DB, product model.Product) {
	t.Helper()
	if err := db.Create(&product).Error; err != nil {
		t.Fatalf("seed product: %v", err)
	}
}

func seedDepartment(t *testing.T, db *gorm.DB, id uint) {
	t.Helper()
	if err := db.Exec(`CREATE TABLE department (
		id integer primary key autoincrement,
		created_at datetime,
		updated_at datetime,
		deleted_at integer not null default 0
	)`).Error; err != nil {
		t.Fatalf("create department table: %v", err)
	}
	if err := db.Exec("INSERT INTO department (id, deleted_at) VALUES (?, 0)", id).Error; err != nil {
		t.Fatalf("seed department: %v", err)
	}
}

func createProductForm(name, description string, price int64, status uint8, deptID uint) *form.CreateProduct {
	params := form.NewCreateProductForm()
	params.Name = name
	params.Description = description
	params.Price = price
	params.Status = status
	params.DeptId = deptID
	return params
}

func updateProductForm(id uint, name, description string, price int64, status uint8, deptID uint) *form.UpdateProduct {
	params := form.NewUpdateProductForm()
	params.Id = id
	params.Name = name
	params.Description = description
	params.Price = price
	params.Status = status
	params.DeptId = deptID
	return params
}

func decodeProductDiff(t *testing.T, raw string) []auditdiff.ChangeDiffItem {
	t.Helper()
	var diff []auditdiff.ChangeDiffItem
	if err := json.Unmarshal([]byte(raw), &diff); err != nil {
		t.Fatalf("decode product diff %q: %v", raw, err)
	}
	return diff
}

func assertProductDiff(t *testing.T, diff []auditdiff.ChangeDiffItem, field string, before, after any, afterDisplay string) {
	t.Helper()
	for _, item := range diff {
		if item.Field != field {
			continue
		}
		if fmt.Sprint(item.Before) != fmt.Sprint(before) || fmt.Sprint(item.After) != fmt.Sprint(after) {
			t.Fatalf("unexpected %s diff: %#v", field, item)
		}
		if afterDisplay != "" && item.AfterDisplay != afterDisplay {
			t.Fatalf("unexpected %s after display: %q", field, item.AfterDisplay)
		}
		return
	}
	t.Fatalf("missing %s diff in %#v", field, diff)
}

func assertBusinessErrorCode(t *testing.T, err error, code int) {
	t.Helper()
	var businessErr *e.BusinessError
	if !errors.As(err, &businessErr) {
		t.Fatalf("expected business error code %d, got %v", code, err)
	}
	if businessErr.GetCode() != code {
		t.Fatalf("expected business error code %d, got %d", code, businessErr.GetCode())
	}
}

func assertTechnicalError(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("expected technical error")
	}
	var businessErr *e.BusinessError
	if errors.As(err, &businessErr) {
		t.Fatalf("technical error was incorrectly converted to business error: %v", err)
	}
}

type fixedProductDataScopeFilter struct {
	condition string
	args      []any
}

func (f fixedProductDataScopeFilter) Build(_ context.Context, _ access.DataScopeInput, _, _ string) (string, []any) {
	return f.condition, f.args
}
