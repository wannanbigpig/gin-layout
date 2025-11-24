package resources

import (
	"github.com/wannanbigpig/gin-layout/internal/model"
	"github.com/wannanbigpig/gin-layout/internal/pkg/utils"
)

// RequestLogBaseResources 请求日志基础资源（公共字段）
type RequestLogBaseResources struct {
	ID                  uint             `json:"id"`
	RequestID           string           `json:"request_id"`            // 请求唯一标识
	OperatorID          uint             `json:"operator_id"`           // 操作ID（用户ID）
	IP                  string           `json:"ip"`                    // 客户端IP地址
	Method              string           `json:"method"`                // HTTP请求方法（GET/POST等）
	BaseURL             string           `json:"base_url"`              // 请求基础URL
	OperationName       string           `json:"operation_name"`        // 操作名称
	OperationStatus     int              `json:"operation_status"`      // 操作状态码（响应返回的code，0=成功，其他=失败）
	OperationStatusName string           `json:"operation_status_name"` // 操作状态名称
	OperatorAccount     string           `json:"operator_account"`      // 操作账号
	OperatorName        string           `json:"operator_name"`         // 操作人员
	ResponseStatus      int              `json:"response_status"`       // 响应状态码
	ExecutionTime       int              `json:"execution_time"`        // 执行时间（毫秒）
	CreatedAt           utils.FormatDate `json:"created_at"`            // 创建时间
}

// RequestLogListResources 请求日志列表资源（简化版，不包含大字段）
type RequestLogListResources struct {
	RequestLogBaseResources
}

// RequestLogResources 请求日志详情资源
type RequestLogResources struct {
	RequestLogBaseResources
	JwtID          string           `json:"jwt_id"`          // 请求授权的jwtId
	UserAgent      string           `json:"user_agent"`      // 用户代理（浏览器/设备信息）
	OS             string           `json:"os"`              // 操作系统
	Browser        string           `json:"browser"`         // 浏览器
	RequestHeaders string           `json:"request_headers"` // 请求头（JSON格式）
	RequestQuery   string           `json:"request_query"`   // 请求参数
	RequestBody    string           `json:"request_body"`    // 请求体
	ResponseBody   string           `json:"response_body"`   // 响应体
	ResponseHeader string           `json:"response_header"` // 响应头
	UpdatedAt      utils.FormatDate `json:"updated_at"`      // 更新时间
}

// RequestLogTransformer 请求日志资源转换器
type RequestLogTransformer struct {
	BaseResources[*model.RequestLogs, *RequestLogResources]
}

// NewRequestLogTransformer 实例化请求日志资源转换器
func NewRequestLogTransformer() RequestLogTransformer {
	return RequestLogTransformer{
		BaseResources: BaseResources[*model.RequestLogs, *RequestLogResources]{
			NewResource: func() *RequestLogResources {
				return &RequestLogResources{}
			},
		},
	}
}

// buildRequestLogBaseResources 构建基础资源（公共字段）
func buildRequestLogBaseResources(data *model.RequestLogs) RequestLogBaseResources {
	return RequestLogBaseResources{
		ID:                  data.ID,
		RequestID:           data.RequestID,
		OperatorID:          data.OperatorID,
		IP:                  data.IP,
		Method:              data.Method,
		BaseURL:             data.BaseURL,
		OperationName:       data.OperationName,
		OperationStatus:     data.OperationStatus,
		OperationStatusName: getOperationStatusName(data.OperationStatus),
		OperatorAccount:     data.OperatorAccount,
		OperatorName:        data.OperatorName,
		ResponseStatus:      data.ResponseStatus,
		ExecutionTime:       data.ExecutionTime,
		CreatedAt:           data.CreatedAt,
	}
}

// ToStruct 转换为单个资源（详情）
func (r RequestLogTransformer) ToStruct(data *model.RequestLogs) *RequestLogResources {
	base := buildRequestLogBaseResources(data)
	return &RequestLogResources{
		RequestLogBaseResources: base,
		JwtID:                   data.JwtID,
		UserAgent:               data.UserAgent,
		OS:                      data.OS,
		Browser:                 data.Browser,
		RequestHeaders:          data.RequestHeaders,
		RequestQuery:            data.RequestQuery,
		RequestBody:             data.RequestBody,
		ResponseBody:            data.ResponseBody,
		ResponseHeader:          data.ResponseHeader,
		UpdatedAt:               data.UpdatedAt,
	}
}

// ToCollection 转换为集合资源（列表，不包含大字段）
func (r RequestLogTransformer) ToCollection(page, perPage int, total int64, data []*model.RequestLogs) *Collection {
	response := make([]any, 0, len(data))
	for _, v := range data {
		base := buildRequestLogBaseResources(v)
		response = append(response, &RequestLogListResources{
			RequestLogBaseResources: base,
		})
	}
	return NewCollection().SetPaginate(page, perPage, total).ToCollection(response)
}

// getOperationStatusName 获取操作状态名称
func getOperationStatusName(code int) string {
	if code == 0 {
		return "成功"
	}
	return "失败"
}
