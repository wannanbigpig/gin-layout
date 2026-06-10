package taskcenter

import (
	"context"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/wannanbigpig/gin-layout/internal/model"
	e "github.com/wannanbigpig/gin-layout/internal/pkg/errors"
	log "github.com/wannanbigpig/gin-layout/internal/pkg/logger"
	"github.com/wannanbigpig/gin-layout/internal/pkg/utils"
	"github.com/wannanbigpig/gin-layout/internal/resources"
	"github.com/wannanbigpig/gin-layout/internal/service"
	"github.com/wannanbigpig/gin-layout/internal/validator/form"
	"go.uber.org/zap"
)

// TaskCenterService 任务中心查询服务。
type TaskCenterService struct {
	service.Base
	db *gorm.DB
}

const (
	taskRunStatsDefaultWindowDays = 7
	taskRunStatsMaxWindowDays     = 31
	taskRunStatsDefaultWindow     = time.Duration(taskRunStatsDefaultWindowDays) * 24 * time.Hour
	taskRunStatsMaxWindow         = time.Duration(taskRunStatsMaxWindowDays) * 24 * time.Hour
)

// TaskCenterServiceDeps 描述 TaskCenterService 可注入依赖。
type TaskCenterServiceDeps struct {
	DB *gorm.DB
}

func NewTaskCenterService() *TaskCenterService {
	return NewTaskCenterServiceWithDeps(TaskCenterServiceDeps{})
}

func NewTaskCenterServiceWithDeps(deps TaskCenterServiceDeps) *TaskCenterService {
	return &TaskCenterService{db: deps.DB}
}

// ListTaskDefinitions 分页查询任务定义列表。
func (s *TaskCenterService) ListTaskDefinitions(params *form.TaskDefinitionList) *resources.Collection {
	query := newListQuery().
		addLike("code", params.Code).
		addLike("name", params.Name).
		addEq("kind", params.Kind).
		addEq("status", params.Status).
		addEq("allow_manual", params.AllowManual).
		addEq("allow_retry", params.AllowRetry).
		addEq("is_high_risk", params.IsHighRisk)
	condition, args := query.Build()

	definitionModel := model.NewTaskDefinition()
	listOptionalParams := model.ListOptionalParams{
		SelectFields: []string{
			"id",
			"code",
			"name",
			"kind",
			"queue",
			"cron_spec",
			"handler",
			"status",
			"allow_manual",
			"allow_retry",
			"is_high_risk",
			"record_success_mode",
			"record_success_rate",
			"record_success_interval_seconds",
			"record_detail_on_manual",
			"record_detail_on_failure",
			"remark",
			"created_at",
			"updated_at",
		},
		OrderBy: "id DESC",
	}

	transformer := resources.NewTaskDefinitionTransformer()
	total, collection, err := model.ListPageE(definitionModel, params.Page, params.PerPage, condition, args, listOptionalParams)
	if err != nil {
		log.Logger.Error("查询任务定义列表失败", zap.Error(err))
		return transformer.ToCollection(params.Page, params.PerPage, 0, nil)
	}
	return transformer.ToCollection(params.Page, params.PerPage, total, collection)
}

