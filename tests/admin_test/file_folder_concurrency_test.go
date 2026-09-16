package admin_test

import (
	"context"
	"testing"
	"time"

	"github.com/wannanbigpig/gin-layout/internal/model"
	"github.com/wannanbigpig/gin-layout/internal/service"
	"github.com/wannanbigpig/gin-layout/internal/validator/form"
)

func TestConcurrentFolderMovesCannotCreateCycle(t *testing.T) {
	requireWritableDB(t)
	db, err := model.GetDB()
	if err != nil {
		t.Fatal(err)
	}
	prefix := uniqueTestName("folder_cycle")
	a := model.UploadFileFolder{Name: prefix + "_a", LogicalPath: "/" + prefix + "_a"}
	b := model.UploadFileFolder{Name: prefix + "_b", LogicalPath: "/" + prefix + "_b"}
	if err := db.Create(&a).Error; err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := db.Unscoped().Where("id IN ?", []uint{a.ID, b.ID}).Delete(&model.UploadFileFolder{}).Error; err != nil {
			t.Error(err)
		}
	})
	if err := db.Create(&b).Error; err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	start := make(chan struct{})
	results := make(chan error, 2)
	for _, pair := range [][2]uint{{a.ID, b.ID}, {b.ID, a.ID}} {
		go func(source, target uint) {
			<-start
			svc := service.NewFileResourceServiceWithDeps(service.FileResourceServiceDeps{DB: db.WithContext(ctx)})
			_, _, err := svc.MoveFolderWithAudit(&form.FileFolderMove{ID: source, TargetParentID: target}, 1)
			results <- err
		}(pair[0], pair[1])
	}
	close(start)
	succeeded := 0
	for range 2 {
		if err := <-results; err == nil {
			succeeded++
		}
	}
	if succeeded != 1 {
		t.Fatalf("expected exactly one valid move, got %d", succeeded)
	}
	if err := db.First(&a, a.ID).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.First(&b, b.ID).Error; err != nil {
		t.Fatal(err)
	}
	if a.ParentID == b.ID && b.ParentID == a.ID {
		t.Fatal("concurrent moves formed a cycle")
	}
}
