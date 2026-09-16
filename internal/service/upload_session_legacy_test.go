package service

import (
	"errors"
	"testing"

	"github.com/wannanbigpig/gin-layout/internal/model"
	businesserrors "github.com/wannanbigpig/gin-layout/internal/pkg/errors"
)

func TestPendingUploadSessionRejectsLegacyUnverifiedReuse(t *testing.T) {
	for _, uploadType := range []string{model.UploadSessionTypeDirect, model.UploadSessionTypeMultipart} {
		t.Run(uploadType, func(t *testing.T) {
			db := newFileResourceSQLiteDB(t)
			token, session, err := createUploadSession(db, uploadSessionInput{UploadType: uploadType, OwnerUID: 9, FileObjectID: 123})
			if err != nil {
				t.Fatal(err)
			}
			_, err = findPendingUploadSession(db, token, uploadType, 9)
			var businessErr *businesserrors.BusinessError
			if !errors.As(err, &businessErr) || businessErr.GetCode() != businesserrors.InvalidParameter {
				t.Fatalf("expected rejected legacy session, got %v", err)
			}
			var stored model.UploadSession
			if err := db.First(&stored, session.ID).Error; err != nil {
				t.Fatal(err)
			}
			if stored.Status != model.UploadSessionStatusPending {
				t.Fatalf("legacy session unexpectedly consumed: %s", stored.Status)
			}
		})
	}
}

func TestPendingUploadSessionPreservesDatabaseFailure(t *testing.T) {
	db := newFileResourceSQLiteDB(t)
	if err := db.Migrator().DropTable(&model.UploadSession{}); err != nil {
		t.Fatal(err)
	}
	_, err := findPendingUploadSession(db, "valid-token", model.UploadSessionTypeDirect, 9)
	var businessErr *businesserrors.BusinessError
	if err == nil || errors.As(err, &businessErr) {
		t.Fatalf("expected technical DB failure, got %v", err)
	}
}
