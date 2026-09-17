package audit

import (
	"encoding/json"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/wannanbigpig/gin-layout/internal/global"
	"github.com/wannanbigpig/gin-layout/internal/model"
	e "github.com/wannanbigpig/gin-layout/internal/pkg/errors"
	"github.com/wannanbigpig/gin-layout/internal/pkg/i18n"
	"github.com/wannanbigpig/gin-layout/internal/resources"
	"github.com/wannanbigpig/gin-layout/internal/validator/form"
)

func newRequestLogSelfTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.Exec(`CREATE TABLE request_logs (
		id integer primary key autoincrement,
		created_at datetime,
		updated_at datetime,
		request_id text,
		jwt_id text,
		operator_id integer,
		ip text,
		user_agent text,
		os text,
		browser text,
		request_kind text,
		method text,
		base_url text,
		operation_name text,
		operation_status integer,
		is_high_risk integer,
		operator_account text,
		operator_name text,
		request_headers text,
		request_query text,
		request_body text,
		change_diff text,
		response_status integer,
		response_body text,
		response_header text,
		execution_time_us integer,
		execution_time_scope text
	)`).Error; err != nil {
		t.Fatalf("create request logs: %v", err)
	}
	return db
}

func insertSelfRequestLog(t *testing.T, db *gorm.DB, operatorID uint, requestID string) *model.RequestLogs {
	t.Helper()
	item := model.NewRequestLogs()
	item.OperatorID = operatorID
	item.RequestID = requestID
	item.Method = "GET"
	item.BaseURL = "/admin/v1/example"
	item.RequestKind = model.RequestKindHTTP
	item.ExecutionTimeScope = model.ExecutionTimeScopeRequest
	item.CreatedAt.Time = time.Now()
	item.UpdatedAt.Time = item.CreatedAt.Time
	item.SetDB(db)
	if err := item.Create(); err != nil {
		t.Fatalf("insert request log: %v", err)
	}
	return item
}

func TestSelfListForcesCurrentOperatorID(t *testing.T) {
	db := newRequestLogSelfTestDB(t)
	owned := insertSelfRequestLog(t, db, 7, "owned")
	if err := db.Model(&model.RequestLogs{}).Where("id = ?", owned.ID).Updates(map[string]any{
		"base_url":    "/admin/v1/product/update",
		"change_diff": `[{"field":"name","before":"旧名称","after":"新名称"}]`,
		"os":          "macOS",
		"browser":     "Chrome",
	}).Error; err != nil {
		t.Fatalf("seed self summary: %v", err)
	}
	insertSelfRequestLog(t, db, 8, "other")

	svc := &RequestLogService{db: db}
	params := form.NewRequestLogListQuery()
	params.Page = 1
	params.PerPage = 10
	params.OperatorID = 8
	result, err := svc.SelfList(params, 7)
	if err != nil {
		t.Fatalf("self list: %v", err)
	}
	if result.Total != 1 || len(result.Data) != 1 {
		t.Fatalf("unexpected self list: total=%d data=%#v", result.Total, result.Data)
	}
	item, ok := result.Data[0].(*resources.RequestLogSelfListResources)
	if !ok || item.ID != owned.ID || item.OperatorID != 7 {
		t.Fatalf("unexpected self list item: %#v", result.Data[0])
	}
	if item.OS != "macOS" || item.Browser != "Chrome" {
		t.Fatalf("self list should expose safe terminal fields: %#v", item)
	}
	if item.ChangeSummary == nil || len(item.ChangeSummary.Items) != 1 || item.ChangeSummary.Items[0].AfterText != "新名称" {
		t.Fatalf("self list should expose safe natural-language data, got %#v", item.ChangeSummary)
	}
}

func TestSelfListRejectsInvalidTimeRange(t *testing.T) {
	svc := &RequestLogService{db: newRequestLogSelfTestDB(t)}
	for _, params := range []*form.RequestLogList{
		{StartTime: "not-a-time"},
		{StartTime: "2026-09-18 00:00:00", EndTime: "2026-09-17 00:00:00"},
	} {
		params.Page = 1
		params.PerPage = 10
		if _, err := svc.SelfList(params, 7); err == nil {
			t.Fatalf("expected invalid time range to fail: %#v", params)
		}
	}
}

