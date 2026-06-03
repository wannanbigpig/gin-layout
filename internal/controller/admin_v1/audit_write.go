package admin_v1

import (
	"github.com/gin-gonic/gin"

	"github.com/wannanbigpig/gin-layout/internal/controller"
	"github.com/wannanbigpig/gin-layout/internal/middleware"
	"github.com/wannanbigpig/gin-layout/internal/validator"
)

// handleAuditWrite 处理「绑定 POST 参数 -> 调用带审计 diff 的 service -> 记录 diff -> 返回成功」的通用写流程。
// newForm 返回表单指针；action 接收该表单并返回审计 diff(JSON 字符串)与错误。
func handleAuditWrite[F any](api controller.Api, c *gin.Context, newForm func() F, action func(F) (string, error)) {
	params := newForm()
	if err := validator.CheckPostParams(c, &params); err != nil {
		return
	}
	changeDiff, err := action(params)
	if err != nil {
		api.Err(c, err)
		return
	}
	middleware.SetAuditChangeDiffRaw(c, changeDiff)
	api.Success(c, nil)
}