// ListTaskRuns 分页查询任务执行记录列表。
func (s *TaskCenterService) ListTaskRuns(params *form.TaskRunList) *resources.Collection {
	query := newListQuery().
		addLike("task_code", params.TaskCode).
		addEq("kind", params.Kind).
		addEq("source", params.Source).
		addLike("source_id", params.SourceID).
		addEq("status", params.Status).
		addEq("detail_record_mode", params.DetailRecordMode).
		addCreatedAtRange(params.StartTime, params.EndTime)
	condition, args := query.Build()

	runModel := model.NewTaskRun()
	listOptionalParams := model.ListOptionalParams{
		SelectFields: []string{
			"id",
			"task_code",
			"kind",
			"source",
			"source_id",
			"queue",
			"status",
			"attempt",
			"max_retry",
			"retry_of_run_id",
			"retry_root_run_id",
			"retry_seq",
			"error_message",
			"duration_ms",
			"started_at",
			"finished_at",
			"created_at",
			"trigger_user_id",
			"trigger_account",
			"detail_record_mode",
		},
		OrderBy: "created_at DESC, id DESC",
	}

	transformer := resources.NewTaskRunTransformer()
	total, collection, err := model.ListPageE(runModel, params.Page, params.PerPage, condition, args, listOptionalParams)
	if err != nil {
		log.Logger.Error("查询任务执行记录列表失败", zap.Error(err))
		return transformer.ToCollection(params.Page, params.PerPage, 0, nil)
	}
	s.decorateTaskRunRetryState(context.Background(), collection)
	return transformer.ToCollection(params.Page, params.PerPage, total, collection)
}

// TaskRunDetail 获取任务执行记录详情。
func (s *TaskCenterService) TaskRunDetail(id uint) (any, error) {
	taskRun := model.NewTaskRun()
	if s.db != nil {
		taskRun.SetDB(s.db)
	}
	if err := taskRun.GetById(id); err != nil || taskRun.ID == 0 {
		return nil, e.NewBusinessError(e.NotFound)
	}
	s.decorateTaskRunRetryState(context.Background(), []*model.TaskRun{taskRun})
	return resources.NewTaskRunTransformer().ToStruct(taskRun), nil
}

// TaskRunEvents 查询任务执行事件列表。
func (s *TaskCenterService) TaskRunEvents(runID uint) ([]*resources.TaskRunEventResources, error) {
	eventModel := model.NewTaskRunEvent()
	if s.db != nil {
		var events []*model.TaskRunEvent
		err := s.db.Select([]string{
			"id",
			"run_id",
			"event_type",
			"message",
			"meta",
			"created_at",
		}).Where("run_id = ?", runID).Order("created_at ASC, id ASC").Find(&events).Error
		if err != nil {
			log.Logger.Error("查询任务执行事件失败", zap.Error(err), zap.Uint("run_id", runID))
			return nil, err
		}
		return buildTaskRunEventResources(events), nil
	}
	listOptionalParams := model.ListOptionalParams{
		SelectFields: []string{"id", "run_id", "event_type", "message", "meta", "created_at"},
		OrderBy:      "created_at ASC, id ASC",
	}

	events, err := model.ListE(eventModel, "run_id = ?", []any{runID}, listOptionalParams)
	if err != nil {
		log.Logger.Error("查询任务执行事件失败", zap.Error(err), zap.Uint("run_id", runID))
		return nil, err
	}
	return buildTaskRunEventResources(events), nil
}

func buildTaskRunEventResources(events []*model.TaskRunEvent) []*resources.TaskRunEventResources {
	transformer := resources.NewTaskRunEventTransformer()
	response := make([]*resources.TaskRunEventResources, 0, len(events))
	for _, item := range events {
		response = append(response, transformer.ToStruct(item))
	}
	return response
}

// TaskRunAuditSnapshot 查询任务执行记录的审计快照（用于构建 change_diff）。
func (s *TaskCenterService) TaskRunAuditSnapshot(id uint) (*TaskRunAuditSnapshot, error) {
	taskRun, err := loadTaskRunByID(id)
	if err != nil || taskRun == nil || taskRun.ID == 0 {
		return nil, e.NewBusinessError(e.NotFound)
	}
	return &TaskRunAuditSnapshot{
		RunID:     taskRun.ID,
		TaskCode:  taskRun.TaskCode,
		Status:    taskRun.Status,
		Queue:     taskRun.Queue,
		SourceID:  taskRun.SourceID,
		Kind:      taskRun.Kind,
		MaxRetry:  taskRun.MaxRetry,
		Attempt:   taskRun.Attempt,
		HasRecord: true,
	}, nil
}

