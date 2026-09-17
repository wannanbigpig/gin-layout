package resources

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/wannanbigpig/gin-layout/internal/model"
	e "github.com/wannanbigpig/gin-layout/internal/pkg/errors"
	"github.com/wannanbigpig/gin-layout/internal/pkg/i18n"
	"github.com/wannanbigpig/gin-layout/internal/pkg/utils"
)

// RequestLogBaseResources 表示请求日志的公共响应字段。
type RequestLogBaseResources struct {
	ID                  uint             `json:"id"`
	RequestID           string           `json:"request_id"`            // 请求唯一标识
	OperatorID          uint             `json:"operator_id"`           // 操作ID（用户ID）
	IP                  string           `json:"ip"`                    // 客户端IP地址
	RequestKind         string           `json:"request_kind"`          // 请求类型：http/websocket
	Method              string           `json:"method"`                // HTTP请求方法（GET/POST等）
	BaseURL             string           `json:"base_url"`              // 请求基础URL
	OperationName       string           `json:"operation_name"`        // 操作名称
	OperationStatus     int              `json:"operation_status"`      // 操作状态码（响应返回的code，0=成功，其他=失败）
	OperationStatusName string           `json:"operation_status_name"` // 操作状态名称
	IsHighRisk          uint8            `json:"is_high_risk"`          // 是否高危操作
	OperatorAccount     string           `json:"operator_account"`      // 操作账号
	OperatorName        string           `json:"operator_name"`         // 操作人员
	ResponseStatus      int              `json:"response_status"`       // 响应状态码
	ExecutionTimeUs     int64            `json:"execution_time_us"`     // 执行时间（微秒，统一单位）
	ExecutionTimeScope  string           `json:"execution_time_scope"`  // 执行时间语义
	CreatedAt           utils.FormatDate `json:"created_at"`            // 创建时间
}

// RequestLogListResources 表示请求日志列表项。
type RequestLogListResources struct {
	RequestLogBaseResources
}

// RequestLogSelfListResources 表示个人日志列表项；只追加安全摘要，不暴露原始差异。
type RequestLogSelfListResources struct {
	RequestLogBaseResources
	OS            string                                `json:"os"`
	Browser       string                                `json:"browser"`
	ChangeSummary *RequestLogSelfChangeSummaryResources `json:"change_summary,omitempty"`
}

// RequestLogSelfChangeItemResources 表示可安全展示给当前用户的单项变更。
// 不返回内部字段名和原始值，避免管理员审计结构外泄。
type RequestLogSelfChangeItemResources struct {
	Label      string `json:"label"`
	ChangeType string `json:"change_type"`
	BeforeText string `json:"before_text,omitempty"`
	AfterText  string `json:"after_text,omitempty"`
}

// RequestLogSelfChangeSummaryResources 表示个人详情中的安全变更摘要。
type RequestLogSelfChangeSummaryResources struct {
	HasChanges   bool                                `json:"has_changes"`
	Items        []RequestLogSelfChangeItemResources `json:"items"`
	OmittedCount int                                 `json:"omitted_count"`
}

// RequestLogSelfDetailResources 表示当前用户查看本人请求日志时可见的详情字段。
// 审计原文只对拥有全局请求日志权限的管理员开放。
type RequestLogSelfDetailResources struct {
	RequestLogBaseResources
	UserAgent     string                                `json:"user_agent"`     // 用户代理（浏览器/设备信息）
	OS            string                                `json:"os"`             // 操作系统
	Browser       string                                `json:"browser"`        // 浏览器
	ChangeSummary *RequestLogSelfChangeSummaryResources `json:"change_summary"` // 安全裁剪后的自然语言展示数据
}

// RequestLogResources 表示请求日志详情响应。
type RequestLogResources struct {
	RequestLogBaseResources
	JwtID          string           `json:"jwt_id"`          // 请求授权的jwtId
	UserAgent      string           `json:"user_agent"`      // 用户代理（浏览器/设备信息）
	OS             string           `json:"os"`              // 操作系统
	Browser        string           `json:"browser"`         // 浏览器
	RequestHeaders string           `json:"request_headers"` // 请求头（JSON格式）
	RequestQuery   string           `json:"request_query"`   // 请求参数
	RequestBody    string           `json:"request_body"`    // 请求体
	ChangeDiff     string           `json:"change_diff"`     // 关键变更前后差异（JSON）
	ResponseBody   string           `json:"response_body"`   // 响应体
	ResponseHeader string           `json:"response_header"` // 响应头
	UpdatedAt      utils.FormatDate `json:"updated_at"`      // 更新时间
}

// RequestLogTransformer 负责请求日志资源转换。
type RequestLogTransformer struct {
	BaseResources[*model.RequestLogs, *RequestLogResources]
	language string
}

// NewRequestLogTransformer 创建请求日志资源转换器。
// locale 为空时使用默认语言。
func NewRequestLogTransformer(locale string) RequestLogTransformer {
	return RequestLogTransformer{
		BaseResources: BaseResources[*model.RequestLogs, *RequestLogResources]{
			NewResource: func() *RequestLogResources {
				return &RequestLogResources{}
			},
		},
		language: i18n.ToErrorLanguage(locale),
	}
}

// buildRequestLogBaseResources 提取请求日志公共字段。
func buildRequestLogBaseResources(data *model.RequestLogs, language string) RequestLogBaseResources {
	return RequestLogBaseResources{
		ID:                  data.ID,
		RequestID:           data.RequestID,
		OperatorID:          data.OperatorID,
		IP:                  data.IP,
		RequestKind:         data.RequestKind,
		Method:              data.Method,
		BaseURL:             data.BaseURL,
		OperationName:       data.OperationName,
		OperationStatus:     data.OperationStatus,
		OperationStatusName: getOperationStatusName(data.OperationStatus, language),
		IsHighRisk:          data.IsHighRisk,
		OperatorAccount:     data.OperatorAccount,
		OperatorName:        data.OperatorName,
		ResponseStatus:      data.ResponseStatus,
		ExecutionTimeUs:     data.ExecutionTimeUs,
		ExecutionTimeScope:  data.ExecutionTimeScope,
		CreatedAt:           data.CreatedAt,
	}
}

