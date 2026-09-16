package service

import (
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"testing"
	"time"

	"github.com/wannanbigpig/gin-layout/internal/global"
	"github.com/wannanbigpig/gin-layout/internal/model"
	"github.com/wannanbigpig/gin-layout/internal/validator/form"
)

func TestStreamFileListExportCSVMatchesListHiddenFilterAndUsesStableCursor(t *testing.T) {
	db := newFileResourceSQLiteDB(t)
	hidden := model.UploadFileFolder{Name: "hidden", LogicalPath: "/hidden", IsHidden: global.Yes}
	if err := db.Create(&hidden).Error; err != nil {
		t.Fatalf("create hidden folder failed: %v", err)
	}
	createdAt := time.Now().Truncate(time.Second)
	rows := make([]model.UploadFiles, 0, fileListExportBatchSize+5)
	for i := 0; i < fileListExportBatchSize+4; i++ {
		rows = append(rows, model.UploadFiles{
			OriginName: fmt.Sprintf("visible-%04d.txt", i),
			UUID:       fmt.Sprintf("%032x", i+1), FileType: "text", MimeType: "text/plain",
			StorageDriver: model.StorageDriverLocal, StorageStatus: model.StorageStatusStored,
		})
		rows[len(rows)-1].CreatedAt.Time = createdAt
	}
	rows = append(rows, model.UploadFiles{
		FolderID: hidden.ID, OriginName: "hidden.txt", UUID: fmt.Sprintf("%032x", len(rows)+1),
		FileType: "text", MimeType: "text/plain", StorageDriver: model.StorageDriverLocal,
		StorageStatus: model.StorageStatusStored,
	})
	rows[len(rows)-1].CreatedAt.Time = createdAt
	if err := db.Create(&rows).Error; err != nil {
		t.Fatalf("create export rows failed: %v", err)
	}

	service := NewFileResourceServiceWithDeps(FileResourceServiceDeps{DB: db})
	var output bytes.Buffer
	params := &form.FileResourceList{Limit: len(rows)}
	if err := service.StreamFileListExportCSV(context.Background(), params, &output, nil); err != nil {
		t.Fatalf("stream export failed: %v", err)
	}
	records, err := csv.NewReader(bytes.NewReader(output.Bytes())).ReadAll()
	if err != nil {
		t.Fatalf("read exported csv failed: %v", err)
	}
	if got, want := len(records), fileListExportBatchSize+5; got != want {
		t.Fatalf("expected header plus %d visible rows, got %d records", want-1, got)
	}

	includeHidden := uint8(global.Yes)
	params.IncludeHidden = &includeHidden
	output.Reset()
	if err := service.StreamFileListExportCSV(context.Background(), params, &output, nil); err != nil {
		t.Fatalf("stream export including hidden failed: %v", err)
	}
	records, err = csv.NewReader(bytes.NewReader(output.Bytes())).ReadAll()
	if err != nil {
		t.Fatalf("read hidden export csv failed: %v", err)
	}
	if got, want := len(records), fileListExportBatchSize+6; got != want {
		t.Fatalf("expected header plus all %d rows, got %d records", want-1, got)
	}
}
