package admin_v1

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/wannanbigpig/gin-layout/internal/global"
	"github.com/wannanbigpig/gin-layout/internal/model"
	"github.com/wannanbigpig/gin-layout/internal/resources"
	"github.com/wannanbigpig/gin-layout/internal/service/todo"
	"github.com/wannanbigpig/gin-layout/internal/validator/form"
)

func TestUserTodoControllerUsesCurrentUserID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	fake := &fakeUserTodoService{}
	previous := newUserTodoService
	newUserTodoService = func() userTodoService { return fake }
	t.Cleanup(func() { newUserTodoService = previous })

	createRecorder := httptest.NewRecorder()
	createCtx, _ := gin.CreateTestContext(createRecorder)
	createCtx.Request = httptest.NewRequest(http.MethodPost, "/admin/v1/todo/create", bytes.NewBufferString(`{"title":"mine","assignee_id":99}`))
	createCtx.Request.Header.Set("Content-Type", "application/json")
	createCtx.Set(global.ContextKeyUID, uint(7))
	createCtx.Set(global.ContextKeyRequestStartTime, time.Now())
	UserTodoController{}.Create(createCtx)
	if createRecorder.Code != http.StatusOK || fake.createUID != 7 || fake.createTitle != "mine" {
		t.Fatalf("create status=%d uid=%d title=%q body=%s", createRecorder.Code, fake.createUID, fake.createTitle, createRecorder.Body.String())
	}

	statusRecorder := httptest.NewRecorder()
	statusCtx, _ := gin.CreateTestContext(statusRecorder)
	statusCtx.Request = httptest.NewRequest(http.MethodPost, "/admin/v1/todo/status", bytes.NewBufferString(`{"id":42,"status":"completed"}`))
	statusCtx.Request.Header.Set("Content-Type", "application/json")
	statusCtx.Set(global.ContextKeyUID, uint(7))
	statusCtx.Set(global.ContextKeyRequestStartTime, time.Now())
	UserTodoController{}.Status(statusCtx)
	if statusRecorder.Code != http.StatusOK || fake.statusUID != 7 || fake.statusID != 42 || fake.status != model.TodoStatusCompleted {
		t.Fatalf("status code=%d uid=%d id=%d status=%q body=%s", statusRecorder.Code, fake.statusUID, fake.statusID, fake.status, statusRecorder.Body.String())
	}

	preferenceRecorder := httptest.NewRecorder()
	preferenceCtx, _ := gin.CreateTestContext(preferenceRecorder)
	preferenceCtx.Request = httptest.NewRequest(http.MethodPost, "/admin/v1/todo/reminder-preference", bytes.NewBufferString(`{"in_app_enabled":false,"sms_enabled":false,"lead_minutes":30}`))
	preferenceCtx.Request.Header.Set("Content-Type", "application/json")
	preferenceCtx.Set(global.ContextKeyUID, uint(7))
	preferenceCtx.Set(global.ContextKeyRequestStartTime, time.Now())
	UserTodoController{}.UpdateReminderPreference(preferenceCtx)
	if preferenceRecorder.Code != http.StatusOK || fake.preferenceUID != 7 || fake.preferenceInApp || fake.preferenceSMS || fake.preferenceLead != 30 {
		t.Fatalf("preference status=%d uid=%d in_app=%v sms=%v body=%s", preferenceRecorder.Code, fake.preferenceUID, fake.preferenceInApp, fake.preferenceSMS, preferenceRecorder.Body.String())
	}
}