// ToStruct 将请求日志模型转换为详情响应。
func (r RequestLogTransformer) ToStruct(data *model.RequestLogs) *RequestLogResources {
	base := buildRequestLogBaseResources(data, r.language)
	return &RequestLogResources{
		RequestLogBaseResources: base,
		JwtID:                   data.JwtID,
		UserAgent:               data.UserAgent,
		OS:                      data.OS,
		Browser:                 data.Browser,
		RequestHeaders:          data.RequestHeaders,
		RequestQuery:            data.RequestQuery,
		RequestBody:             data.RequestBody,
		ChangeDiff:              data.ChangeDiff,
		ResponseBody:            data.ResponseBody,
		ResponseHeader:          data.ResponseHeader,
		UpdatedAt:               data.UpdatedAt,
	}
}

// ToSelfDetail 将请求日志模型转换为当前用户可见的本人日志详情。
func (r RequestLogTransformer) ToSelfDetail(data *model.RequestLogs) *RequestLogSelfDetailResources {
	return &RequestLogSelfDetailResources{
		RequestLogBaseResources: buildRequestLogBaseResources(data, r.language),
		UserAgent:               data.UserAgent,
		OS:                      data.OS,
		Browser:                 data.Browser,
		ChangeSummary:           buildRequestLogSelfChangeSummary(data, r.language),
	}
}

const (
	requestLogSelfProfilePath = "/admin/v1/admin-user/update-profile"
	selfChangeTypeCreated     = "created"
	selfChangeTypeUpdated     = "updated"
	selfChangeTypeRemoved     = "removed"
	selfChangeTypeChanged     = "changed"
	selfChangeTypeAction      = "action"
	selfChangeTextMaxRunes    = 80
	selfChangeMaxItems        = 24
)

type requestLogSelfChangeValueMode uint8

const (
	selfChangeValuePlain requestLogSelfChangeValueMode = iota
	selfChangeValuePhone
	selfChangeValueEmail
	selfChangeValueHidden
)

type requestLogSelfChangeRule struct {
	labelZh  string
	labelEn  string
	mode     requestLogSelfChangeValueMode
	valuesZh map[string]string
	valuesEn map[string]string
}

var requestLogSelfProfileChangeRules = map[string]requestLogSelfChangeRule{
	"nickname":         {labelZh: "昵称", labelEn: "Nickname", mode: selfChangeValuePlain},
	"phone_number":     {labelZh: "手机号", labelEn: "Phone number", mode: selfChangeValuePhone},
	"country_code":     {labelZh: "国家区号", labelEn: "Country code", mode: selfChangeValuePlain},
	"email":            {labelZh: "邮箱", labelEn: "Email", mode: selfChangeValueEmail},
	"avatar":           {labelZh: "头像", labelEn: "Avatar", mode: selfChangeValueHidden},
	"password_changed": {labelZh: "密码", labelEn: "Password", mode: selfChangeValueHidden},
}

var requestLogSelfProductChangeRules = map[string]requestLogSelfChangeRule{
	"id":          {labelZh: "产品ID", labelEn: "Product ID", mode: selfChangeValuePlain},
	"name":        {labelZh: "产品名称", labelEn: "Product name", mode: selfChangeValuePlain},
	"description": {labelZh: "产品描述", labelEn: "Product description", mode: selfChangeValuePlain},
	"price":       {labelZh: "价格（分）", labelEn: "Price (cents)", mode: selfChangeValuePlain},
	"status": {
		labelZh:  "状态",
		labelEn:  "Status",
		mode:     selfChangeValuePlain,
		valuesZh: map[string]string{"0": "禁用", "1": "启用"},
		valuesEn: map[string]string{"0": "Disabled", "1": "Enabled"},
	},
	"dept_id":    {labelZh: "归属部门ID", labelEn: "Department ID", mode: selfChangeValuePlain},
	"created_by": {labelZh: "创建者ID", labelEn: "Creator ID", mode: selfChangeValuePlain},
}

var requestLogSelfStatusValuesZh = map[string]string{"0": "禁用", "1": "启用"}
var requestLogSelfStatusValuesEn = map[string]string{"0": "Disabled", "1": "Enabled"}
var requestLogSelfBooleanValuesZh = map[string]string{"0": "否", "1": "是", "false": "否", "true": "是"}
var requestLogSelfBooleanValuesEn = map[string]string{"0": "No", "1": "Yes", "false": "No", "true": "Yes"}

var requestLogSelfAdminUserChangeRules = map[string]requestLogSelfChangeRule{
	"id":               {labelZh: "用户ID", labelEn: "User ID", mode: selfChangeValuePlain},
	"user_id":          {labelZh: "用户ID", labelEn: "User ID", mode: selfChangeValuePlain},
	"username":         {labelZh: "用户名", labelEn: "Username", mode: selfChangeValuePlain},
	"nickname":         {labelZh: "昵称", labelEn: "Nickname", mode: selfChangeValuePlain},
	"phone_number":     {labelZh: "手机号", labelEn: "Phone number", mode: selfChangeValuePhone},
	"country_code":     {labelZh: "国家区号", labelEn: "Country code", mode: selfChangeValuePlain},
	"email":            {labelZh: "邮箱", labelEn: "Email", mode: selfChangeValueEmail},
	"avatar":           {labelZh: "头像", labelEn: "Avatar", mode: selfChangeValueHidden},
	"password_changed": {labelZh: "密码", labelEn: "Password", mode: selfChangeValueHidden},
	"status":           {labelZh: "状态", labelEn: "Status", mode: selfChangeValuePlain, valuesZh: requestLogSelfStatusValuesZh, valuesEn: requestLogSelfStatusValuesEn},
	"is_super_admin":   {labelZh: "超级管理员", labelEn: "Super administrator", mode: selfChangeValuePlain, valuesZh: requestLogSelfBooleanValuesZh, valuesEn: requestLogSelfBooleanValuesEn},
	"dept_ids":         {labelZh: "部门权限", labelEn: "Department access", mode: selfChangeValueHidden},
	"role_ids":         {labelZh: "角色权限", labelEn: "Role access", mode: selfChangeValueHidden},
}

