package admin_v1

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/wannanbigpig/gin-layout/internal/controller"
	"github.com/wannanbigpig/gin-layout/internal/global"
	"github.com/wannanbigpig/gin-layout/internal/model"
	e "github.com/wannanbigpig/gin-layout/internal/pkg/errors"
	"github.com/wannanbigpig/gin-layout/internal/service/taskcenter"
)

type exportPreflightRecorder struct{ enqueued bool }

func (r *exportPreflightRecorder) Enqueue(context.Context, taskcenter.RunStart) (*model.TaskRun, error) {
	r.enqueued = true
	return nil, e.NewBusinessError(e.InvalidParameter)
}

func (r *exportPreflightRecorder) Start(context.Context, taskcenter.RunStart) (*model.TaskRun, error) {
	return nil, nil
}

func (r *exportPreflightRecorder) Finish(context.Context, *model.TaskRun, taskcenter.RunFinish) error {
	return nil
}

func TestSubmitAsyncExportRejectsEmptyBeforeCreatingTask(t *testing.T) {
	recorder := &exportPreflightRecorder{}
	defer taskcenter.SetRecorderForTesting(recorder)()

	response := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(response)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/admin/v1/log/request/export", nil)
	ctx.Set(global.ContextKeyRequestStartTime, time.Now())
	ctx.Set(global.ContextKeyLocale, "zh-CN")

	submitAsyncExport(controller.Api{}, ctx, asyncExportRequest{
		taskCode:  "export:request-log",
		queueName: "export",
		hasRows:   func(context.Context) (bool, error) { return false, nil },
	})

	if recorder.enqueued {
		t.Fatal("empty export must not create a task run")
	}
	if response.Code != http.StatusBadRequest {
		t.Fatalf("expected HTTP 400, got %d", response.Code)
	}
	var result struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Code != e.NoExportData || result.Msg != "暂无需要导出的数据" {
		t.Fatalf("unexpected empty export response: %#v", result)
	}
}
