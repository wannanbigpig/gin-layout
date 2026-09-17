package admin_v1

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/wannanbigpig/gin-layout/internal/global"
	"github.com/wannanbigpig/gin-layout/internal/resources"
	"github.com/wannanbigpig/gin-layout/internal/service/dashboard"
	"github.com/wannanbigpig/gin-layout/internal/validator/form"
)

func TestPersonalDashboardControllerUsesCurrentUserID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	fake := &fakePersonalDashboardService{}
	previous := newPersonalDashboardService
	newPersonalDashboardService = func() personalDashboardService { return fake }
	t.Cleanup(func() { newPersonalDashboardService = previous })

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/admin/v1/dashboard/personal", nil)
	ctx.Set(global.ContextKeyUID, uint(9))
	ctx.Set(global.ContextKeyRequestStartTime, time.Now())

	DashboardController{}.Personal(ctx)
	if recorder.Code != http.StatusOK || fake.uid != 9 {
		t.Fatalf("status=%d uid=%d body=%s", recorder.Code, fake.uid, recorder.Body.String())
	}
}

func TestSelfRequestLogControllersUseCurrentUserID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	fake := &fakeSelfRequestLogService{}
	previous := newSelfRequestLogService
	newSelfRequestLogService = func() selfRequestLogService { return fake }
	t.Cleanup(func() { newSelfRequestLogService = previous })

	listRecorder := httptest.NewRecorder()
	listCtx, _ := gin.CreateTestContext(listRecorder)
	listCtx.Request = httptest.NewRequest(http.MethodGet, "/admin/v1/log/request/self-list?page=1&per_page=10&operator_id=77", nil)
	listCtx.Set(global.ContextKeyUID, uint(9))
	listCtx.Set(global.ContextKeyRequestStartTime, time.Now())
	RequestLogController{}.SelfList(listCtx)
	if listRecorder.Code != http.StatusOK || fake.listUID != 9 || fake.requestedOperatorID != 77 {
		t.Fatalf("list status=%d uid=%d requested_operator=%d body=%s", listRecorder.Code, fake.listUID, fake.requestedOperatorID, listRecorder.Body.String())
	}

	detailRecorder := httptest.NewRecorder()
	detailCtx, _ := gin.CreateTestContext(detailRecorder)
	detailCtx.Request = httptest.NewRequest(http.MethodGet, "/admin/v1/log/request/self-detail?id=42", nil)
	detailCtx.Set(global.ContextKeyUID, uint(9))
	detailCtx.Set(global.ContextKeyRequestStartTime, time.Now())
	RequestLogController{}.SelfDetail(detailCtx)
	if detailRecorder.Code != http.StatusOK || fake.detailUID != 9 || fake.detailID != 42 {
		t.Fatalf("detail status=%d uid=%d id=%d body=%s", detailRecorder.Code, fake.detailUID, fake.detailID, detailRecorder.Body.String())
	}
}

type fakePersonalDashboardService struct {
	uid uint
}

func (f *fakePersonalDashboardService) SetAdminUserId(uid uint) {
	f.uid = uid
}

func (f *fakePersonalDashboardService) Personal() (*dashboard.Personal, error) {
	return &dashboard.Personal{}, nil
}

type fakeSelfRequestLogService struct {
	listUID             uint
	detailUID           uint
	detailID            uint
	requestedOperatorID uint
}

func (f *fakeSelfRequestLogService) SetCtx(*gin.Context) {}

func (f *fakeSelfRequestLogService) SelfList(params *form.RequestLogList, uid uint) (*resources.Collection, error) {
	f.listUID = uid
	f.requestedOperatorID = params.OperatorID
	return resources.NewCollection().SetPaginate(params.Page, params.PerPage, 0).ToCollection([]any{}), nil
}

func (f *fakeSelfRequestLogService) SelfDetail(id, uid uint) (any, error) {
	f.detailID = id
	f.detailUID = uid
	return map[string]any{"id": id}, nil
}