var requestLogSelfRoleChangeRules = map[string]requestLogSelfChangeRule{
	"id":          {labelZh: "角色ID", labelEn: "Role ID", mode: selfChangeValuePlain},
	"code":        {labelZh: "角色编码", labelEn: "Role code", mode: selfChangeValuePlain},
	"name":        {labelZh: "角色名称", labelEn: "Role name", mode: selfChangeValuePlain},
	"description": {labelZh: "描述", labelEn: "Description", mode: selfChangeValuePlain},
	"status":      {labelZh: "状态", labelEn: "Status", mode: selfChangeValuePlain, valuesZh: requestLogSelfStatusValuesZh, valuesEn: requestLogSelfStatusValuesEn},
	"sort":        {labelZh: "排序", labelEn: "Sort", mode: selfChangeValuePlain},
	"menu_list":   {labelZh: "菜单权限", labelEn: "Menu access", mode: selfChangeValueHidden},
}

var requestLogSelfDeptChangeRules = map[string]requestLogSelfChangeRule{
	"id":          {labelZh: "部门ID", labelEn: "Department ID", mode: selfChangeValuePlain},
	"dept_id":     {labelZh: "部门ID", labelEn: "Department ID", mode: selfChangeValuePlain},
	"code":        {labelZh: "部门编码", labelEn: "Department code", mode: selfChangeValuePlain},
	"name":        {labelZh: "部门名称", labelEn: "Department name", mode: selfChangeValuePlain},
	"pid":         {labelZh: "上级部门ID", labelEn: "Parent department ID", mode: selfChangeValuePlain},
	"pids":        {labelZh: "上级路径", labelEn: "Parent path", mode: selfChangeValuePlain},
	"level":       {labelZh: "层级", labelEn: "Level", mode: selfChangeValuePlain},
	"sort":        {labelZh: "排序", labelEn: "Sort", mode: selfChangeValuePlain},
	"description": {labelZh: "描述", labelEn: "Description", mode: selfChangeValuePlain},
	"user_number": {labelZh: "用户数量", labelEn: "User count", mode: selfChangeValuePlain},
	"role_ids":    {labelZh: "角色权限", labelEn: "Role access", mode: selfChangeValueHidden},
}

var requestLogSelfMenuChangeRules = map[string]requestLogSelfChangeRule{
	"id":                {labelZh: "菜单ID", labelEn: "Menu ID", mode: selfChangeValuePlain},
	"icon":              {labelZh: "图标", labelEn: "Icon", mode: selfChangeValuePlain},
	"title_i18n":        {labelZh: "标题", labelEn: "Title", mode: selfChangeValueHidden},
	"code":              {labelZh: "权限标识", labelEn: "Permission code", mode: selfChangeValuePlain},
	"path":              {labelZh: "路由路径", labelEn: "Route path", mode: selfChangeValuePlain},
	"full_path":         {labelZh: "完整路径", labelEn: "Full path", mode: selfChangeValuePlain},
	"name":              {labelZh: "路由名称", labelEn: "Route name", mode: selfChangeValuePlain},
	"component_key":     {labelZh: "组件", labelEn: "Component", mode: selfChangeValuePlain},
	"status":            {labelZh: "状态", labelEn: "Status", mode: selfChangeValuePlain, valuesZh: requestLogSelfStatusValuesZh, valuesEn: requestLogSelfStatusValuesEn},
	"type":              {labelZh: "类型", labelEn: "Type", mode: selfChangeValuePlain, valuesZh: map[string]string{"1": "目录", "2": "菜单", "3": "按钮"}, valuesEn: map[string]string{"1": "Directory", "2": "Menu", "3": "Button"}},
	"is_show":           {labelZh: "显示", labelEn: "Visible", mode: selfChangeValuePlain, valuesZh: requestLogSelfBooleanValuesZh, valuesEn: requestLogSelfBooleanValuesEn},
	"is_auth":           {labelZh: "鉴权", labelEn: "Authorization", mode: selfChangeValuePlain, valuesZh: requestLogSelfBooleanValuesZh, valuesEn: requestLogSelfBooleanValuesEn},
	"is_new_window":     {labelZh: "新窗口", labelEn: "New window", mode: selfChangeValuePlain, valuesZh: requestLogSelfBooleanValuesZh, valuesEn: requestLogSelfBooleanValuesEn},
	"is_external_links": {labelZh: "外链", labelEn: "External link", mode: selfChangeValuePlain, valuesZh: requestLogSelfBooleanValuesZh, valuesEn: requestLogSelfBooleanValuesEn},
	"sort":              {labelZh: "排序", labelEn: "Sort", mode: selfChangeValuePlain},
	"pid":               {labelZh: "上级菜单ID", labelEn: "Parent menu ID", mode: selfChangeValuePlain},
	"pids":              {labelZh: "上级路径", labelEn: "Parent path", mode: selfChangeValuePlain},
	"level":             {labelZh: "层级", labelEn: "Level", mode: selfChangeValuePlain},
	"redirect":          {labelZh: "重定向", labelEn: "Redirect", mode: selfChangeValuePlain},
	"animate_enter":     {labelZh: "进入动画", labelEn: "Enter animation", mode: selfChangeValuePlain},
	"animate_leave":     {labelZh: "离开动画", labelEn: "Leave animation", mode: selfChangeValuePlain},
	"animate_duration":  {labelZh: "动画时长", labelEn: "Animation duration", mode: selfChangeValuePlain},
	"description":       {labelZh: "描述", labelEn: "Description", mode: selfChangeValuePlain},
	"api_list":          {labelZh: "接口权限", labelEn: "API access", mode: selfChangeValueHidden},
}

