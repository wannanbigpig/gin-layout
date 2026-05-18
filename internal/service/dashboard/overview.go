package dashboard

import (
	"math"
	"strconv"
	"time"

	"gorm.io/gorm"

	"github.com/wannanbigpig/gin-layout/internal/model"
	"github.com/wannanbigpig/gin-layout/internal/service"
)

type Metric struct {
	Key     string  `json:"key"`
	Title   string  `json:"title"`
	Value   float64 `json:"value"`
	Suffix  string  `json:"suffix"`
	Compare string  `json:"compare"`
	Change  string  `json:"change"`
	Type    string  `json:"type"`
}

type ActivityItem struct {
	Key   string `json:"key"`
	Title string `json:"title"`
	Desc  string `json:"desc"`
	Time  string `json:"time"`
	Type  string `json:"type"`
}

type UserLogin struct {
	LastLogin string `json:"last_login"`
	LastIP    string `json:"last_ip"`
}

type Overview struct {
	Metrics    []Metric       `json:"metrics"`
	Activities []ActivityItem `json:"activities"`
	UserLogin  UserLogin      `json:"user_login"`
}

type OverviewService struct {
	service.Base
}

func NewOverviewService() *OverviewService {
	return &OverviewService{}
}

func (s *OverviewService) Overview() (*Overview, error) {
	db, err := model.GetDB()
	if err != nil {
		return nil, err
	}

	now := time.Now()
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	tomorrowStart := todayStart.AddDate(0, 0, 1)
	yesterdayStart := todayStart.AddDate(0, 0, -1)

	activeUsers, err := countDistinct(db.Table("admin_login_logs").Where("deleted_at = 0 AND login_status = ? AND uid > 0 AND created_at >= ? AND created_at < ?", model.LoginStatusSuccess, todayStart, tomorrowStart), "uid")
	if err != nil {
		return nil, err
	}
	activeUsersYesterday, err := countDistinct(db.Table("admin_login_logs").Where("deleted_at = 0 AND login_status = ? AND uid > 0 AND created_at >= ? AND created_at < ?", model.LoginStatusSuccess, yesterdayStart, todayStart), "uid")
	if err != nil {
		return nil, err
	}

	requestsToday, err := countRows(db.Table("request_logs").Where("created_at >= ? AND created_at < ?", todayStart, tomorrowStart))
	if err != nil {
		return nil, err
	}
	requestsYesterday, err := countRows(db.Table("request_logs").Where("created_at >= ? AND created_at < ?", yesterdayStart, todayStart))
	if err != nil {
		return nil, err
	}

	errorsToday, err := countRows(db.Table("request_logs").Where("created_at >= ? AND created_at < ? AND (operation_status <> 0 OR response_status >= 400)", todayStart, tomorrowStart))
	if err != nil {
		return nil, err
	}
	errorsYesterday, err := countRows(db.Table("request_logs").Where("created_at >= ? AND created_at < ? AND (operation_status <> 0 OR response_status >= 400)", yesterdayStart, todayStart))
	if err != nil {
		return nil, err
	}

	taskCompletion, err := taskCompletionRate(db, todayStart, tomorrowStart)
	if err != nil {
		return nil, err
	}

	userLogin := UserLogin{}
	if s.GetAdminUserId() > 0 {
		var user model.AdminUser
		if err := db.Table("admin_user").Select("last_login,last_ip").Where("id = ? AND deleted_at = 0", s.GetAdminUserId()).First(&user).Error; err == nil {
			userLogin.LastLogin = user.LastLogin.String()
			userLogin.LastIP = user.LastIp
		}
	}

	return &Overview{
		Metrics: []Metric{
			{Key: "users", Title: "活跃用户", Value: float64(activeUsers), Compare: "较昨日", Change: formatChange(activeUsers, activeUsersYesterday), Type: changeType(activeUsers, activeUsersYesterday)},
			{Key: "requests", Title: "请求总量", Value: float64(requestsToday), Compare: "较昨日", Change: formatChange(requestsToday, requestsYesterday), Type: changeType(requestsToday, requestsYesterday)},
			{Key: "errors", Title: "异常告警", Value: float64(errorsToday), Compare: "较昨日", Change: formatChange(errorsToday, errorsYesterday), Type: inverseChangeType(errorsToday, errorsYesterday)},
			{Key: "tasks", Title: "任务完成率", Value: taskCompletion, Suffix: "%", Compare: "计划完成", Change: "+0.0%", Type: "primary"},
		},
		Activities: buildActivities(db),
		UserLogin:  userLogin,
	}, nil
}

func countRows(db *gorm.DB) (int64, error) {
	var count int64
	if err := db.Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

func countDistinct(db *gorm.DB, field string) (int64, error) {
	var count int64
	if err := db.Select("COUNT(DISTINCT " + field + ")").Scan(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

func taskCompletionRate(db *gorm.DB, start time.Time, end time.Time) (float64, error) {
	total, err := countRows(db.Table("task_runs").Where("created_at >= ? AND created_at < ?", start, end))
	if err != nil {
		return 0, err
	}
	if total == 0 {
		return 100, nil
	}
	success, err := countRows(db.Table("task_runs").Where("created_at >= ? AND created_at < ? AND status = ?", start, end, model.TaskRunStatusSuccess))
	if err != nil {
		return 0, err
	}
	return math.Round(float64(success)*1000/float64(total)) / 10, nil
}

func buildActivities(db *gorm.DB) []ActivityItem {
	type row struct {
		ID              uint
		OperatorAccount string
		OperationName   string
		Method          string
		BaseURL         string
		CreatedAt       time.Time
		OperationStatus int
	}
	var rows []row
	_ = db.Table("request_logs").
		Select("id, operator_account, operation_name, method, base_url, created_at, operation_status").
		Order("created_at DESC, id DESC").
		Limit(4).
		Scan(&rows).Error

	activities := make([]ActivityItem, 0, len(rows))
	for _, item := range rows {
		title := item.OperationName
		if title == "" {
			title = item.Method + " " + item.BaseURL
		}
		activities = append(activities, ActivityItem{
			Key:   strconv.FormatUint(uint64(item.ID), 10),
			Title: title,
			Desc:  item.OperatorAccount,
			Time:  item.CreatedAt.Format("15:04"),
			Type:  activityType(item.OperationStatus),
		})
	}
	return activities
}

func formatChange(current int64, previous int64) string {
	if previous == 0 {
		if current == 0 {
			return "+0.0%"
		}
		return "+100.0%"
	}
	change := (float64(current) - float64(previous)) * 100 / float64(previous)
	prefix := "+"
	if change < 0 {
		prefix = ""
	}
	return prefix + strconvFormatFloat(change) + "%"
}

func strconvFormatFloat(value float64) string {
	return strconv.FormatFloat(math.Round(value*10)/10, 'f', 1, 64)
}

func changeType(current int64, previous int64) string {
	if current >= previous {
		return "success"
	}
	return "warning"
}

func inverseChangeType(current int64, previous int64) string {
	if current <= previous {
		return "success"
	}
	return "danger"
}

func activityType(status int) string {
	if status == 0 {
		return "success"
	}
	return "warning"
}
