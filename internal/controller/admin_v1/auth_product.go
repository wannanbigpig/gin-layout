package admin_v1

import (
	"github.com/gin-gonic/gin"
	"github.com/wannanbigpig/gin-layout/internal/controller"
	"github.com/wannanbigpig/gin-layout/internal/service/product"
	"github.com/wannanbigpig/gin-layout/internal/validator"
	"github.com/wannanbigpig/gin-layout/internal/validator/form"
)

type ProductController struct {
	controller.Api
}

// NewProductController 创建产品控制器实例
func NewProductController() *ProductController {
	return &ProductController{}
}

func (api ProductController) newProductService(ctx *gin.Context) *product.ProductService {
	svc := product.NewProductService()
	svc.SetCtx(ctx)
	svc.SetAdminUserId(api.GetCurrentUserID(ctx))
	return svc
}

// List 产品列表
func (api ProductController) List(ctx *gin.Context) {
	params := form.NewProductListQuery()
	if err := validator.CheckQueryParams(ctx, &params); err != nil {
		return
	}

	data, err := api.newProductService(ctx).List(params)
	if err != nil {
		api.Err(ctx, err)
		return
	}
	api.Success(ctx, data)
}

// Create 新增产品
func (api ProductController) Create(ctx *gin.Context) {
	service := api.newProductService(ctx)
	handleAuditWrite(api.Api, ctx, form.NewCreateProductForm, func(params *form.CreateProduct) (string, error) {
		return service.CreateWithAuditDiff(params)
	})
}

// Update 更新产品
func (api ProductController) Update(ctx *gin.Context) {
	service := api.newProductService(ctx)
	handleAuditWrite(api.Api, ctx, form.NewUpdateProductForm, func(params *form.UpdateProduct) (string, error) {
		return service.UpdateWithAuditDiff(params)
	})
}

// Delete 删除产品
func (api ProductController) Delete(ctx *gin.Context) {
	service := api.newProductService(ctx)
	handleAuditWrite(api.Api, ctx, form.NewIdForm, func(params *form.ID) (string, error) {
		return service.DeleteWithAuditDiff(params.ID)
	})
}

// Detail 产品详情
func (api ProductController) Detail(ctx *gin.Context) {
	query := form.NewIdForm()
	if err := validator.CheckQueryParams(ctx, &query); err != nil {
		return
	}

	data, err := api.newProductService(ctx).Detail(query.ID)
	if err != nil {
		api.Err(ctx, err)
		return
	}
	api.Success(ctx, data)
}