var requestLogSelfDictTypeChangeRules = map[string]requestLogSelfChangeRule{
	"type_code":      {labelZh: "字典类型编码", labelEn: "Dictionary type code", mode: selfChangeValuePlain},
	"type_name_i18n": {labelZh: "字典类型名称", labelEn: "Dictionary type name", mode: selfChangeValueHidden},
	"status":         {labelZh: "状态", labelEn: "Status", mode: selfChangeValuePlain, valuesZh: requestLogSelfStatusValuesZh, valuesEn: requestLogSelfStatusValuesEn},
	"sort":           {labelZh: "排序", labelEn: "Sort", mode: selfChangeValuePlain},
	"remark":         {labelZh: "备注", labelEn: "Remark", mode: selfChangeValuePlain},
}

var requestLogSelfDictItemChangeRules = map[string]requestLogSelfChangeRule{
	"type_code":  {labelZh: "字典类型编码", labelEn: "Dictionary type code", mode: selfChangeValuePlain},
	"label_i18n": {labelZh: "字典标签", labelEn: "Dictionary label", mode: selfChangeValueHidden},
	"value":      {labelZh: "字典值", labelEn: "Dictionary value", mode: selfChangeValuePlain},
	"color":      {labelZh: "颜色", labelEn: "Color", mode: selfChangeValuePlain},
	"tag_type":   {labelZh: "标签类型", labelEn: "Tag type", mode: selfChangeValuePlain},
	"is_default": {labelZh: "默认项", labelEn: "Default item", mode: selfChangeValuePlain, valuesZh: requestLogSelfBooleanValuesZh, valuesEn: requestLogSelfBooleanValuesEn},
	"status":     {labelZh: "状态", labelEn: "Status", mode: selfChangeValuePlain, valuesZh: requestLogSelfStatusValuesZh, valuesEn: requestLogSelfStatusValuesEn},
	"sort":       {labelZh: "排序", labelEn: "Sort", mode: selfChangeValuePlain},
	"remark":     {labelZh: "备注", labelEn: "Remark", mode: selfChangeValuePlain},
}

var requestLogSelfConfigChangeRules = map[string]requestLogSelfChangeRule{
	"config_key":              {labelZh: "参数键名", labelEn: "Config key", mode: selfChangeValuePlain},
	"config_name_i18n":        {labelZh: "参数名称", labelEn: "Config name", mode: selfChangeValueHidden},
	"config_value":            {labelZh: "参数值", labelEn: "Config value", mode: selfChangeValueHidden},
	"sensitive_value_changed": {labelZh: "敏感参数值", labelEn: "Sensitive config value", mode: selfChangeValueHidden},
	"value_type":              {labelZh: "值类型", labelEn: "Value type", mode: selfChangeValuePlain},
	"group_code":              {labelZh: "参数分组", labelEn: "Config group", mode: selfChangeValuePlain},
	"is_sensitive":            {labelZh: "敏感参数", labelEn: "Sensitive", mode: selfChangeValuePlain, valuesZh: requestLogSelfBooleanValuesZh, valuesEn: requestLogSelfBooleanValuesEn},
	"is_visible":              {labelZh: "系统参数页展示", labelEn: "Visible on config page", mode: selfChangeValuePlain, valuesZh: requestLogSelfBooleanValuesZh, valuesEn: requestLogSelfBooleanValuesEn},
	"manage_tab":              {labelZh: "专属配置Tab", labelEn: "Managed tab", mode: selfChangeValuePlain},
	"status":                  {labelZh: "状态", labelEn: "Status", mode: selfChangeValuePlain, valuesZh: requestLogSelfStatusValuesZh, valuesEn: requestLogSelfStatusValuesEn},
	"sort":                    {labelZh: "排序", labelEn: "Sort", mode: selfChangeValuePlain},
	"remark":                  {labelZh: "备注", labelEn: "Remark", mode: selfChangeValuePlain},
}

var requestLogSelfFileChangeRules = map[string]requestLogSelfChangeRule{
	"origin_name":  {labelZh: "文件名称", labelEn: "File name", mode: selfChangeValuePlain},
	"folder_id":    {labelZh: "文件所在目录ID", labelEn: "File folder ID", mode: selfChangeValuePlain},
	"logical_path": {labelZh: "文件路径", labelEn: "File path", mode: selfChangeValuePlain},
}

var requestLogSelfFolderChangeRules = map[string]requestLogSelfChangeRule{
	"parent_id":    {labelZh: "上级目录ID", labelEn: "Parent folder ID", mode: selfChangeValuePlain},
	"name":         {labelZh: "目录名称", labelEn: "Folder name", mode: selfChangeValuePlain},
	"logical_path": {labelZh: "目录路径", labelEn: "Folder path", mode: selfChangeValuePlain},
}

type requestLogRawChangeItem struct {
	Field         string          `json:"field"`
	Label         string          `json:"label"`
	Before        json.RawMessage `json:"before"`
	After         json.RawMessage `json:"after"`
	BeforeDisplay string          `json:"before_display"`
	AfterDisplay  string          `json:"after_display"`
}

type requestLogSelfFilePolicyKind uint8

const (
	requestLogSelfFileSnapshots requestLogSelfFilePolicyKind = iota
	requestLogSelfFolderSnapshots
	requestLogSelfFileAction
)

type requestLogSelfFileActionKind uint8

