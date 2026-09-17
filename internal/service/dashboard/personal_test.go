package dashboard

import (
	"testing"
	"time"
)

func TestPersonalReturnsOnlyCurrentUserLoginAndActivities(t *testing.T) {
	db := newDashboardTestDB(t)
	if err := db.Exec(`CREATE TABLE admin_user (
		id integer primary key,
		last_login datetime,
		last_ip text,
		deleted_at integer default 0
	)`).Error; err != nil {
		t.Fatalf("create admin_user: %v", err)
	}
	now := time.Now().Truncate(time.Second)
	if err := db.Exec(`INSERT INTO admin_user (id, last_login, last_ip, deleted_at) VALUES (?, ?, ?, 0), (?, ?, ?, 0)`,
		7, now, "10.0.0.7", 8, now.Add(-time.Hour), "10.0.0.8").Error; err != nil {
		t.Fatalf("insert admin users: %v", err)
	}
	if err := db.Exec(`INSERT INTO request_logs
		(created_at, updated_at, request_id, operator_id, operator_account, operation_name, method, base_url, request_kind, execution_time_scope, response_status, operation_status, is_high_risk, change_diff, execution_time_us)
		VALUES (?, ?, 'owned', 7, 'user7', 'Owned Action', 'POST', '/owned', 'http', 'request', 200, 0, 0, '', 10),
		       (?, ?, 'other', 8, 'user8', 'Other Action', 'POST', '/other', 'http', 'request', 200, 0, 0, '', 10)`,
		now, now, now, now).Error; err != nil {
		t.Fatalf("insert activities: %v", err)
	}
	if err := db.Exec(`INSERT INTO request_logs
		(created_at, updated_at, request_id, operator_id, operator_account, operation_name, method, base_url, request_kind, execution_time_scope, response_status, operation_status, is_high_risk, change_diff, execution_time_us)
		VALUES (?, ?, 'failed', 7, 'user7', 'Failed Action', 'GET', '/failed', 'http', 'request', 400, 400, 0, '', 10),
		       (?, ?, 'changed', 7, 'user7', 'Changed Action', 'GET', '/changed', 'http', 'request', 200, 0, 0, '[{"field":"name"}]', 10),
		       (?, ?, 'self-read', 7, 'user7', '个人请求日志列表', 'GET', '/admin/v1/log/request/self-list', 'http', 'request', 500, 500, 1, '[{"field":"name"}]', 10),
		       (?, ?, 'yesterday', 7, 'user7', 'Old Action', 'POST', '/old', 'http', 'request', 200, 0, 0, '', 10)`,
		now, now, now, now, now, now, now.AddDate(0, 0, -1), now.AddDate(0, 0, -1)).Error; err != nil {
		t.Fatalf("insert summary logs: %v", err)
	}
	if err := db.Exec(`INSERT INTO sys_messages (user_id, is_read, created_at) VALUES (7, 0, ?), (7, 0, ?), (7, 1, ?), (8, 0, ?)`, now, now, now, now).Error; err != nil {
		t.Fatalf("insert messages: %v", err)
	}

	svc := &OverviewService{db: db}
	svc.SetAdminUserId(7)
	result, err := svc.Personal()
	if err != nil {
		t.Fatalf("personal dashboard: %v", err)
	}
	if result.UserLogin.LastIP != "10.0.0.7" {
		t.Fatalf("unexpected login: %#v", result.UserLogin)
	}
	if len(result.Activities) != 4 {
		t.Fatalf("unexpected activities: %#v", result.Activities)
	}
	for _, activity := range result.Activities {
		if activity.OperatorID != 7 || activity.RequestID == "self-read" || activity.RequestID == "other" {
			t.Fatalf("personal activities leaked excluded activity: %#v", activity)
		}
	}
	if result.Summary.TodayOperations != 3 || result.Summary.TodayFailures != 1 || result.Summary.TodayChanges != 1 || result.Summary.UnreadNotifications != 2 {
		t.Fatalf("unexpected personal summary: %#v", result.Summary)
	}
}

func TestPersonalReturnsSummaryQueryErrors(t *testing.T) {
	db := newDashboardTestDB(t)
	if err := db.Exec(`CREATE TABLE admin_user (id integer primary key, last_login datetime, last_ip text, deleted_at integer default 0)`).Error; err != nil {
		t.Fatalf("create admin_user: %v", err)
	}
	if err := db.Exec(`INSERT INTO admin_user (id, deleted_at) VALUES (7, 0)`).Error; err != nil {
		t.Fatalf("insert admin_user: %v", err)
	}
	if err := db.Exec(`DROP TABLE sys_messages`).Error; err != nil {
		t.Fatalf("drop sys_messages: %v", err)
	}
	svc := &OverviewService{db: db}
	svc.SetAdminUserId(7)
	if _, err := svc.Personal(); err == nil {
		t.Fatal("summary query errors must not be reported as zero counts")
	}
}
