package taskcenter

import (
	"hash/fnv"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/wannanbigpig/gin-layout/internal/jobs"
	"github.com/wannanbigpig/gin-layout/internal/model"
	"github.com/wannanbigpig/gin-layout/internal/pkg/utils"
)

const taskRunStatsWindow = time.Hour

type detailDecision struct {
	Keep bool
	Mode string
}

func resolveDetailDecision(tx *gorm.DB, run *model.TaskRun, status string) detailDecision {
	if run == nil {
		return detailDecision{Keep: true, Mode: model.TaskDetailRecordAll}
	}
	if status != model.TaskRunStatusSuccess {
		return detailDecision{Keep: true, Mode: model.TaskDetailRecordForced}
	}

	definition := loadTaskDefinitionForStats(tx, run.TaskCode)
	if shouldForceSuccessDetail(run, definition) {
		return detailDecision{Keep: true, Mode: model.TaskDetailRecordForced}
	}
	if definition == nil {
		return detailDecision{Keep: true, Mode: model.TaskDetailRecordAll}
	}

	switch normalizeRecordSuccessMode(definition.RecordSuccessMode) {
	case model.TaskRecordSuccessNone:
		return detailDecision{Keep: false, Mode: model.TaskDetailRecordNone}
	case model.TaskRecordSuccessSample:
		if shouldSampleSuccess(run, definition.RecordSuccessRate) {
			return detailDecision{Keep: true, Mode: model.TaskDetailRecordSampled}
		}
		return detailDecision{Keep: false, Mode: model.TaskDetailRecordNone}
	case model.TaskRecordSuccessInterval:
		if shouldRecordByInterval(tx, run, definition.RecordSuccessIntervalSeconds) {
			return detailDecision{Keep: true, Mode: model.TaskDetailRecordSampled}
		}
		return detailDecision{Keep: false, Mode: model.TaskDetailRecordNone}
	default:
		return detailDecision{Keep: true, Mode: model.TaskDetailRecordAll}
	}
}

func normalizeRecordSuccessMode(mode string) string {
	switch strings.TrimSpace(mode) {
	case model.TaskRecordSuccessSample, model.TaskRecordSuccessInterval, model.TaskRecordSuccessNone:
		return strings.TrimSpace(mode)
	default:
		return model.TaskRecordSuccessAll
	}
}

func loadTaskDefinitionForStats(tx *gorm.DB, taskCode string) *model.TaskDefinition {
	taskCode = strings.TrimSpace(taskCode)
	if tx == nil || taskCode == "" {
		return nil
	}
	var definition model.TaskDefinition
	if err := tx.Where("code = ? AND deleted_at = 0", taskCode).First(&definition).Error; err != nil {
		return nil
	}
	return &definition
}

func shouldForceSuccessDetail(run *model.TaskRun, definition *model.TaskDefinition) bool {
	if run == nil {
		return false
	}
	if run.RetryOfRunID > 0 || run.RetryRootRunID > 0 {
		return true
	}
	if run.Source == model.TaskSourceManual || run.TriggerUserID > 0 {
		if definition == nil || definition.RecordDetailOnManual != 0 {
			return true
		}
	}
	if definition != nil && definition.IsHighRisk == model.TaskHighRisk {
		return true
	}
	return isExportTaskCode(run.TaskCode)
}

func isExportTaskCode(taskCode string) bool {
	switch strings.TrimSpace(taskCode) {
	case jobs.TypeExportRequestLog, jobs.TypeExportFileList:
		return true
	default:
		return false
	}
}

func shouldSampleSuccess(run *model.TaskRun, rate uint8) bool {
	if rate >= 100 {
		return true
	}
	if rate == 0 || run == nil {
		return false
	}
	h := fnv.New32a()
	_, _ = h.Write([]byte(run.TaskCode))
	_, _ = h.Write([]byte(":"))
	_, _ = h.Write([]byte(run.SourceID))
	_, _ = h.Write([]byte(":"))
	_, _ = h.Write([]byte(time.Now().Format("200601021504")))
	return h.Sum32()%100 < uint32(rate)
}

