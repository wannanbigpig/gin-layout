package taskcenter

import (
	"sort"
	"strings"

	"github.com/wannanbigpig/gin-layout/internal/model"
	"github.com/wannanbigpig/gin-layout/internal/pkg/auditdiff"
	"github.com/wannanbigpig/gin-layout/internal/validator/form"
)

var taskRunStatusLabels = map[string]string{
	model.TaskRunStatusPending:     "待执行",
	model.TaskRunStatusRunning:     "执行中",
	model.TaskRunStatusSuccess:     "成功",
	model.TaskRunStatusFailed:      "失败",
	model.TaskRunStatusCanceled:    "已取消",
	model.TaskRunStatusRetrying:    "重试中",
	model.TaskRunStatusTimeout:     "超时",
	model.TaskRunStatusInterrupted: "已中断",
}

var taskActionDiffRules = []auditdiff.FieldRule{
	{Field: "action", Label: "操作类型"},
	{Field: "task_code", Label: "任务编码"},
	{Field: "run_id", Label: "执行记录ID"},
	{Field: "status", Label: "执行状态", ValueLabels: taskRunStatusLabels},
	{Field: "queue", Label: "队列"},
	{Field: "task_id", Label: "任务ID"},
	{Field: "retry_from_run", Label: "重试来源执行记录ID"},
	{Field: "retry_root_run", Label: "重试根执行记录ID"},
	{Field: "retry_seq", Label: "重试序号"},
	{Field: "payload_keys", Label: "Payload字段"},
	{Field: "confirm", Label: "确认信息"},
	{Field: "reason", Label: "操作原因"},
	{Field: "canceled_by", Label: "取消人ID"},
	{Field: "canceled_by_account", Label: "取消人账号"},
	{Field: "cancel_reason", Label: "取消原因"},
}

var taskRecordPolicyDiffRules = []auditdiff.FieldRule{
	{Field: "action", Label: "操作类型"},
	{Field: "task_code", Label: "任务编码"},
	{Field: "record_success_mode", Label: "成功明细策略"},
	{Field: "record_success_rate", Label: "成功采样比例"},
	{Field: "record_success_interval_seconds", Label: "成功记录间隔秒"},
	{Field: "record_detail_on_manual", Label: "手动触发强制明细"},
	{Field: "record_detail_on_failure", Label: "失败强制明细"},
}

var yesNoLabels = map[string]string{
	"0": "否",
	"1": "是",
}

var taskDefinitionStatusLabels = map[string]string{
	"0": "禁用",
	"1": "启用",
}

var taskOperationConfigDiffRules = []auditdiff.FieldRule{
	{Field: "action", Label: "操作类型"},
	{Field: "task_code", Label: "任务编码"},
	{Field: "status", Label: "任务状态", ValueLabels: taskDefinitionStatusLabels},
	{Field: "allow_manual", Label: "允许手动触发", ValueLabels: yesNoLabels},
	{Field: "allow_retry", Label: "允许重试", ValueLabels: yesNoLabels},
}

// TaskRunAuditSnapshot 描述任务执行记录审计快照。
type TaskRunAuditSnapshot struct {
	RunID     uint
	TaskCode  string
	Status    string
	Queue     string
	SourceID  string
	Kind      string
	MaxRetry  int
	Attempt   int
	HasRecord bool
}

func BuildTriggerAuditDiff(params *form.TaskTriggerForm, result map[string]any) string {
	after := map[string]any{
		"action":       "trigger",
		"task_code":    strings.TrimSpace(params.TaskCode),
		"run_id":       result["run_id"],
		"queue":        result["queue"],
		"task_id":      result["task_id"],
		"payload_keys": payloadKeys(params.Payload),
	}
	if strings.TrimSpace(params.Confirm) != "" {
		after["confirm"] = strings.TrimSpace(params.Confirm)
	}
	if strings.TrimSpace(params.Reason) != "" {
		after["reason"] = strings.TrimSpace(params.Reason)
	}
	items := auditdiff.BuildFieldDiff(nil, after, taskActionDiffRules)
	return auditdiff.Marshal(items)
}