const (
	requestLogSelfActionDeleteFile requestLogSelfFileActionKind = iota
	requestLogSelfActionBatchDeleteFiles
	requestLogSelfActionRestoreFile
	requestLogSelfActionBatchRestoreFiles
	requestLogSelfActionDestroyFile
	requestLogSelfActionBatchDestroyFiles
	requestLogSelfActionCreateFolder
	requestLogSelfActionDeleteFolder
)

type requestLogSelfFilePolicy struct {
	action      string
	kind        requestLogSelfFilePolicyKind
	actionKind  requestLogSelfFileActionKind
	nestedFiles bool
}

var requestLogSelfFilePolicies = map[string]requestLogSelfFilePolicy{
	"/admin/v1/system/file/update":              {action: "update_file", kind: requestLogSelfFileSnapshots},
	"/admin/v1/system/file/move":                {action: "move_files", kind: requestLogSelfFileSnapshots, nestedFiles: true},
	"/admin/v1/system/file/folder/update":       {action: "update_file_folder", kind: requestLogSelfFolderSnapshots},
	"/admin/v1/system/file/folder/move":         {action: "move_file_folder", kind: requestLogSelfFolderSnapshots},
	"/admin/v1/system/file/delete":              {action: "delete_file", kind: requestLogSelfFileAction, actionKind: requestLogSelfActionDeleteFile},
	"/admin/v1/system/file/batch-delete":        {action: "batch_delete_files", kind: requestLogSelfFileAction, actionKind: requestLogSelfActionBatchDeleteFiles},
	"/admin/v1/system/file/trash/restore":       {action: "restore_file", kind: requestLogSelfFileAction, actionKind: requestLogSelfActionRestoreFile},
	"/admin/v1/system/file/trash/batch-restore": {action: "batch_restore_files", kind: requestLogSelfFileAction, actionKind: requestLogSelfActionBatchRestoreFiles},
	"/admin/v1/system/file/trash/destroy":       {action: "destroy_file", kind: requestLogSelfFileAction, actionKind: requestLogSelfActionDestroyFile},
	"/admin/v1/system/file/trash/batch-destroy": {action: "batch_destroy_files", kind: requestLogSelfFileAction, actionKind: requestLogSelfActionBatchDestroyFiles},
	"/admin/v1/system/file/folder/create":       {action: "create_file_folder", kind: requestLogSelfFileAction, actionKind: requestLogSelfActionCreateFolder},
	"/admin/v1/system/file/folder/delete":       {action: "delete_file_folder", kind: requestLogSelfFileAction, actionKind: requestLogSelfActionDeleteFolder},
}

type requestLogSelfAuditEnvelope struct {
	Before json.RawMessage `json:"before"`
	After  json.RawMessage `json:"after"`
}

type requestLogSelfAuditSide struct {
	Action string          `json:"action"`
	Value  json.RawMessage `json:"value"`
	Result json.RawMessage `json:"result"`
}

type requestLogSelfFileSnapshot struct {
	ID          *uint   `json:"id"`
	OriginName  *string `json:"origin_name"`
	FolderID    *uint   `json:"folder_id"`
	LogicalPath *string `json:"logical_path"`
}

type requestLogSelfFolderSnapshot struct {
	ID          *uint   `json:"id"`
	ParentID    *uint   `json:"parent_id"`
	Name        *string `json:"name"`
	LogicalPath *string `json:"logical_path"`
}

type requestLogSelfFileActionResult struct {
	ID        uint   `json:"id"`
	IDs       []uint `json:"ids"`
	Total     int    `json:"total"`
	Deleted   int    `json:"deleted"`
	Destroyed int    `json:"destroyed"`
}

func buildRequestLogSelfChangeSummary(data *model.RequestLogs, language string) *RequestLogSelfChangeSummaryResources {
	if data == nil {
		return nil
	}
	raw := strings.TrimSpace(data.ChangeDiff)
	if raw == "" || raw == "[]" || raw == "{}" || raw == "null" {
		return nil
	}

	path := strings.TrimSpace(data.BaseURL)
	if rules := requestLogSelfChangeRulesForPath(path); rules != nil {
		return buildRequestLogSelfFieldChangeSummary(raw, rules, language)
	}
	if policy, ok := requestLogSelfFilePolicies[path]; ok {
		return buildRequestLogSelfFileChangeSummary(raw, policy, language)
	}
	return newRequestLogSelfOmittedSummary()
}

func buildRequestLogSelfFieldChangeSummary(raw string, rules map[string]requestLogSelfChangeRule, language string) *RequestLogSelfChangeSummaryResources {
	var changes []requestLogRawChangeItem
	if err := json.Unmarshal([]byte(raw), &changes); err != nil || len(changes) == 0 {
		return newRequestLogSelfOmittedSummary()
	}
	summary := newRequestLogSelfChangeSummary(len(changes))
	for _, change := range changes {
		rule, allowed := rules[strings.TrimSpace(change.Field)]
		if !allowed {
			summary.OmittedCount++
			continue
		}
		item, ok := buildRequestLogSelfChangeItem(change, rule, language)
		if !ok {
			summary.OmittedCount++
			continue
		}
		appendRequestLogSelfChangeItem(summary, item)
	}
	return summary
}

