package audit

import (
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/wannanbigpig/gin-layout/internal/global"
	"github.com/wannanbigpig/gin-layout/internal/model"
	e "github.com/wannanbigpig/gin-layout/internal/pkg/errors"
	"github.com/wannanbigpig/gin-layout/internal/pkg/i18n"
	log "github.com/wannanbigpig/gin-layout/internal/pkg/logger"
	"github.com/wannanbigpig/gin-layout/internal/resources"
	"github.com/wannanbigpig/gin-layout/internal/service"
	"github.com/wannanbigpig/gin-layout/internal/validator/form"
	"go.uber.org/zap"
)

// RequestLogService 请求日志服务
type RequestLogService struct {
	service.Base
}

// NewRequestLogService 创建请求日志服务实例
func NewRequestLogService() *RequestLogService {
	return &RequestLogService{}
}

// List 分页查询请求日志列表
func (s *RequestLogService) List(params *form.RequestLogList) *resources.Collection {
	query := buildRequestLogQuery(requestLogQueryInput{
		RequestID:          params.RequestID,
		OperatorID:         params.OperatorID,
		OperatorAccount:    params.OperatorAccount,
		OperationStatus:    params.OperationStatus,
		IsHighRisk:         params.IsHighRisk,
		RequestKind:        params.RequestKind,
		ExecutionTimeScope: params.ExecutionTimeScope,
		Method:             params.Method,
		BaseURL:            params.BaseURL,
		OperationName:      params.OperationName,
		IP:                 params.IP,
		StartTime:          params.StartTime,
		EndTime:            params.EndTime,
	})
	conditionStr, args := query.Build()
	requestLogModel := model.NewRequestLogs()

	// 构建查询参数，只查询列表需要的字段，排除大字段
	listOptionalParams := model.ListOptionalParams{
		SelectFields: []string{
			"id",
			"request_id",
			"operator_id",
			"ip",
			"request_kind",
			"method",
			"base_url",
			"operation_name",
			"operation_status",
			"is_high_risk",
			"operator_account",
			"operator_name",
			"response_status",
			"execution_time_us",
			"execution_time_scope",
			"created_at",
		},
		OrderBy:       resolveRequestLogOrderBy(params.OrderBy),
		OrderAllowMap: requestLogOrderAllowMap,
	}
	locale := localeFromCtx(s.GetCtx())
	transformer := resources.NewRequestLogTransformer(locale)

	// 分页查询（只查询列表需要的字段）
	total, collection, err := model.ListPageE(requestLogModel, params.Page, params.PerPage, conditionStr, args, listOptionalParams)
	if err != nil {
		log.Logger.Error("查询请求日志列表失败", zap.Error(err))
		return transformer.ToCollection(params.Page, params.PerPage, 0, nil)
	}

	// 使用资源类转换，列表不包含大字段
	return transformer.ToCollection(params.Page, params.PerPage, total, collection)
}

var requestLogOrderAllowMap = map[string]struct{}{
	"id":                {},
	"created_at":        {},
	"execution_time_us": {},
	"response_status":   {},
	"operation_status":  {},
}

func resolveRequestLogOrderBy(orderBy string) string {
	orderBy = strings.TrimSpace(orderBy)
	if orderBy == "" {
		return "created_at DESC, id DESC"
	}
	return orderBy + ", id DESC"
}

// localeFromCtx 从 gin.Context 中读取归一化语言，用于资源层国际化。
func localeFromCtx(c *gin.Context) string {
	if c == nil {
		return i18n.DefaultLocale
	}
	if locale, exists := c.Get(global.ContextKeyLocale); exists {
		if localeText, ok := locale.(string); ok {
			return i18n.NormalizeLocale(localeText)
		}
	}
	return i18n.DefaultLocale
}

// Detail 获取请求日志详情
func (s *RequestLogService) Detail(id uint) (any, error) {
	requestLog := model.NewRequestLogs()
	if err := requestLog.GetById(id); err != nil || requestLog.ID == 0 {
		return nil, e.NewBusinessError(e.NotFound)
	}
	// 使用资源类转换，详情包含所有字段
	locale := localeFromCtx(s.GetCtx())
	transformer := resources.NewRequestLogTransformer(locale)
	return transformer.ToStruct(requestLog), nil
}

type requestLogQueryInput struct {
	RequestID          string
	OperatorID         uint
	OperatorAccount    string
	OperationStatus    *int
	IsHighRisk         *uint8
	RequestKind        string
	ExecutionTimeScope string
	Method             string
	BaseURL            string
	OperationName      string
	IP                 string
	StartTime          string
	EndTime            string
}

func buildRequestLogQuery(input requestLogQueryInput) *logListQuery {
	query := newLogListQuery().
		addEq("request_id", input.RequestID).
		addEq("operator_id", uintFilterValue(input.OperatorID)).
		addLike("operator_account", input.OperatorAccount).
		addLike("base_url", input.BaseURL).
		addEq("request_kind", input.RequestKind).
		addEq("execution_time_scope", input.ExecutionTimeScope).
		addEq("method", input.Method).
		addLike("operation_name", input.OperationName).
		addLike("ip", input.IP).
		addEq("is_high_risk", input.IsHighRisk).
		addCreatedAtRange(input.StartTime, input.EndTime)

	if input.OperationStatus != nil {
		switch *input.OperationStatus {
		case 0:
			query.addCondition("operation_status = ?", 0)
		case 1:
			query.addCondition("operation_status != ?", 0)
		}
	}
	return query
}
