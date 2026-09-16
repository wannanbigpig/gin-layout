package admin_v1

import (
	"bytes"
	"encoding/json"
	"errors"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/wannanbigpig/gin-layout/internal/global"
	"github.com/wannanbigpig/gin-layout/internal/resources"
	"github.com/wannanbigpig/gin-layout/internal/service"
	"github.com/wannanbigpig/gin-layout/internal/validator/form"
)

func TestFileAuditMutationControllersUseTransactionalWrappers(t *testing.T) {
	initControllerAuthTest(t)
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name       string
		path       string
		body       map[string]any
		invoke     func(FileResourceController, *gin.Context)
		wantCalled string
		wantAction string
	}{
		{
			name:       "update file",
			path:       "/admin/v1/system/file/update",
			body:       map[string]any{"id": 11, "origin_name": "new.txt"},
			invoke:     func(controller FileResourceController, ctx *gin.Context) { controller.Update(ctx) },
			wantCalled: "update_file",
			wantAction: "update_file",
		},
		{
			name:       "update folder",
			path:       "/admin/v1/system/file/folder/update",
			body:       map[string]any{"id": 12, "name": "new-folder"},
			invoke:     func(controller FileResourceController, ctx *gin.Context) { controller.FolderUpdate(ctx) },
			wantCalled: "update_folder",
			wantAction: "update_file_folder",
		},
		{
			name:       "move folder",
			path:       "/admin/v1/system/file/folder/move",
			body:       map[string]any{"id": 13, "parent_id": 2},
			invoke:     func(controller FileResourceController, ctx *gin.Context) { controller.FolderMove(ctx) },
			wantCalled: "move_folder",
			wantAction: "move_file_folder",
		},
		{
			name:       "move files",
			path:       "/admin/v1/system/file/move",
			body:       map[string]any{"ids": []uint{14}, "folder_id": 2},
			invoke:     func(controller FileResourceController, ctx *gin.Context) { controller.Move(ctx) },
			wantCalled: "move_files",
			wantAction: "move_files",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fake := &fakeFileAuditMutationService{}
			previous := newFileAuditMutationService
			newFileAuditMutationService = func() fileAuditMutationService { return fake }
			t.Cleanup(func() { newFileAuditMutationService = previous })

			body, err := json.Marshal(test.body)
			if err != nil {
				t.Fatalf("marshal request: %v", err)
			}
			recorder := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(recorder)
			ctx.Request = httptest.NewRequest(http.MethodPost, test.path, bytes.NewReader(body))
			ctx.Request.Header.Set("Content-Type", "application/json")
			ctx.Set(global.ContextKeyUID, uint(9))
			ctx.Set(global.ContextKeyRequestStartTime, time.Now())

			test.invoke(FileResourceController{}, ctx)
			if recorder.Code != http.StatusOK {
				t.Fatalf("expected status 200, got %d: %s", recorder.Code, recorder.Body.String())
			}
			if fake.called != test.wantCalled {
				t.Fatalf("called wrapper = %q, want %q", fake.called, test.wantCalled)
			}
			raw, exists := ctx.Get(global.ContextKeyAuditChangeDiff)
			if !exists || !strings.Contains(raw.(string), `"action":"`+test.wantAction+`"`) {
				t.Fatalf("missing audit action %q in %v", test.wantAction, raw)
			}
		})
	}
}

func TestFileAuditMutationControllerDoesNotWriteDiffOnFailure(t *testing.T) {
	initControllerAuthTest(t)
	gin.SetMode(gin.TestMode)
	fake := &fakeFileAuditMutationService{err: errors.New("snapshot failed")}
	previous := newFileAuditMutationService
	newFileAuditMutationService = func() fileAuditMutationService { return fake }
	t.Cleanup(func() { newFileAuditMutationService = previous })

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/admin/v1/system/file/update", strings.NewReader(`{"id":11,"origin_name":"new.txt"}`))
	ctx.Request.Header.Set("Content-Type", "application/json")
	ctx.Set(global.ContextKeyRequestStartTime, time.Now())

	FileResourceController{}.Update(ctx)
	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d: %s", recorder.Code, recorder.Body.String())
	}
	if _, exists := ctx.Get(global.ContextKeyAuditChangeDiff); exists {
		t.Fatal("failed mutation must not write audit diff")
	}
}