func buildRequestLogSelfFileChangeSummary(raw string, policy requestLogSelfFilePolicy, language string) *RequestLogSelfChangeSummaryResources {
	var envelope requestLogSelfAuditEnvelope
	if err := json.Unmarshal([]byte(raw), &envelope); err != nil || len(envelope.After) == 0 {
		return newRequestLogSelfOmittedSummary()
	}
	var after requestLogSelfAuditSide
	if err := json.Unmarshal(envelope.After, &after); err != nil || strings.TrimSpace(after.Action) != policy.action {
		return newRequestLogSelfOmittedSummary()
	}
	if policy.kind == requestLogSelfFileAction {
		return buildRequestLogSelfFileActionSummary(after.Result, policy.actionKind, language)
	}

	var before requestLogSelfAuditSide
	if len(envelope.Before) == 0 || bytes.Equal(bytes.TrimSpace(envelope.Before), []byte("null")) ||
		json.Unmarshal(envelope.Before, &before) != nil || strings.TrimSpace(before.Action) != policy.action {
		return newRequestLogSelfOmittedSummary()
	}
	if policy.kind == requestLogSelfFolderSnapshots {
		return buildRequestLogSelfFolderSnapshotSummary(before.Value, after.Value, language)
	}
	return buildRequestLogSelfFileSnapshotSummary(before.Value, after.Value, policy.nestedFiles, language)
}

func buildRequestLogSelfFileSnapshotSummary(beforeRaw, afterRaw json.RawMessage, nestedAfter bool, language string) *RequestLogSelfChangeSummaryResources {
	var before []requestLogSelfFileSnapshot
	if err := json.Unmarshal(beforeRaw, &before); err != nil || len(before) == 0 {
		return newRequestLogSelfOmittedSummary()
	}
	var after []requestLogSelfFileSnapshot
	if nestedAfter {
		var wrapper struct {
			Files []requestLogSelfFileSnapshot `json:"files"`
		}
		if err := json.Unmarshal(afterRaw, &wrapper); err != nil {
			return newRequestLogSelfOmittedSummary()
		}
		after = wrapper.Files
	} else if err := json.Unmarshal(afterRaw, &after); err != nil {
		return newRequestLogSelfOmittedSummary()
	}
	if len(after) == 0 {
		return newRequestLogSelfOmittedSummary()
	}

	summary := newRequestLogSelfChangeSummary(len(before) * len(requestLogSelfFileChangeRules))
	afterByID := make(map[uint]requestLogSelfFileSnapshot, len(after))
	for _, snapshot := range after {
		if !validRequestLogSelfFileSnapshot(snapshot) {
			summary.OmittedCount++
			continue
		}
		if _, duplicate := afterByID[*snapshot.ID]; duplicate {
			summary.OmittedCount++
			continue
		}
		afterByID[*snapshot.ID] = snapshot
	}
	for _, previous := range before {
		if !validRequestLogSelfFileSnapshot(previous) {
			summary.OmittedCount++
			continue
		}
		current, ok := afterByID[*previous.ID]
		if !ok {
			summary.OmittedCount++
			continue
		}
		appendRequestLogSelfPrimitiveChange(summary, "origin_name", *previous.OriginName, *current.OriginName, requestLogSelfFileChangeRules, language)
		appendRequestLogSelfPrimitiveChange(summary, "folder_id", *previous.FolderID, *current.FolderID, requestLogSelfFileChangeRules, language)
		appendRequestLogSelfPrimitiveChange(summary, "logical_path", *previous.LogicalPath, *current.LogicalPath, requestLogSelfFileChangeRules, language)
		delete(afterByID, *previous.ID)
	}
	summary.OmittedCount += len(afterByID)
	return ensureRequestLogSelfSummaryHasResult(summary)
}

func buildRequestLogSelfFolderSnapshotSummary(beforeRaw, afterRaw json.RawMessage, language string) *RequestLogSelfChangeSummaryResources {
	var before requestLogSelfFolderSnapshot
	var after requestLogSelfFolderSnapshot
	if json.Unmarshal(beforeRaw, &before) != nil || json.Unmarshal(afterRaw, &after) != nil ||
		!validRequestLogSelfFolderSnapshot(before) || !validRequestLogSelfFolderSnapshot(after) || *before.ID != *after.ID {
		return newRequestLogSelfOmittedSummary()
	}
	summary := newRequestLogSelfChangeSummary(len(requestLogSelfFolderChangeRules))
	appendRequestLogSelfPrimitiveChange(summary, "parent_id", *before.ParentID, *after.ParentID, requestLogSelfFolderChangeRules, language)
	appendRequestLogSelfPrimitiveChange(summary, "name", *before.Name, *after.Name, requestLogSelfFolderChangeRules, language)
	appendRequestLogSelfPrimitiveChange(summary, "logical_path", *before.LogicalPath, *after.LogicalPath, requestLogSelfFolderChangeRules, language)
	return ensureRequestLogSelfSummaryHasResult(summary)
}

func validRequestLogSelfFileSnapshot(snapshot requestLogSelfFileSnapshot) bool {
	return snapshot.ID != nil && *snapshot.ID > 0 && snapshot.OriginName != nil && snapshot.FolderID != nil && snapshot.LogicalPath != nil
}

func validRequestLogSelfFolderSnapshot(snapshot requestLogSelfFolderSnapshot) bool {
	return snapshot.ID != nil && *snapshot.ID > 0 && snapshot.ParentID != nil && snapshot.Name != nil && snapshot.LogicalPath != nil
}

func appendRequestLogSelfPrimitiveChange(
	summary *RequestLogSelfChangeSummaryResources,
	field string,
	before, after any,
	rules map[string]requestLogSelfChangeRule,
	language string,
) {
	beforeRaw, beforeErr := json.Marshal(before)
	afterRaw, afterErr := json.Marshal(after)
	if beforeErr != nil || afterErr != nil || bytes.Equal(beforeRaw, afterRaw) {
		return
	}
	item, ok := buildRequestLogSelfChangeItem(requestLogRawChangeItem{
		Field:  field,
		Before: beforeRaw,
		After:  afterRaw,
	}, rules[field], language)
	if !ok {
		summary.OmittedCount++
		return
	}
	appendRequestLogSelfChangeItem(summary, item)
}

func ensureRequestLogSelfSummaryHasResult(summary *RequestLogSelfChangeSummaryResources) *RequestLogSelfChangeSummaryResources {
	if len(summary.Items) == 0 && summary.OmittedCount == 0 {
		summary.OmittedCount = 1
	}
	return summary
}