// ListCronTaskStates 分页查询定时任务最近状态列表。
func (s *TaskCenterService) ListCronTaskStates(params *form.CronTaskStateList) *resources.Collection {
	query := newListQuery().
		addLike("task_code", params.TaskCode).
		addEq("last_status", params.LastStatus)
	condition, args := query.Build()

	stateModel := model.NewCronTaskState()
	listOptionalParams := model.ListOptionalParams{
		SelectFields: []string{
			"id",
			"task_code",
			"cron_spec",
			"last_run_id",
			"last_status",
			"last_started_at",
			"last_finished_at",
			"next_run_at",
			"last_success_at",
			"last_failed_at",
			"last_stats_window_start",
			"last_error",
			"updated_at",
		},
		OrderBy: "updated_at DESC, id DESC",
	}

	transformer := resources.NewCronTaskStateTransformer()
	total, collection, err := model.ListPageE(stateModel, params.Page, params.PerPage, condition, args, listOptionalParams)
	if err != nil {
		log.Logger.Error("查询定时任务最近状态列表失败", zap.Error(err))
		return transformer.ToCollection(params.Page, params.PerPage, 0, nil)
	}
	return transformer.ToCollection(params.Page, params.PerPage, total, collection)
}

// TaskRunStats 查询任务执行全量统计汇总。
func (s *TaskCenterService) TaskRunStats(params *form.TaskRunStatsQuery) (*resources.TaskRunStatsResources, error) {
	db, err := s.dbOrDefault()
	if err != nil {
		return nil, err
	}
	statsFilter, err := newStatsFilter(params, time.Now())
	if err != nil {
		return nil, err
	}
	query := statsFilter.Apply(db.Model(&model.TaskRunStat{}))
	type row struct {
		TotalCount          uint64
		SuccessCount        uint64
		FailedCount         uint64
		CanceledCount       uint64
		TimeoutCount        uint64
		InterruptedCount    uint64
		SampledSuccessCount uint64
		DurationTotalMS     float64
		DurationMaxMS       float64
	}
	var result row
	err = query.Select([]string{
		"COALESCE(SUM(total_count), 0) AS total_count",
		"COALESCE(SUM(success_count), 0) AS success_count",
		"COALESCE(SUM(failed_count), 0) AS failed_count",
		"COALESCE(SUM(canceled_count), 0) AS canceled_count",
		"COALESCE(SUM(timeout_count), 0) AS timeout_count",
		"COALESCE(SUM(interrupted_count), 0) AS interrupted_count",
		"COALESCE(SUM(sampled_success_count), 0) AS sampled_success_count",
		"COALESCE(SUM(duration_total_ms), 0) AS duration_total_ms",
		"COALESCE(MAX(duration_max_ms), 0) AS duration_max_ms",
	}).Scan(&result).Error
	if err != nil {
		return nil, err
	}
	avg := float64(0)
	if result.TotalCount > 0 {
		avg = result.DurationTotalMS / float64(result.TotalCount)
	}
	successRate := float64(0)
	if result.TotalCount > 0 {
		successRate = float64(result.SuccessCount) * 100 / float64(result.TotalCount)
	}
	return &resources.TaskRunStatsResources{
		TaskCode:            statsFilter.taskCode(),
		Kind:                statsFilter.kind(),
		Source:              statsFilter.source(),
		TotalCount:          result.TotalCount,
		SuccessCount:        result.SuccessCount,
		FailedCount:         result.FailedCount,
		CanceledCount:       result.CanceledCount,
		TimeoutCount:        result.TimeoutCount,
		InterruptedCount:    result.InterruptedCount,
		SampledSuccessCount: result.SampledSuccessCount,
		DurationTotalMS:     result.DurationTotalMS,
		DurationMaxMS:       result.DurationMaxMS,
		DurationAvgMS:       avg,
		SuccessRate:         successRate,
	}, nil
}

