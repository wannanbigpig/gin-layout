package model

import (
	"sync"
	"testing"

	"gorm.io/gorm/schema"
)

func TestUploadFilesETagColumnName(t *testing.T) {
	parsed, err := schema.Parse(&UploadFiles{}, &sync.Map{}, schema.NamingStrategy{})
	if err != nil {
		t.Fatalf("parse upload files schema: %v", err)
	}
	field := parsed.LookUpField("ETag")
	if field == nil {
		t.Fatal("expected ETag field to exist")
	}
	if field.DBName != "etag" {
		t.Fatalf("expected ETag DB column to be etag, got %s", field.DBName)
	}
}