func TestSelfListExcludesSelfReadRoutesByDefaultButAllowsOperationNameFilter(t *testing.T) {
	db := newRequestLogSelfTestDB(t)
	business := insertSelfRequestLog(t, db, 7, "business")
	selfList := insertSelfRequestLog(t, db, 7, "self-list")
	selfDetail := insertSelfRequestLog(t, db, 7, "self-detail")
	for id, path := range map[uint]string{
		selfList.ID:   "/admin/v1/log/request/self-list",
		selfDetail.ID: "/admin/v1/log/request/self-detail",
	} {
		operationName := "个人请求日志列表"
		if path == "/admin/v1/log/request/self-detail" {
			operationName = "个人请求日志详情"
		}
		if err := db.Model(&model.RequestLogs{}).Where("id = ?", id).Updates(map[string]any{
			"base_url":       path,
			"operation_name": operationName,
		}).Error; err != nil {
			t.Fatalf("seed self request log path %q: %v", path, err)
		}
	}

	svc := &RequestLogService{db: db}
	params := form.NewRequestLogListQuery()
	params.Page = 1
	params.PerPage = 10
	params.OperatorID = 7

	selfResult, err := svc.SelfList(params, 7)
	if err != nil {
		t.Fatalf("self list: %v", err)
	}
	if selfResult.Total != 1 || len(selfResult.Data) != 1 {
		t.Fatalf("self list must exclude recursive read routes: total=%d data=%#v", selfResult.Total, selfResult.Data)
	}
	selfItem, ok := selfResult.Data[0].(*resources.RequestLogSelfListResources)
	if !ok || selfItem.ID != business.ID {
		t.Fatalf("unexpected self list item: %#v", selfResult.Data[0])
	}

	globalResult, err := svc.List(params)
	if err != nil {
		t.Fatalf("global list: %v", err)
	}
	if globalResult.Total != 3 || len(globalResult.Data) != 3 {
		t.Fatalf("global list must retain self read routes: total=%d data=%#v", globalResult.Total, globalResult.Data)
	}

	params.OperationName = "个人请求日志"
	filteredResult, err := svc.SelfList(params, 7)
	if err != nil {
		t.Fatalf("filtered self list: %v", err)
	}
	if filteredResult.Total != 2 || len(filteredResult.Data) != 2 {
		t.Fatalf("operation name filter must include self read routes: total=%d data=%#v", filteredResult.Total, filteredResult.Data)
	}
}

func TestSelfListScopesAndFailureSemantics(t *testing.T) {
	db := newRequestLogSelfTestDB(t)
	type seed struct {
		requestID       string
		operatorID      uint
		method          string
		baseURL         string
		operationStatus int
		highRisk        uint8
		changeDiff      string
	}
	seeds := []seed{
		{requestID: "regular", operatorID: 7, method: "GET", baseURL: "/regular"},
		{requestID: "write", operatorID: 7, method: "POST", baseURL: "/write"},
		{requestID: "failed-code", operatorID: 7, method: "GET", baseURL: "/failed", operationStatus: 400},
		{requestID: "high-risk", operatorID: 7, method: "GET", baseURL: "/risk", highRisk: 1},
		{requestID: "changed", operatorID: 7, method: "GET", baseURL: "/changed", changeDiff: `[{"field":"name","before":"a","after":"b"}]`},
		{requestID: "self-read", operatorID: 7, method: "GET", baseURL: "/admin/v1/log/request/self-list"},
		{requestID: "other-user", operatorID: 8, method: "POST", baseURL: "/other"},
	}
	for _, item := range seeds {
		row := insertSelfRequestLog(t, db, item.operatorID, item.requestID)
		if err := db.Model(&model.RequestLogs{}).Where("id = ?", row.ID).Updates(map[string]any{
			"method": item.method, "base_url": item.baseURL, "operation_status": item.operationStatus,
			"is_high_risk": item.highRisk, "change_diff": item.changeDiff,
		}).Error; err != nil {
			t.Fatalf("seed %s: %v", item.requestID, err)
		}
	}

	wantByScope := map[string][]string{
		"":           {"regular", "write", "failed-code", "high-risk", "changed"},
		"regular":    {"regular", "write", "failed-code", "high-risk", "changed"},
		"all":        {"regular", "write", "failed-code", "high-risk", "changed", "self-read"},
		"important":  {"write", "failed-code", "high-risk", "changed"},
		"changes":    {"changed"},
		"failed":     {"failed-code"},
		"high_risk":  {"high-risk"},
		"self_reads": {"self-read"},
	}
	svc := &RequestLogService{db: db}
	for scope, want := range wantByScope {
		t.Run("scope_"+scope, func(t *testing.T) {
			params := form.NewRequestLogListQuery()
			params.Page, params.PerPage, params.Scope = 1, 20, scope
			result, err := svc.SelfList(params, 7)
			if err != nil {
				t.Fatalf("self list: %v", err)
			}
			got := make(map[string]struct{}, len(result.Data))
			for _, raw := range result.Data {
				got[raw.(*resources.RequestLogSelfListResources).RequestID] = struct{}{}
			}
			if int64(len(want)) != result.Total || len(got) != len(want) {
				t.Fatalf("scope %q total/data = %d/%v, want %d/%v", scope, result.Total, got, len(want), want)
			}
			for _, requestID := range want {
				if _, ok := got[requestID]; !ok {
					t.Errorf("scope %q missing %q: %v", scope, requestID, got)
				}
			}
		})
	}

	failureFilter := 1
	params := form.NewRequestLogListQuery()
	params.Page, params.PerPage, params.OperationStatus = 1, 20, &failureFilter
	result, err := svc.SelfList(params, 7)
	if err != nil {
		t.Fatalf("failure filter: %v", err)
	}
	if result.Total != 1 || result.Data[0].(*resources.RequestLogSelfListResources).RequestID != "failed-code" {
		t.Fatalf("failure filter must match any non-zero business code: %#v", result.Data)
	}
}