func buildRequestLogSelfFileActionSummary(raw json.RawMessage, kind requestLogSelfFileActionKind, language string) *RequestLogSelfChangeSummaryResources {
	var result requestLogSelfFileActionResult
	if len(raw) == 0 || json.Unmarshal(raw, &result) != nil {
		return newRequestLogSelfOmittedSummary()
	}
	label, ok := requestLogSelfFileActionLabel(result, kind, language)
	if !ok {
		return newRequestLogSelfOmittedSummary()
	}
	summary := newRequestLogSelfChangeSummary(1)
	appendRequestLogSelfChangeItem(summary, RequestLogSelfChangeItemResources{
		Label:      label,
		ChangeType: selfChangeTypeAction,
	})
	return summary
}

func requestLogSelfFileActionLabel(result requestLogSelfFileActionResult, kind requestLogSelfFileActionKind, language string) (string, bool) {
	english := i18n.NormalizeLocale(language) == i18n.LocaleEnUS
	if kind == requestLogSelfActionBatchDeleteFiles || kind == requestLogSelfActionBatchRestoreFiles || kind == requestLogSelfActionBatchDestroyFiles {
		count := result.Deleted
		if kind == requestLogSelfActionBatchRestoreFiles {
			count = len(result.IDs)
		}
		if kind == requestLogSelfActionBatchDestroyFiles {
			count = result.Destroyed
		}
		if count <= 0 || count > 10000 {
			return "", false
		}
		if english {
			switch kind {
			case requestLogSelfActionBatchDeleteFiles:
				return fmt.Sprintf("Deleted %d file(s)", count), true
			case requestLogSelfActionBatchRestoreFiles:
				return fmt.Sprintf("Restored %d file(s)", count), true
			default:
				return fmt.Sprintf("Permanently deleted %d file(s)", count), true
			}
		}
		switch kind {
		case requestLogSelfActionBatchDeleteFiles:
			return fmt.Sprintf("已删除 %d 个文件", count), true
		case requestLogSelfActionBatchRestoreFiles:
			return fmt.Sprintf("已恢复 %d 个文件", count), true
		default:
			return fmt.Sprintf("已永久删除 %d 个文件", count), true
		}
	}
	if result.ID == 0 {
		return "", false
	}
	if english {
		switch kind {
		case requestLogSelfActionDeleteFile:
			return fmt.Sprintf("Deleted file (ID %d)", result.ID), true
		case requestLogSelfActionRestoreFile:
			return fmt.Sprintf("Restored file (ID %d)", result.ID), true
		case requestLogSelfActionDestroyFile:
			return fmt.Sprintf("Permanently deleted file (ID %d)", result.ID), true
		case requestLogSelfActionCreateFolder:
			return fmt.Sprintf("Created folder (ID %d)", result.ID), true
		case requestLogSelfActionDeleteFolder:
			return fmt.Sprintf("Deleted folder (ID %d)", result.ID), true
		}
		return "", false
	}
	switch kind {
	case requestLogSelfActionDeleteFile:
		return fmt.Sprintf("已删除文件（ID %d）", result.ID), true
	case requestLogSelfActionRestoreFile:
		return fmt.Sprintf("已恢复文件（ID %d）", result.ID), true
	case requestLogSelfActionDestroyFile:
		return fmt.Sprintf("已永久删除文件（ID %d）", result.ID), true
	case requestLogSelfActionCreateFolder:
		return fmt.Sprintf("已创建目录（ID %d）", result.ID), true
	case requestLogSelfActionDeleteFolder:
		return fmt.Sprintf("已删除目录（ID %d）", result.ID), true
	default:
		return "", false
	}
}

func newRequestLogSelfChangeSummary(capacity int) *RequestLogSelfChangeSummaryResources {
	if capacity > selfChangeMaxItems {
		capacity = selfChangeMaxItems
	}
	return &RequestLogSelfChangeSummaryResources{
		HasChanges: true,
		Items:      make([]RequestLogSelfChangeItemResources, 0, capacity),
	}
}

func newRequestLogSelfOmittedSummary() *RequestLogSelfChangeSummaryResources {
	return &RequestLogSelfChangeSummaryResources{
		HasChanges:   true,
		Items:        []RequestLogSelfChangeItemResources{},
		OmittedCount: 1,
	}
}

func appendRequestLogSelfChangeItem(summary *RequestLogSelfChangeSummaryResources, item RequestLogSelfChangeItemResources) {
	if len(summary.Items) >= selfChangeMaxItems {
		summary.OmittedCount++
		return
	}
	summary.Items = append(summary.Items, item)
}

func requestLogSelfChangeRulesForPath(path string) map[string]requestLogSelfChangeRule {
	switch strings.TrimSpace(path) {
	case requestLogSelfProfilePath:
		return requestLogSelfProfileChangeRules
	case "/admin/v1/product/create", "/admin/v1/product/update", "/admin/v1/product/delete":
		return requestLogSelfProductChangeRules
	case "/admin/v1/admin-user/create", "/admin/v1/admin-user/update", "/admin/v1/admin-user/delete", "/admin/v1/admin-user/bind-role":
		return requestLogSelfAdminUserChangeRules
	case "/admin/v1/role/create", "/admin/v1/role/update", "/admin/v1/role/delete":
		return requestLogSelfRoleChangeRules
	case "/admin/v1/department/create", "/admin/v1/department/update", "/admin/v1/department/delete", "/admin/v1/department/bind-role":
		return requestLogSelfDeptChangeRules
	case "/admin/v1/menu/create", "/admin/v1/menu/update", "/admin/v1/menu/delete":
		return requestLogSelfMenuChangeRules
	case "/admin/v1/system/dict/type/create", "/admin/v1/system/dict/type/update", "/admin/v1/system/dict/type/delete":
		return requestLogSelfDictTypeChangeRules
	case "/admin/v1/system/dict/item/create", "/admin/v1/system/dict/item/update", "/admin/v1/system/dict/item/delete":
		return requestLogSelfDictItemChangeRules
	case "/admin/v1/system/config/create", "/admin/v1/system/config/update", "/admin/v1/system/config/delete":
		return requestLogSelfConfigChangeRules
	default:
		return nil
	}
}