func TestUserTodoListIncludesCurrentGroupSummary(t *testing.T) {
	gin.SetMode(gin.TestMode)
	fake := &fakeUserTodoService{}
	previous := newUserTodoService
	newUserTodoService = func() userTodoService { return fake }
	t.Cleanup(func() { newUserTodoService = previous })

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/admin/v1/todo/list?page=1&per_page=10&group_id=7&status=completed", nil)
	c.Set(global.ContextKeyUID, uint(9))
	c.Set(global.ContextKeyRequestStartTime, time.Now())
	UserTodoController{}.List(c)

	var body struct {
		Data struct {
			Total   int64 `json:"total"`
			Summary struct {
				Total   int64 `json:"total"`
				Pending int64 `json:"pending"`
			} `json:"summary"`
		} `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if recorder.Code != http.StatusOK || fake.summaryGroupID == nil || *fake.summaryGroupID != 7 || body.Data.Total != 0 || body.Data.Summary.Total != 2 || body.Data.Summary.Pending != 2 {
		t.Fatalf("list summary mismatch: code=%d group=%v body=%s", recorder.Code, fake.summaryGroupID, recorder.Body.String())
	}
}

func TestUserTodoListStillReturnsRowsWhenSummaryFails(t *testing.T) {
	gin.SetMode(gin.TestMode)
	fake := &fakeUserTodoService{summaryErr: errors.New("summary unavailable")}
	previous := newUserTodoService
	newUserTodoService = func() userTodoService { return fake }
	t.Cleanup(func() { newUserTodoService = previous })

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/admin/v1/todo/list?page=1&per_page=10", nil)
	c.Set(global.ContextKeyUID, uint(9))
	c.Set(global.ContextKeyRequestStartTime, time.Now())
	UserTodoController{}.List(c)

	var body struct {
		Data struct {
			Data    []any                               `json:"data"`
			Summary *resources.UserTodoSummaryResources `json:"summary"`
		} `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if recorder.Code != http.StatusOK || body.Data.Data == nil || body.Data.Summary != nil {
		t.Fatalf("list must survive summary error: %s", recorder.Body.String())
	}
}

type fakeUserTodoService struct {
	summaryGroupID  *uint
	summaryErr      error
	createUID       uint
	createTitle     string
	statusUID       uint
	statusID        uint
	status          string
	preferenceUID   uint
	preferenceInApp bool
	preferenceSMS   bool
	preferenceLead  int
}

func (f *fakeUserTodoService) List(context.Context, uint, *form.UserTodoList) (*resources.Collection, error) {
	return resources.NewCollection().SetPaginate(1, 20, 0).ToCollection([]any{}), nil
}

func (f *fakeUserTodoService) Summary(context.Context, uint) (*resources.UserTodoSummaryResources, error) {
	return &resources.UserTodoSummaryResources{}, nil
}

func (f *fakeUserTodoService) SummaryInGroup(_ context.Context, _ uint, groupID *uint) (*resources.UserTodoSummaryResources, error) {
	f.summaryGroupID = groupID
	if f.summaryErr != nil {
		return nil, f.summaryErr
	}
	return &resources.UserTodoSummaryResources{Total: 2, Pending: 2}, nil
}

func (f *fakeUserTodoService) CreateManual(_ context.Context, uid uint, params *form.UserTodoCreate) (*model.UserTodo, error) {
	f.createUID = uid
	f.createTitle = params.Title
	return &model.UserTodo{AssigneeID: uid, CreatorID: uid, SourceType: model.TodoSourceManual, Title: params.Title}, nil
}

func (f *fakeUserTodoService) UpdateManual(context.Context, uint, *form.UserTodoUpdate) (*model.UserTodo, error) {
	return &model.UserTodo{}, nil
}

func (f *fakeUserTodoService) UpdateStatus(_ context.Context, uid, id uint, status string) (*model.UserTodo, error) {
	f.statusUID = uid
	f.statusID = id
	f.status = status
	return &model.UserTodo{AssigneeID: uid, Status: status}, nil
}

func (f *fakeUserTodoService) DeleteManual(context.Context, uint, uint) error {
	return nil
}

func (f *fakeUserTodoService) GetReminderPreference(context.Context, uint) (*todo.ReminderPreference, error) {
	return &todo.ReminderPreference{InAppEnabled: true}, nil
}

func (f *fakeUserTodoService) ListGroups(context.Context, uint) ([]*model.UserTodoGroup, error) {
	return []*model.UserTodoGroup{}, nil
}

func (f *fakeUserTodoService) CreateGroup(_ context.Context, uid uint, name string) (*model.UserTodoGroup, error) {
	return &model.UserTodoGroup{UserID: uid, Name: name}, nil
}

func (f *fakeUserTodoService) RenameGroup(_ context.Context, uid, id uint, name string) (*model.UserTodoGroup, error) {
	return &model.UserTodoGroup{UserID: uid, Name: name}, nil
}

func (f *fakeUserTodoService) DeleteGroup(context.Context, uint, uint) error { return nil }

func (f *fakeUserTodoService) MoveToGroup(context.Context, uint, uint, uint) (*model.UserTodo, error) {
	return &model.UserTodo{}, nil
}

func (f *fakeUserTodoService) UpdateReminderPreference(_ context.Context, uid uint, inAppEnabled, smsEnabled bool, leadMinutes *int) (*todo.ReminderPreference, error) {
	f.preferenceUID = uid
	f.preferenceInApp = inAppEnabled
	f.preferenceSMS = smsEnabled
	if leadMinutes != nil {
		f.preferenceLead = *leadMinutes
	}
	return &todo.ReminderPreference{InAppEnabled: inAppEnabled, SMSEnabled: smsEnabled}, nil
}
