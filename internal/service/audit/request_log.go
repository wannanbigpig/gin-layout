package audit

import (
	"errors"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/wannanbigpig/gin-layout/internal/global"
	"github.com/wannanbigpig/gin-layout/internal/model"
	e "github.com/wannanbigpig/gin-layout/internal/pkg/errors"
	"github.com/wannanbigpig/gin-layout/internal/pkg/i18n"
	log "github.com/wannanbigpig/gin-layout/internal/pkg/logger"
	"github.com/wannanbigpig/gin-layout/internal/pkg/utils"
	"github.com/wannanbigpig/gin-layout/internal/resources"
	"github.com/wannanbigpig/gin-layout/internal/service"
	"github.com/wannanbigpig/gin-layout/internal/validator/form"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// RequestLogService 请求日志服务
type RequestLogService struct {
	service.Base
	db *gorm.DB
}

var selfRequestLogReadPaths = []string{
	"/admin/v1/log/request/self-list",
	"/admin/v1/log/request/self-detail",
}

// NewRequestLogService 创建请求日志服务实例
func NewRequestLogService() *RequestLogService {
	return &RequestLogService{}
}

func (s *RequestLogService) dbOrDefault() (*gorm.DB, error) {
	if s.db != nil {
		return s.db, nil
	}
	return model.GetDB()
}

// List 分页查询请求日志列表
func (s *RequestLogService) List(params *form.RequestLogList) (*resources.Collection, error) {
	return s.list(params, params.OperatorID, false)
}

// SelfList 分页查询当前登录用户自己的请求日志，客户端 operator_id 不参与授权范围。
func (s *RequestLogService) SelfList(params *form.RequestLogList, operatorID uint) (*resources.Collection, error) {
	if operatorID == 0 {
		return nil, e.NewBusinessError(e.NotLogin)
	}
	return s.list(params, operatorID, true)
}

func (s *RequestLogService) list(params *form.RequestLogList, operatorID uint, excludeSelfReadPaths bool) (*resources.Collection, error) {
	if err := ValidateRequestLogTimeRange(params.StartTime, params.EndTime); err != nil {
		return nil, err
	}
	query := buildRequestLogQuery(requestLogQueryInput{
		RequestID:          params.RequestID,
		HasChangeDiff:      params.HasChangeDiff,
		OperatorID:         operatorID,
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
	if excludeSelfReadPaths {
		applySelfRequestLogScope(query, params)
	}
	conditionStr, args := query.Build()
	requestLogModel := model.NewRequestLogs()
	db, err := s.dbOrDefault()
	if err != nil {
		return nil, err
	}
	requestLogModel.SetDB(db)

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
	if excludeSelfReadPaths {
		// 个人列表读取安全终端字段与 change_diff；资源层不会返回原始差异。
		listOptionalParams.SelectFields = append(listOptionalParams.SelectFields, "os", "browser", "change_diff")
	}
	locale := localeFromCtx(s.GetCtx())
	transformer := resources.NewRequestLogTransformer(locale)

	// 分页查询（只查询列表需要的字段）
	total, collection, err := model.ListPageE(requestLogModel, params.Page, params.PerPage, conditionStr, args, listOptionalParams)
	if err != nil {
		log.Logger.Error("查询请求日志列表失败", zap.Error(err))
		return nil, err
	}

	// 使用资源类转换，列表不包含大字段
	if excludeSelfReadPaths {
		return transformer.ToSelfCollection(params.Page, params.PerPage, total, collection), nil
	}
	return transformer.ToCollection(params.Page, params.PerPage, total, collection), nil
}

// ValidateRequestLogTimeRange 拒绝无效或反向的日志时间范围，避免静默扩大查询结果。
func ValidateRequestLogTimeRange(startTime, endTime string) error {
	var start, end time.Time
	var hasStart, hasEnd bool
	if value := strings.TrimSpace(startTime); value != "" {
		var ok bool
		start, ok = utils.ParseTimeBoundary(value)
		if !ok {
			return e.NewBusinessError(e.InvalidParameter, "start_time 时间格式无效")
		}
		hasStart = true
	}
	if value := strings.TrimSpace(endTime); value != "" {
		var ok bool
		end, ok = utils.ParseTimeBoundary(value)
		if !ok {
			return e.NewBusinessError(e.InvalidParameter, "end_time 时间格式无效")
		}
		hasEnd = true
	}
	if hasStart && hasEnd && start.After(end) {
		return e.NewBusinessError(e.InvalidParameter, "start_time 不能晚于 end_time")
	}
	return nil
}

func applySelfRequestLogScope(query *logListQuery, params *form.RequestLogList) {
	scope := strings.TrimSpace(params.Scope)
	switch scope {
	case "all":
		return
	case "important":
		query.addCondition("base_url NOT IN (?, ?)", selfRequestLogReadPaths[0], selfRequestLogReadPaths[1])
		query.addCondition("(operation_status <> 0 OR is_high_risk = 1 OR (change_diff IS NOT NULL AND TRIM(change_diff) NOT IN (?, ?, ?)) OR method IN (?, ?, ?, ?))", "", "[]", "{}", "POST", "PUT", "PATCH", "DELETE")
	case "changes":
		query.addCondition("change_diff IS NOT NULL AND TRIM(change_diff) NOT IN (?, ?, ?)", "", "[]", "{}")
	case "failed":
		query.addCondition("operation_status <> ?", 0)
	case "high_risk":
		query.addCondition("is_high_risk = ?", 1)
	case "self_reads":
		query.addCondition("base_url IN (?, ?)", selfRequestLogReadPaths[0], selfRequestLogReadPaths[1])
	default: // 空值与 regular 保持个人日志原有行为。
		if strings.TrimSpace(params.OperationName) == "" {
			query.addCondition("base_url NOT IN (?, ?)", selfRequestLogReadPaths[0], selfRequestLogReadPaths[1])
		}
	}
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
	if s.db != nil {
		requestLog.SetDB(s.db)
	}
	if err := requestLog.GetById(id); err != nil || requestLog.ID == 0 {
		return nil, e.NewBusinessError(e.NotFound)
	}
	// 使用资源类转换，详情包含所有字段
	locale := localeFromCtx(s.GetCtx())
	transformer := resources.NewRequestLogTransformer(locale)
	return transformer.ToStruct(requestLog), nil
}

// SelfDetail 按日志 ID 与当前用户 ID 联合查询，其他用户的记录统一表现为不存在。
func (s *RequestLogService) SelfDetail(id, operatorID uint) (any, error) {
	if operatorID == 0 {
		return nil, e.NewBusinessError(e.NotLogin)
	}
	requestLog := model.NewRequestLogs()
	if s.db != nil {
		requestLog.SetDB(s.db)
	}
	if err := requestLog.GetDetail("id = ? AND operator_id = ?", id, operatorID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, e.NewBusinessError(e.NotFound)
		}
		return nil, err
	}
	if requestLog.ID == 0 {
		return nil, e.NewBusinessError(e.NotFound)
	}
	locale := localeFromCtx(s.GetCtx())
	transformer := resources.NewRequestLogTransformer(locale)
	return transformer.ToSelfDetail(requestLog), nil
}

type requestLogQueryInput struct {
	HasChangeDiff      uint8
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
	var operatorID any
	if input.OperatorID > 0 {
		operatorID = input.OperatorID
	}
	query := newLogListQuery().
		addEq("request_id", input.RequestID).
		addEq("operator_id", operatorID).
		addLike("operator_account", input.OperatorAccount).
		addLike("base_url", input.BaseURL).
		addEq("request_kind", input.RequestKind).
		addEq("execution_time_scope", input.ExecutionTimeScope).
		addEq("method", input.Method).
		addLike("operation_name", input.OperationName).
		addLike("ip", input.IP).
		addEq("is_high_risk", input.IsHighRisk).
		addCreatedAtRange(input.StartTime, input.EndTime)

	if input.HasChangeDiff == 1 {
		query.addCondition("change_diff IS NOT NULL AND TRIM(change_diff) NOT IN (?, ?, ?)", "", "[]", "{}")
	}
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
