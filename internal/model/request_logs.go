package model

// RequestLogs 请求日志表
type RequestLogs struct {
	BaseModel
	RequestID       string  `json:"request_id"`       // 请求唯一标识
	JwtID           string  `json:"jwt_id"`           // 请求授权的jwtId
	OperatorID      uint    `json:"operator_id"`      // 操作ID（用户ID）
	IP              string  `json:"ip"`               // 客户端IP地址
	UserAgent       string  `json:"user_agent"`       // 用户代理（浏览器/设备信息）
	OS              string  `json:"os"`               // 操作系统
	Browser         string  `json:"browser"`          // 浏览器
	Method          string  `json:"method"`           // HTTP请求方法（GET/POST等）
	BaseURL         string  `json:"base_url"`         // 请求基础URL
	OperationName   string  `json:"operation_name"`   // 操作名称
	OperationStatus int     `json:"operation_status"` // 操作状态码（响应返回的code，0=成功，其他=失败）
	OperatorAccount string  `json:"operator_account"` // 操作账号
	OperatorName    string  `json:"operator_name"`    // 操作人员
	RequestHeaders  string  `json:"request_headers"`  // 请求头（JSON格式）
	RequestQuery    string  `json:"request_query"`    // 请求参数
	RequestBody     string  `json:"request_body"`     // 请求体
	ResponseStatus  int     `json:"response_status"`  // 响应状态码
	ResponseBody    string  `json:"response_body"`    // 响应体
	ResponseHeader  string  `json:"response_header"`  // 响应头
	ExecutionTime   float64 `json:"execution_time"`   // 执行时间（毫秒，支持小数，最多4位）
}

func NewRequestLogs() *RequestLogs {
	return BindModel(&RequestLogs{})
}

// TableName 获取表名
func (m *RequestLogs) TableName() string {
	return "request_logs"
}
