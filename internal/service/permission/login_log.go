package permission

import (
	"strings"

	"github.com/wannanbigpig/gin-layout/internal/model"
	e "github.com/wannanbigpig/gin-layout/internal/pkg/errors"
	"github.com/wannanbigpig/gin-layout/internal/resources"
	"github.com/wannanbigpig/gin-layout/internal/service"
	"github.com/wannanbigpig/gin-layout/internal/validator/form"
)

// LoginLogService 登录日志服务
type AdminLoginLogService struct {
	service.Base
}

// NewAdminLoginLogService 创建登录日志服务实例
func NewAdminLoginLogService() *AdminLoginLogService {
	return &AdminLoginLogService{}
}

// List 分页查询登录日志列表
func (s *AdminLoginLogService) List(params *form.AdminLoginLogList) *resources.Collection {
	var conditions []string
	var args []any

	// 登录账号（模糊查询）
	if params.Username != "" {
		conditions = append(conditions, "username LIKE ?")
		args = append(args, "%"+params.Username+"%")
	}

	// 登录状态
	if params.LoginStatus != nil {
		conditions = append(conditions, "login_status = ?")
		args = append(args, *params.LoginStatus)
	}

	// 登录IP（模糊查询）
	if params.IP != "" {
		conditions = append(conditions, "ip LIKE ?")
		args = append(args, "%"+params.IP+"%")
	}

	// 开始时间
	if params.StartTime != "" {
		conditions = append(conditions, "created_at >= ?")
		args = append(args, params.StartTime)
	}

	// 结束时间
	if params.EndTime != "" {
		conditions = append(conditions, "created_at <= ?")
		args = append(args, params.EndTime)
	}

	conditionStr := strings.Join(conditions, " AND ")
	loginLogModel := model.NewAdminLoginLogs()

	// 构建查询参数，只查询列表需要的字段，排除大字段
	listOptionalParams := model.ListOptionalParams{
		SelectFields: []string{
			"id",
			"uid",
			"username",
			"ip",
			"os",
			"browser",
			"execution_time",
			"login_status",
			"login_fail_reason",
			"type",
			"is_revoked",
			"revoked_code",
			"revoked_reason",
			"revoked_at",
			"created_at",
		},
		OrderBy: "created_at DESC, id DESC",
	}

	// 分页查询（只查询列表需要的字段）
	total, collection := model.ListPage(loginLogModel, params.Page, params.PerPage, conditionStr, args, listOptionalParams)

	// 使用资源类转换，列表不包含大字段
	transformer := resources.NewAdminLoginLogTransformer()
	return transformer.ToCollection(params.Page, params.PerPage, total, collection)
}

// Detail 获取登录日志详情
func (s *AdminLoginLogService) Detail(id uint) (any, error) {
	loginLog := model.NewAdminLoginLogs()
	if err := loginLog.GetById(loginLog, id); err != nil || loginLog.ID == 0 {
		return nil, e.NewBusinessError(1, "登录日志不存在")
	}
	// 使用资源类转换，详情包含所有字段
	transformer := resources.NewAdminLoginLogTransformer()
	return transformer.ToStruct(loginLog), nil
}