func TestSelfDetailRequiresMatchingOperatorID(t *testing.T) {
	db := newRequestLogSelfTestDB(t)
	other := insertSelfRequestLog(t, db, 8, "other")
	if err := db.Model(&model.RequestLogs{}).Where("id = ?", other.ID).Updates(map[string]any{
		"jwt_id":          "sensitive-jwt",
		"user_agent":      "self-agent",
		"os":              "self-os",
		"browser":         "self-browser",
		"request_headers": `{"authorization":"secret"}`,
		"request_query":   "secret_query=1",
		"request_body":    `{"secret":"request"}`,
		"change_diff":     `{"secret":"diff"}`,
		"response_body":   `{"secret":"response"}`,
		"response_header": `{"x-secret":"response"}`,
	}).Error; err != nil {
		t.Fatalf("seed sensitive request log fields: %v", err)
	}
	svc := &RequestLogService{db: db}

	if _, err := svc.SelfDetail(other.ID, 7); err == nil {
		t.Fatal("expected other user's request log to be hidden")
	} else {
		var businessErr *e.BusinessError
		if !errors.As(err, &businessErr) || businessErr.GetCode() != e.NotFound {
			t.Fatalf("expected NotFound, got %v", err)
		}
	}

	detail, err := svc.SelfDetail(other.ID, 8)
	if err != nil {
		t.Fatalf("self detail: %v", err)
	}
	item, ok := detail.(*resources.RequestLogSelfDetailResources)
	if !ok || item.ID != other.ID || item.OperatorID != 8 || item.UserAgent != "self-agent" || item.OS != "self-os" || item.Browser != "self-browser" {
		t.Fatalf("unexpected detail: %#v", detail)
	}
	serialized, err := json.Marshal(item)
	if err != nil {
		t.Fatalf("marshal self detail: %v", err)
	}
	for _, forbidden := range []string{
		`"jwt_id"`, `"request_headers"`, `"request_query"`, `"request_body"`,
		`"change_diff"`, `"response_body"`, `"response_header"`,
		"sensitive-jwt", "secret_query", "authorization", `"secret"`,
	} {
		if strings.Contains(string(serialized), forbidden) {
			t.Fatalf("self detail leaked %q: %s", forbidden, serialized)
		}
	}
}

