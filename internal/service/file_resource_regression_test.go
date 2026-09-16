package service

import (
	"net/http"
	"strings"
	"testing"

	"github.com/wannanbigpig/gin-layout/internal/model"
)

func TestUploadConflictPreservesOwnerAndVisibility(t *testing.T) {
	for _, tc := range []struct {
		name      string
		uid       uint
		public    uint8
		oldPublic uint8
		reuse     bool
	}{
		{"same private owner", 1, 0, 0, true}, {"other private owner", 2, 0, 0, false},
		{"visibility change", 1, 1, 0, false}, {"other public owner", 2, 1, 1, false},
		{"public to other private owner", 2, 0, 1, false}, {"same public owner", 1, 1, 1, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db := newFileResourceSQLiteDB(t)
			old := model.UploadFiles{UID: 1, OriginName: "report.csv", Hash: "hash", UUID: "old", IsPublic: tc.oldPublic}
			if err := db.Create(&old).Error; err != nil {
				t.Fatal(err)
			}
			got, reused, err := NewFileResourceService().createUploadFileWithConflict(db, 0, "report.csv", "hash", tc.uid, tc.public, func(r *uploadFileConflictResult) *model.UploadFiles {
				return &model.UploadFiles{UID: tc.uid, OriginName: r.FinalName, Hash: "hash", UUID: "new", IsPublic: tc.public}
			})
			if err != nil {
				t.Fatal(err)
			}
			if reused != tc.reuse || got.UID != tc.uid || got.IsPublic != tc.public {
				t.Fatalf("reused=%v file=%+v", reused, got)
			}
			if !tc.reuse && (got.ID == old.ID || got.OriginName != "report (1).csv") {
				t.Fatalf("expected independent renamed file: %+v", got)
			}
		})
	}
}

func TestLogicalPathRenameTreatsWildcardsLiterally(t *testing.T) {
	for _, name := range []string{"a_b", "a%b", "a!b"} {
		t.Run(name, func(t *testing.T) {
			db := newFileResourceSQLiteDB(t)
			root := model.UploadFileFolder{Name: name, LogicalPath: "/" + name}
			child := model.UploadFileFolder{Name: "child", LogicalPath: "/" + name + "/child"}
			other := model.UploadFileFolder{Name: "other", LogicalPath: "/acb/child"}
			for _, f := range []*model.UploadFileFolder{&root, &child, &other} {
				if err := db.Create(f).Error; err != nil {
					t.Fatal(err)
				}
			}
			for i, f := range []*model.UploadFileFolder{&child, &other} {
				file := model.UploadFiles{FolderID: f.ID, OriginName: "file", LogicalPath: f.LogicalPath, UUID: strings.Repeat(string(rune('a'+i)), 32)}
				if err := db.Create(&file).Error; err != nil {
					t.Fatal(err)
				}
			}
			if err := updateLogicalPathSnapshots(db, root.ID, root.LogicalPath, "/renamed"); err != nil {
				t.Fatal(err)
			}
			if err := db.First(&child, child.ID).Error; err != nil {
				t.Fatal(err)
			}
			if err := db.First(&other, other.ID).Error; err != nil {
				t.Fatal(err)
			}
			if child.LogicalPath != "/renamed/child" || other.LogicalPath != "/acb/child" {
				t.Fatalf("child=%s other=%s", child.LogicalPath, other.LogicalPath)
			}
			var file model.UploadFiles
			if err := db.Where("folder_id = ?", other.ID).First(&file).Error; err != nil {
				t.Fatal(err)
			}
			if file.LogicalPath != "/acb/child" {
				t.Fatal(file.LogicalPath)
			}
		})
	}
}

func TestCSVSniffAcceptsPlainTextButRejectsHTML(t *testing.T) {
	for _, tc := range []struct {
		body string
		want bool
	}{{"name,count\nalice,1\n", true}, {"<html><script>alert(1)</script></html>", false}, {"\x89PNG\r\n\x1a\n", false}} {
		detected := strings.Split(http.DetectContentType([]byte(tc.body)), ";")[0]
		if got := isMimeCompatible(detected, "text/csv"); got != tc.want {
			t.Fatalf("detected=%s compatible=%v", detected, got)
		}
	}
}
