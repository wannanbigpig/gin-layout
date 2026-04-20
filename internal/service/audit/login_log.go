package audit

import (
	"github.com/wannanbigpig/gin-layout/config"
	"github.com/wannanbigpig/gin-layout/internal/model"
	e "github.com/wannanbigpig/gin-layout/internal/pkg/errors"
	log "github.com/wannanbigpig/gin-layout/internal/pkg/logger"
	"github.com/wannanbigpig/gin-layout/internal/resources"
	"github.com/wannanbigpig/gin-layout/internal/service"
	"github.com/wannanbigpig/gin-layout/internal/validator/form"
	"github.com/wannanbigpig/gin-layout/pkg/utils/crypto"
	"go.uber.org/zap"
)

// LoginLogService 登录日志服务
type AdminLoginLogService struct {
	service.Base
	configProvider func() *config.Conf
}

// AdminLoginLogServiceDeps 描述 AdminLoginLogService 可注入依赖。
type AdminLoginLogServiceDeps struct {
	ConfigProvider func() *config.Conf
}

// NewAdminLoginLogService 创建登录日志服务实例
func NewAdminLoginLogService() *AdminLoginLogService {
	return NewAdminLoginLogServiceWithDeps(AdminLoginLogServiceDeps{})
}

// NewAdminLoginLogServiceWithDeps 创建带依赖注入的登录日志服务实例。
func NewAdminLoginLogServiceWithDeps(deps AdminLoginLogServiceDeps) *AdminLoginLogService {
	s := &AdminLoginLogService{
		configProvider: deps.ConfigProvider,
	}
	s.ensureRuntimeDeps()
	return s
}

func (s *AdminLoginLogService) ensureRuntimeDeps() {
	if s.configProvider == nil {
		s.configProvider = config.GetConfig
	}
}

func (s *AdminLoginLogService) currentConfig() *config.Conf {
	s.ensureRuntimeDeps()
	return config.GetConfigFrom(s.configProvider)
}

// List 分页查询登录日志列表
func (s *AdminLoginLogService) List(params *form.AdminLoginLogList) *resources.Collection {
	query := newLogListQuery().
		addLike("username", params.Username).
		addEq("login_status", params.LoginStatus).
		addLike("ip", params.IP).
		addCreatedAtRange(params.StartTime, params.EndTime)
	conditionStr, args := query.Build()
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
	total, collection, err := model.ListPageE(loginLogModel, params.Page, params.PerPage, conditionStr, args, listOptionalParams)
	if err != nil {
		log.Logger.Error("查询登录日志列表失败", zap.Error(err))
		return resources.NewAdminLoginLogTransformer().ToCollection(params.Page, params.PerPage, 0, nil)
	}

	// 使用资源类转换，列表不包含大字段
	transformer := resources.NewAdminLoginLogTransformer()
	return transformer.ToCollection(params.Page, params.PerPage, total, collection)
}

// Detail 获取登录日志详情
func (s *AdminLoginLogService) Detail(id uint) (any, error) {
	loginLog := model.NewAdminLoginLogs()
	if err := loginLog.GetById(id); err != nil || loginLog.ID == 0 {
		return nil, e.NewBusinessError(1, "登录日志不存在")
	}
	decryptKey := s.currentConfig().Jwt.SecretKey
	loginLog.AccessToken = decryptLoginTokenIfNeeded(loginLog.AccessToken, decryptKey)
	loginLog.RefreshToken = decryptLoginTokenIfNeeded(loginLog.RefreshToken, decryptKey)

	transformer := resources.NewAdminLoginLogTransformer()
	return transformer.ToStruct(loginLog), nil
}

func decryptLoginTokenIfNeeded(token, decryptKey string) string {
	if token == "" || decryptKey == "" {
		return token
	}
	decrypted, err := crypto.Decrypt(decryptKey, token)
	if err != nil {
		return token
	}
	return decrypted
}