func buildRequestLogSelfChangeItem(change requestLogRawChangeItem, rule requestLogSelfChangeRule, language string) (RequestLogSelfChangeItemResources, bool) {
	item := RequestLogSelfChangeItemResources{Label: rule.labelZh}
	if i18n.NormalizeLocale(language) == i18n.LocaleEnUS {
		item.Label = rule.labelEn
	}

	hasBefore := len(change.Before) > 0
	hasAfter := len(change.After) > 0
	if !hasBefore && !hasAfter {
		return RequestLogSelfChangeItemResources{}, false
	}
	if rule.mode == selfChangeValueHidden {
		item.ChangeType = selfChangeTypeChanged
		return item, true
	}

	var ok bool
	if hasBefore {
		item.BeforeText, ok = formatRequestLogSelfChangeValue(change.Before, change.BeforeDisplay, rule, language)
		if !ok {
			return RequestLogSelfChangeItemResources{}, false
		}
	}
	if hasAfter {
		item.AfterText, ok = formatRequestLogSelfChangeValue(change.After, change.AfterDisplay, rule, language)
		if !ok {
			return RequestLogSelfChangeItemResources{}, false
		}
	}

	switch {
	case !hasBefore:
		item.ChangeType = selfChangeTypeCreated
	case !hasAfter:
		item.ChangeType = selfChangeTypeRemoved
	default:
		item.ChangeType = selfChangeTypeUpdated
	}
	return item, true
}

func formatRequestLogSelfChangeValue(raw json.RawMessage, display string, rule requestLogSelfChangeRule, language string) (string, bool) {
	if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		if i18n.NormalizeLocale(language) == i18n.LocaleEnUS {
			return "Not set", true
		}
		return "未设置", true
	}

	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return "", false
	}

	var text string
	switch typed := value.(type) {
	case string:
		text = typed
	case json.Number:
		text = typed.String()
	case bool:
		if typed {
			text = "true"
		} else {
			text = "false"
		}
	default:
		return "", false
	}
	if localized := requestLogSelfLocalizedValue(text, display, rule, language); localized != "" {
		return localized, true
	}
	if len(rule.valuesZh) > 0 || len(rule.valuesEn) > 0 {
		return "", false
	}
	text = strings.Join(strings.Fields(text), " ")
	if text == "" {
		if i18n.NormalizeLocale(language) == i18n.LocaleEnUS {
			text = "Not set"
		} else {
			text = "未设置"
		}
	}

	switch rule.mode {
	case selfChangeValuePhone:
		text = utils.NewPhoneRule().Apply(text)
	case selfChangeValueEmail:
		text = utils.NewEmailRule().Apply(text)
	}
	return truncateRequestLogSelfChangeText(text), true
}

func requestLogSelfLocalizedValue(value, display string, rule requestLogSelfChangeRule, language string) string {
	values := rule.valuesZh
	if i18n.NormalizeLocale(language) == i18n.LocaleEnUS {
		values = rule.valuesEn
	}
	localized := strings.TrimSpace(values[strings.TrimSpace(value)])
	if localized == "" {
		return ""
	}
	// auditdiff 的 display 只在与服务端规则一致时复用；伪造或旧版本 display
	// 不进入个人响应，英文则始终使用本地可信映射。
	if i18n.NormalizeLocale(language) != i18n.LocaleEnUS && strings.TrimSpace(display) == localized {
		return truncateRequestLogSelfChangeText(strings.TrimSpace(display))
	}
	return truncateRequestLogSelfChangeText(localized)
}

func truncateRequestLogSelfChangeText(value string) string {
	runes := []rune(value)
	if len(runes) <= selfChangeTextMaxRunes {
		return value
	}
	return string(runes[:selfChangeTextMaxRunes]) + "…"
}

// ToCollection 将请求日志模型集合转换为分页响应。
func (r RequestLogTransformer) ToCollection(page, perPage int, total int64, data []*model.RequestLogs) *Collection {
	response := make([]any, 0, len(data))
	for _, v := range data {
		base := buildRequestLogBaseResources(v, r.language)
		response = append(response, &RequestLogListResources{
			RequestLogBaseResources: base,
		})
	}
	return NewCollection().SetPaginate(page, perPage, total).ToCollection(response)
}

// ToSelfCollection 将本人日志转换为带安全自然语言摘要的分页响应。
func (r RequestLogTransformer) ToSelfCollection(page, perPage int, total int64, data []*model.RequestLogs) *Collection {
	response := make([]any, 0, len(data))
	for _, v := range data {
		response = append(response, &RequestLogSelfListResources{
			RequestLogBaseResources: buildRequestLogBaseResources(v, r.language),
			OS:                      v.OS,
			Browser:                 v.Browser,
			ChangeSummary:           buildRequestLogSelfChangeSummary(v, r.language),
		})
	}
	return NewCollection().SetPaginate(page, perPage, total).ToCollection(response)
}

// getOperationStatusName 将业务码映射为国际化结果名称。
func getOperationStatusName(code int, language string) string {
	key := e.MsgKeyRequestLogOpStatusSuccess
	if code != 0 {
		key = e.MsgKeyRequestLogOpStatusFailure
	}
	if text, ok := e.NewErrorText(language).TextByKey(key); ok {
		return text
	}
	if code == 0 {
		return "成功"
	}
	return "失败"
}