// TaskRunStatsTrend 查询任务执行统计趋势。
func (s *TaskCenterService) TaskRunStatsTrend(params *form.TaskRunStatsQuery) ([]*resources.TaskRunTrendPointResources, error) {
	db, err := s.dbOrDefault()
	if err != nil {
		return nil, err
	}
	statsFilter, err := newStatsFilter(params, time.Now())
	if err != nil {
		return nil, err
	}
	type row struct {
		WindowStart         utils.FormatDate
		WindowSize          string
		TotalCount          uint64
		SuccessCount        uint64
		FailedCount         uint64
		CanceledCount       uint64
		TimeoutCount        uint64
		InterruptedCount    uint64
		SampledSuccessCount uint64
		DurationTotalMS     float64
		DurationMaxMS       float64
	}
	var rows []row
	err = statsFilter.Apply(db.Model(&model.TaskRunStat{})).
		Select([]string{
			"window_start",
			"window_size",
			"COALESCE(SUM(total_count), 0) AS total_count",
			"COALESCE(SUM(success_count), 0) AS success_count",
			"COALESCE(SUM(failed_count), 0) AS failed_count",
			"COALESCE(SUM(canceled_count), 0) AS canceled_count",
			"COALESCE(SUM(timeout_count), 0) AS timeout_count",
			"COALESCE(SUM(interrupted_count), 0) AS interrupted_count",
			"COALESCE(SUM(sampled_success_count), 0) AS sampled_success_count",
			"COALESCE(SUM(duration_total_ms), 0) AS duration_total_ms",
			"COALESCE(MAX(duration_max_ms), 0) AS duration_max_ms",
		}).
		Group("window_start, window_size").
		Order("window_start ASC").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	result := make([]*resources.TaskRunTrendPointResources, 0, len(rows))
	for _, item := range rows {
		result = append(result, &resources.TaskRunTrendPointResources{
			WindowStart:         item.WindowStart,
			WindowSize:          item.WindowSize,
			TotalCount:          item.TotalCount,
			SuccessCount:        item.SuccessCount,
			FailedCount:         item.FailedCount,
			CanceledCount:       item.CanceledCount,
			TimeoutCount:        item.TimeoutCount,
			InterruptedCount:    item.InterruptedCount,
			SampledSuccessCount: item.SampledSuccessCount,
			DurationTotalMS:     item.DurationTotalMS,
			DurationMaxMS:       item.DurationMaxMS,
		})
	}
	return result, nil
}

func (s *TaskCenterService) dbOrDefault() (*gorm.DB, error) {
	if s.db != nil {
		return s.db, nil
	}
	return model.GetDB()
}

func (s *TaskCenterService) decorateTaskRunRetryState(ctx context.Context, runs []*model.TaskRun) {
	if len(runs) == 0 {
		return
	}
	db, err := s.dbOrDefault()
	if err != nil {
		for _, run := range runs {
			if run == nil {
				continue
			}
			run.CanRetry = false
			run.RetryBlockReason = "读取数据库连接失败"
		}
		return
	}

	taskCodes := make([]string, 0, len(runs))
	taskCodeSeen := make(map[string]struct{}, len(runs))
	rootIDs := make([]uint, 0, len(runs))
	rootSeen := make(map[uint]struct{}, len(runs))
	for _, run := range runs {
		if run == nil {
			continue
		}
		if run.TaskCode != "" {
			if _, ok := taskCodeSeen[run.TaskCode]; !ok {
				taskCodeSeen[run.TaskCode] = struct{}{}
				taskCodes = append(taskCodes, run.TaskCode)
			}
		}
		rootID := retryRootRunID(run)
		if rootID > 0 {
			if _, ok := rootSeen[rootID]; !ok {
				rootSeen[rootID] = struct{}{}
				rootIDs = append(rootIDs, rootID)
			}
		}
	}

	definitions := make(map[string]*model.TaskDefinition, len(taskCodes))
	if len(taskCodes) > 0 {
		var rows []*model.TaskDefinition
		if err := db.WithContext(ctx).Where("code IN ? AND deleted_at = 0", taskCodes).Find(&rows).Error; err == nil {
			for _, item := range rows {
				definitions[item.Code] = item
			}
		}
	}
	summaries, err := loadTaskRetryChainSummaries(ctx, db, rootIDs)
	if err != nil {
		log.Logger.Warn("批量读取任务重试链路失败", zap.Error(err))
		for _, run := range runs {
			if run == nil {
				continue
			}
			run.CanRetry = false
			run.RetryBlockReason = "读取重试链路失败"
		}
		return
	}

	for _, run := range runs {
		if run == nil {
			continue
		}
		rootID := retryRootRunID(run)
		canRetry, reason := evaluateTaskRunRetryState(run, definitions[run.TaskCode], summaries[rootID])
		run.CanRetry = canRetry
		run.RetryBlockReason = reason
	}
}