func BuildRetryAuditDiff(before *TaskRunAuditSnapshot, result map[string]any) string {
	beforeState := map[string]any{}
	if before != nil && before.HasRecord {
		beforeState["action"] = "retry"
		beforeState["task_code"] = before.TaskCode
		beforeState["run_id"] = before.RunID
		beforeState["status"] = before.Status
		beforeState["queue"] = before.Queue
	}

	after := map[string]any{
		"action":         "retry",
		"task_code":      beforeState["task_code"],
		"run_id":         result["run_id"],
		"status":         model.TaskRunStatusPending,
		"queue":          result["queue"],
		"task_id":        result["task_id"],
		"retry_from_run": result["retry_from_run"],
		"retry_root_run": result["retry_root_run"],
		"retry_seq":      result["retry_seq"],
	}
	items := auditdiff.BuildFieldDiff(beforeState, after, taskActionDiffRules)
	return auditdiff.Marshal(items)
}

func BuildCancelAuditDiff(before *TaskRunAuditSnapshot, result map[string]any) string {
	beforeState := map[string]any{}
	if before != nil && before.HasRecord {
		beforeState["action"] = "cancel"
		beforeState["task_code"] = before.TaskCode
		beforeState["run_id"] = before.RunID
		beforeState["status"] = before.Status
		beforeState["queue"] = before.Queue
		beforeState["task_id"] = before.SourceID
	}

	after := map[string]any{
		"action":    "cancel",
		"task_code": beforeState["task_code"],
		"run_id":    result["run_id"],
		"status":    result["status"],
		"queue":     beforeState["queue"],
		"task_id":   result["task_id"],
	}
	if result["canceled_by"] != nil {
		after["canceled_by"] = result["canceled_by"]
	}
	if result["canceled_by_account"] != nil {
		after["canceled_by_account"] = result["canceled_by_account"]
	}
	if result["cancel_reason"] != nil {
		after["cancel_reason"] = result["cancel_reason"]
	}
	items := auditdiff.BuildFieldDiff(beforeState, after, taskActionDiffRules)
	return auditdiff.Marshal(items)
}

func BuildRecordPolicyAuditDiff(params *form.TaskRecordPolicyForm) string {
	after := map[string]any{
		"action":                          "record_policy",
		"task_code":                       strings.TrimSpace(params.TaskCode),
		"record_success_mode":             strings.TrimSpace(params.RecordSuccessMode),
		"record_success_rate":             params.RecordSuccessRate,
		"record_success_interval_seconds": params.RecordSuccessIntervalSeconds,
		"record_detail_on_manual":         params.RecordDetailOnManual,
		"record_detail_on_failure":        params.RecordDetailOnFailure,
	}
	items := auditdiff.BuildFieldDiff(nil, after, taskRecordPolicyDiffRules)
	return auditdiff.Marshal(items)
}

func BuildOperationConfigAuditDiff(params *form.TaskOperationConfigForm) string {
	after := map[string]any{
		"action":       "operation_config",
		"task_code":    strings.TrimSpace(params.TaskCode),
		"status":       params.Status,
		"allow_manual": params.AllowManual,
		"allow_retry":  params.AllowRetry,
	}
	items := auditdiff.BuildFieldDiff(nil, after, taskOperationConfigDiffRules)
	return auditdiff.Marshal(items)
}

func payloadKeys(payload map[string]any) []string {
	if len(payload) == 0 {
		return []string{}
	}
	keys := make([]string, 0, len(payload))
	for key := range payload {
		trimmed := strings.TrimSpace(key)
		if trimmed == "" {
			continue
		}
		keys = append(keys, trimmed)
	}
	sort.Strings(keys)
	return keys
}