func TestSelfDetailReturnsAllowlistedLocalizedChangeSummary(t *testing.T) {
	db := newRequestLogSelfTestDB(t)
	owned := insertSelfRequestLog(t, db, 8, "owned-profile-update")
	changeDiff := `[
		{"field":"nickname","label":"untrusted label","before":"Old Name","after":"New Name"},
		{"field":"phone_number","before":"13800138000","after":"13900139000"},
		{"field":"email","before":"old@example.com","after":"new@example.com"},
		{"field":"avatar","before":"private-old-key","after":"private-new-key"},
		{"field":"password_changed","before":false,"after":true},
		{"field":"role_ids","before":[1],"after":[2]},
		{"field":"secret_field","before":"secret-before","after":"secret-after"}
	]`
	if err := db.Model(&model.RequestLogs{}).Where("id = ?", owned.ID).Updates(map[string]any{
		"base_url":    "/admin/v1/admin-user/update-profile",
		"change_diff": changeDiff,
	}).Error; err != nil {
		t.Fatalf("seed self change diff: %v", err)
	}

	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Set(global.ContextKeyLocale, i18n.LocaleEnUS)
	svc := &RequestLogService{db: db}
	svc.SetCtx(ctx)

	detail, err := svc.SelfDetail(owned.ID, 8)
	if err != nil {
		t.Fatalf("self detail: %v", err)
	}
	item, ok := detail.(*resources.RequestLogSelfDetailResources)
	if !ok || item.ChangeSummary == nil {
		t.Fatalf("expected self change summary, got %#v", detail)
	}
	if !item.ChangeSummary.HasChanges || item.ChangeSummary.OmittedCount != 2 || len(item.ChangeSummary.Items) != 5 {
		t.Fatalf("unexpected self change summary: %#v", item.ChangeSummary)
	}

	itemsByLabel := make(map[string]resources.RequestLogSelfChangeItemResources, len(item.ChangeSummary.Items))
	for _, change := range item.ChangeSummary.Items {
		itemsByLabel[change.Label] = change
	}
	if change := itemsByLabel["Nickname"]; change.ChangeType != "updated" || change.BeforeText != "Old Name" || change.AfterText != "New Name" {
		t.Fatalf("unexpected nickname change: %#v", change)
	}
	if change := itemsByLabel["Phone number"]; change.BeforeText != "138****8000" || change.AfterText != "139****9000" {
		t.Fatalf("unexpected phone change: %#v", change)
	}
	if change := itemsByLabel["Email"]; change.BeforeText != "ol***@example.com" || change.AfterText != "ne***@example.com" {
		t.Fatalf("unexpected email change: %#v", change)
	}
	for _, label := range []string{"Avatar", "Password"} {
		change := itemsByLabel[label]
		if change.ChangeType != "changed" || change.BeforeText != "" || change.AfterText != "" {
			t.Fatalf("unexpected hidden %s change: %#v", label, change)
		}
	}

	serialized, err := json.Marshal(item)
	if err != nil {
		t.Fatalf("marshal self detail: %v", err)
	}
	for _, forbidden := range []string{
		`"field"`, `"before"`, `"after"`, `"change_diff"`, "untrusted label",
		"13800138000", "13900139000", "old@example.com", "new@example.com",
		"private-old-key", "private-new-key", "role_ids", "secret_field", "secret-before", "secret-after",
	} {
		if strings.Contains(string(serialized), forbidden) {
			t.Fatalf("self change summary leaked %q: %s", forbidden, serialized)
		}
	}

	ctx.Set(global.ContextKeyLocale, i18n.LocaleZhCN)
	detail, err = svc.SelfDetail(owned.ID, 8)
	if err != nil {
		t.Fatalf("self detail in zh-CN: %v", err)
	}
	zhItem := detail.(*resources.RequestLogSelfDetailResources)
	if zhItem.ChangeSummary == nil || zhItem.ChangeSummary.Items[0].Label != "昵称" {
		t.Fatalf("expected localized zh-CN label, got %#v", zhItem.ChangeSummary)
	}

	adminDetail, err := svc.Detail(owned.ID)
	if err != nil {
		t.Fatalf("admin detail: %v", err)
	}
	adminItem, ok := adminDetail.(*resources.RequestLogResources)
	if !ok || adminItem.ChangeDiff != changeDiff {
		t.Fatalf("admin detail must retain raw change_diff: %#v", adminDetail)
	}
}

func TestSelfDetailOmitsChangeSummaryWhenNoDiff(t *testing.T) {
	db := newRequestLogSelfTestDB(t)
	owned := insertSelfRequestLog(t, db, 8, "owned-without-change")

	detail, err := (&RequestLogService{db: db}).SelfDetail(owned.ID, 8)
	if err != nil {
		t.Fatalf("self detail: %v", err)
	}
	item := detail.(*resources.RequestLogSelfDetailResources)
	if item.ChangeSummary != nil {
		t.Fatalf("expected null change_summary, got %#v", item.ChangeSummary)
	}
}

func TestSelfRequestLogRejectsMissingCurrentUser(t *testing.T) {
	svc := &RequestLogService{db: newRequestLogSelfTestDB(t)}
	params := form.NewRequestLogListQuery()
	params.Page = 1
	params.PerPage = 10

	_, err := svc.SelfList(params, 0)
	var businessErr *e.BusinessError
	if !errors.As(err, &businessErr) || businessErr.GetCode() != e.NotLogin {
		t.Fatalf("expected NotLogin, got %v", err)
	}
}
