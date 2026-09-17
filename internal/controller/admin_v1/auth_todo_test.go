package admin_v1

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/wannanbigpig/gin-layout/internal/global"
	"github.com/wannanbigpig/gin-layout/internal/model"
	"github.com/wannanbigpig/gin-layout/internal/resources"
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
}

type fakeUserTodoService struct {
	createUID   uint
	createTitle string
	statusUID   uint
	statusID    uint
	status      string
}

func (f *fakeUserTodoService) List(context.Context, uint, *form.UserTodoList) (*resources.Collection, error) {
	return resources.NewCollection().SetPaginate(1, 20, 0).ToCollection([]any{}), nil
}

func (f *fakeUserTodoService) Summary(context.Context, uint) (*resources.UserTodoSummaryResources, error) {
	return &resources.UserTodoSummaryResources{}, nil
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
