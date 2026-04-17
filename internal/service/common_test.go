package service

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	c "github.com/wannanbigpig/gin-layout/config"
	"github.com/wannanbigpig/gin-layout/internal/global"
	"github.com/wannanbigpig/gin-layout/internal/model"
	"github.com/wannanbigpig/gin-layout/pkg/utils"
)

func TestVisibilityFlag(t *testing.T) {
	assert.Equal(t, uint8(global.Yes), visibilityFlag(true))
	assert.Equal(t, uint8(global.No), visibilityFlag(false))
}

func TestStorageBasePath(t *testing.T) {
	originBasePath := c.Config.BasePath
	t.Cleanup(func() {
		c.Config.BasePath = originBasePath
	})

	c.Config.BasePath = "/tmp/go-layout"

	assert.Equal(t, filepath.Join("/tmp/go-layout", "storage/public"), storageBasePath(true))
	assert.Equal(t, filepath.Join("/tmp/go-layout", "storage/private"), storageBasePath(false))
}

func TestNormalizeUploadPath(t *testing.T) {
	path, err := normalizeUploadPath("")
	assert.NoError(t, err)
	assert.Equal(t, "default", path)

	path, err = normalizeUploadPath("avatar")
	assert.NoError(t, err)
	assert.Equal(t, "avatar", path)
}

func TestBuildFileURL(t *testing.T) {
	originBaseURL := c.Config.BaseURL
	t.Cleanup(func() {
		c.Config.BaseURL = originBaseURL
	})

	c.Config.BaseURL = "https://example.com/"
	assert.Equal(t, "https://example.com/admin/v1/file/abc123", buildFileURL("abc123"))

	c.Config.BaseURL = ""
	assert.Equal(t, "/admin/v1/file/abc123", buildFileURL("abc123"))
	assert.Equal(t, "", buildFileURL(""))
}

func TestFillFileInfoFromModel(t *testing.T) {
	fileInfo := &utils.FileInfo{OriginName: "origin.png"}
	uploadFile := &model.UploadFiles{
		Name:     "stored.png",
		Path:     "avatar/stored.png",
		Size:     12,
		Ext:      ".png",
		Hash:     "hash",
		UUID:     "uuid123",
		MimeType: "image/png",
	}

	fillFileInfoFromModel(fileInfo, uploadFile)

	assert.Equal(t, "stored.png", fileInfo.Name)
	assert.Equal(t, "avatar/stored.png", fileInfo.Path)
	assert.Equal(t, int64(12), fileInfo.Size)
	assert.Equal(t, ".png", fileInfo.Ext)
	assert.Equal(t, "hash", fileInfo.Sha256)
	assert.Equal(t, "uuid123", fileInfo.UUID)
	assert.Equal(t, "image/png", fileInfo.MimeType)
	assert.Equal(t, global.SUCCESS, fileInfo.Status)
}

func TestFillFileInfoFromUploadResult(t *testing.T) {
	fileInfo := &utils.FileInfo{OriginName: "origin.png"}
	result := &utils.FileInfo{
		Name:     "stored.png",
		Path:     "avatar/stored.png",
		Size:     12,
		Ext:      ".png",
		Sha256:   "hash",
		UUID:     "uuid123",
		MimeType: "image/png",
	}

	fillFileInfoFromUploadResult(fileInfo, result)

	assert.Equal(t, "stored.png", fileInfo.Name)
	assert.Equal(t, "avatar/stored.png", fileInfo.Path)
	assert.Equal(t, int64(12), fileInfo.Size)
	assert.Equal(t, ".png", fileInfo.Ext)
	assert.Equal(t, "hash", fileInfo.Sha256)
	assert.Equal(t, "uuid123", fileInfo.UUID)
	assert.Equal(t, "image/png", fileInfo.MimeType)
	assert.Equal(t, global.SUCCESS, fileInfo.Status)
}

func TestSummarizeImageUploadResults(t *testing.T) {
	filesInfo := []*utils.FileInfo{
		{Status: global.SUCCESS},
		{Status: global.ERROR},
	}

	result, err := summarizeImageUploadResults(filesInfo)
	assert.Len(t, result, 2)
	assert.Error(t, err)
	assert.True(t, IsPartialImageUploadError(err))
}

func TestSummarizeImageUploadResultsAllFailed(t *testing.T) {
	filesInfo := []*utils.FileInfo{
		{Status: global.ERROR},
		{Status: global.ERROR},
	}

	_, err := summarizeImageUploadResults(filesInfo)
	assert.Error(t, err)
	assert.False(t, IsPartialImageUploadError(err))
}

func TestIsPartialImageUploadError(t *testing.T) {
	assert.False(t, IsPartialImageUploadError(nil))
	assert.False(t, IsPartialImageUploadError(errors.New("plain error")))
}