func shouldRecordByInterval(tx *gorm.DB, run *model.TaskRun, seconds int) bool {
	if tx == nil || run == nil {
		return true
	}
	if seconds <= 0 {
		return true
	}
	cutoff := time.Now().Add(-time.Duration(seconds) * time.Second)
	var count int64
	err := tx.Model(&model.TaskRun{}).
		Where("id <> ? AND task_code = ? AND status = ? AND detail_record_mode IN ? AND finished_at >= ?",
			run.ID,
			run.TaskCode,
			model.TaskRunStatusSuccess,
			[]string{model.TaskDetailRecordAll, model.TaskDetailRecordSampled, model.TaskDetailRecordForced},
			cutoff).
		Count(&count).Error
	return err != nil || count == 0
}

func recordTaskRunStat(tx *gorm.DB, run *model.TaskRun, sampledSuccess bool) (time.Time, error) {
	if tx == nil || run == nil || strings.TrimSpace(run.TaskCode) == "" {
		return time.Time{}, nil
	}

	finishedAt := time.Now()
	if run.FinishedAt != nil && !run.FinishedAt.IsZero() {
		finishedAt = run.FinishedAt.Time
	}
	windowStart := finishedAt.Truncate(taskRunStatsWindow)
	now := utils.FormatDate{Time: time.Now()}

	stat := model.NewTaskRunStat()
	stat.TaskCode = run.TaskCode
	stat.Kind = run.Kind
	stat.Source = run.Source
	stat.WindowStart = utils.FormatDate{Time: windowStart}
	stat.WindowSize = model.TaskRunStatsWindowHour
	stat.TotalCount = 1
	stat.DurationTotalMS = run.DurationMS
	stat.DurationMaxMS = run.DurationMS
	stat.CreatedAt = now
	stat.UpdatedAt = now

	updates := map[string]any{
		"total_count":       gorm.Expr("total_count + 1"),
		"duration_total_ms": gorm.Expr("duration_total_ms + ?", run.DurationMS),
		"duration_max_ms":   gorm.Expr("CASE WHEN duration_max_ms > ? THEN duration_max_ms ELSE ? END", run.DurationMS, run.DurationMS),
		"updated_at":        now,
	}

	switch run.Status {
	case model.TaskRunStatusSuccess:
		stat.SuccessCount = 1
		updates["success_count"] = gorm.Expr("success_count + 1")
		if sampledSuccess {
			stat.SampledSuccessCount = 1
			updates["sampled_success_count"] = gorm.Expr("sampled_success_count + 1")
		}
	case model.TaskRunStatusCanceled:
		stat.CanceledCount = 1
		updates["canceled_count"] = gorm.Expr("canceled_count + 1")
	case model.TaskRunStatusTimeout:
		stat.TimeoutCount = 1
		updates["timeout_count"] = gorm.Expr("timeout_count + 1")
	case model.TaskRunStatusInterrupted:
		stat.InterruptedCount = 1
		updates["interrupted_count"] = gorm.Expr("interrupted_count + 1")
	case model.TaskRunStatusFailed:
		fallthrough
	default:
		stat.FailedCount = 1
		updates["failed_count"] = gorm.Expr("failed_count + 1")
	}

	return windowStart, tx.Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "task_code"},
			{Name: "kind"},
			{Name: "source"},
			{Name: "window_start"},
			{Name: "window_size"},
		},
		DoUpdates: clause.Assignments(updates),
	}).Create(stat).Error
}

func deleteRunDetail(tx *gorm.DB, runID uint) error {
	if tx == nil || runID == 0 {
		return nil
	}
	if err := tx.Where("run_id = ?", runID).Delete(&model.TaskRunEvent{}).Error; err != nil {
		return err
	}
	return tx.Delete(&model.TaskRun{}, runID).Error
}
