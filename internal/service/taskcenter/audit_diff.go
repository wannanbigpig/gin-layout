package taskcenter

import (
	"sort"
	"strings"

	"github.com/wannanbigpig/gin-layout/internal/model"
	"github.com/wannanbigpig/gin-layout/internal/pkg/auditdiff"
	"github.com/wannanbigpig/gin-layout/internal/validator/form"
)

var taskRunStatusLabels = map[string]string{
	model.TaskRunStatusPending:  "待执行",
	model.TaskRunStatusRunning:  "执行中",
	model.TaskRunStatusSuccess:  "成功",
	model.TaskRunStatusFailed:   "失败",
	model.TaskRunStatusCanceled: "已取消",
	model.TaskRunStatusRetrying: "重试中",
}

var taskActionDiffRules = []auditdiff.FieldRule{
	{Field: "action", Label: "操作类型"},
	{Field: "task_code", Label: "任务编码"},
	{Field: "run_id", Label: "执行记录ID"},
	{Field: "status", Label: "执行状态", ValueLabels: taskRunStatusLabels},
	{Field: "queue", Label: "队列"},
	{Field: "task_id", Label: "任务ID"},
	{Field: "retry_from_run", Label: "重试来源执行记录ID"},
	{Field: "payload_keys", Label: "Payload字段"},
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
	items := auditdiff.BuildFieldDiff(beforeState, after, taskActionDiffRules)
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