func TestUploadLocalReturns413WhenChunkedMultipartExceedsBodyLimit(t *testing.T) {
	initControllerAuthTest(t)
	gin.SetMode(gin.TestMode)

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("files", "oversized.txt")
	if err != nil {
		t.Fatalf("create multipart file: %v", err)
	}
	if _, err := part.Write(bytes.Repeat([]byte("x"), 1024)); err != nil {
		t.Fatalf("write multipart file: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	request := httptest.NewRequest(http.MethodPost, "/admin/v1/system/file/upload/local", bytes.NewReader(body.Bytes()))
	request.Header.Set("Content-Type", writer.FormDataContentType())
	request.ContentLength = -1
	request.Body = http.MaxBytesReader(recorder, request.Body, 128)
	ctx.Request = request
	ctx.Set(global.ContextKeyRequestStartTime, time.Now())

	FileResourceController{}.UploadLocal(ctx)
	if recorder.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected status 413, got %d: %s", recorder.Code, recorder.Body.String())
	}
}

type fakeFileAuditMutationService struct {
	called string
	err    error
}

func (f *fakeFileAuditMutationService) UpdateFileWithAudit(params *form.FileUpdate) (any, service.FileAuditDiff[[]service.FileAuditSnapshot], error) {
	f.called = "update_file"
	diff := service.FileAuditDiff[[]service.FileAuditSnapshot]{
		Before: []service.FileAuditSnapshot{{ID: params.ID, OriginName: "old.txt"}},
		After:  []service.FileAuditSnapshot{{ID: params.ID, OriginName: params.OriginName}},
	}
	return map[string]any{"id": params.ID}, diff, f.err
}

func (f *fakeFileAuditMutationService) UpdateFolderWithAudit(params *form.FileFolderUpdate, _ uint) (*resources.FileFolderResources, service.FileAuditDiff[*service.FileFolderAuditSnapshot], error) {
	f.called = "update_folder"
	diff := service.FileAuditDiff[*service.FileFolderAuditSnapshot]{
		Before: &service.FileFolderAuditSnapshot{ID: params.ID, Name: "old-folder"},
		After:  &service.FileFolderAuditSnapshot{ID: params.ID, Name: params.Name},
	}
	return &resources.FileFolderResources{ID: params.ID}, diff, f.err
}

func (f *fakeFileAuditMutationService) MoveFolderWithAudit(params *form.FileFolderMove, _ uint) (*resources.FileFolderResources, service.FileAuditDiff[*service.FileFolderAuditSnapshot], error) {
	f.called = "move_folder"
	diff := service.FileAuditDiff[*service.FileFolderAuditSnapshot]{
		Before: &service.FileFolderAuditSnapshot{ID: params.ID, ParentID: 1},
		After:  &service.FileFolderAuditSnapshot{ID: params.ID, ParentID: params.ParentID},
	}
	return &resources.FileFolderResources{ID: params.ID}, diff, f.err
}

func (f *fakeFileAuditMutationService) MoveFilesWithAudit(params *form.FileMove) (*resources.FileMoveResult, service.FileAuditDiff[[]service.FileAuditSnapshot], error) {
	f.called = "move_files"
	diff := service.FileAuditDiff[[]service.FileAuditSnapshot]{
		Before: []service.FileAuditSnapshot{{ID: params.IDs[0], FolderID: 1}},
		After:  []service.FileAuditSnapshot{{ID: params.IDs[0], FolderID: params.FolderID}},
	}
	return &resources.FileMoveResult{Total: 1, Moved: 1}, diff, f.err
}
