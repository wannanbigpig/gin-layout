package service

import (
	"errors"
	"strings"
	"testing"

	"github.com/wannanbigpig/gin-layout/internal/model"
	e "github.com/wannanbigpig/gin-layout/internal/pkg/errors"
)

func TestBindReferenceRejectsMissingFileAndPreservesQueryErrors(t *testing.T) {
	db := newFileResourceSQLiteDB(t)
	fileURL := "/admin/v1/file/" + strings.Repeat("a", 32)
	err := NewFileReferenceService(db).BindReference(fileURL, "review_test", 1, "attachment")
	var businessErr *e.BusinessError
	if !errors.As(err, &businessErr) || businessErr.GetCode() != e.NotFound {
		t.Fatalf("expected missing file error, got %v", err)
	}
	if err := db.Migrator().DropTable(&model.UploadFiles{}); err != nil {
		t.Fatal(err)
	}
	err = NewFileReferenceService(db).BindReference(fileURL, "review_test", 1, "attachment")
	if err == nil || errors.As(err, &businessErr) {
		t.Fatalf("expected database query failure, got %v", err)
	}
}