type statsFilter struct {
	params *form.TaskRunStatsQuery
	start  time.Time
	end    time.Time
}

func newStatsFilter(params *form.TaskRunStatsQuery, now time.Time) (*statsFilter, error) {
	start, end, err := normalizeTaskRunStatsRange(params, now)
	if err != nil {
		return nil, err
	}
	return &statsFilter{params: params, start: start, end: end}, nil
}

func (f *statsFilter) Apply(db *gorm.DB) *gorm.DB {
	query := newListQuery().
		addLike("task_code", f.taskCode()).
		addEq("kind", f.kind()).
		addEq("source", f.source())
	query.QueryBuilder.AddCondition("window_start >= ?", f.start)
	query.QueryBuilder.AddCondition("window_start <= ?", f.end)
	condition, args := query.Build()
	if condition != "" {
		return db.Where(condition, args...)
	}
	return db
}

func (f *statsFilter) taskCode() string {
	if f == nil || f.params == nil {
		return ""
	}
	return f.params.TaskCode
}

func (f *statsFilter) kind() string {
	if f == nil || f.params == nil {
		return ""
	}
	return f.params.Kind
}

func (f *statsFilter) source() string {
	if f == nil || f.params == nil {
		return ""
	}
	return f.params.Source
}

func normalizeTaskRunStatsRange(params *form.TaskRunStatsQuery, now time.Time) (time.Time, time.Time, error) {
	if now.IsZero() {
		now = time.Now()
	}

	start, hasStart, err := parseOptionalStatsTime(statsStartTime(params), "开始时间")
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	end, hasEnd, err := parseOptionalStatsTime(statsEndTime(params), "结束时间")
	if err != nil {
		return time.Time{}, time.Time{}, err
	}

	switch {
	case !hasStart && !hasEnd:
		end = now
		start = end.Add(-taskRunStatsDefaultWindow)
	case hasStart && !hasEnd:
		end = now
	case !hasStart && hasEnd:
		start = end.Add(-taskRunStatsDefaultWindow)
	}

	if end.Before(start) {
		return time.Time{}, time.Time{}, e.NewBusinessError(e.InvalidParameter, "任务统计结束时间不能早于开始时间")
	}
	if end.Sub(start) > taskRunStatsMaxWindow {
		return time.Time{}, time.Time{}, e.NewBusinessError(e.InvalidParameter, "任务统计时间范围不能超过31天")
	}
	return start, end, nil
}

func parseOptionalStatsTime(value string, label string) (time.Time, bool, error) {
	if strings.TrimSpace(value) == "" {
		return time.Time{}, false, nil
	}
	parsed, ok := utils.ParseTimeBoundary(value)
	if !ok {
		return time.Time{}, false, e.NewBusinessError(e.InvalidParameter, "任务统计"+label+"格式不正确")
	}
	return parsed, true, nil
}

func statsStartTime(params *form.TaskRunStatsQuery) string {
	if params == nil {
		return ""
	}
	return params.StartTime
}

func statsEndTime(params *form.TaskRunStatsQuery) string {
	if params == nil {
		return ""
	}
	return params.EndTime
}
