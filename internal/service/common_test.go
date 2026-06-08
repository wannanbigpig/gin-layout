package service

import (
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	c "github.com/wannanbigpig/gin-layout/config"
	"github.com/wannanbigpig/gin-layout/internal/global"
	"github.com/wannanbigpig/gin-layout/internal/model"
	e "github.com/wannanbigpig/gin-layout/internal/pkg/errors"
	dateutils "github.com/wannanbigpig/gin-layout/internal/pkg/utils"
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

func TestBuildFileThumbnailURL(t *testing.T) {
	originBaseURL := c.Config.BaseURL
	t.Cleanup(func() {
		c.Config.BaseURL = originBaseURL
	})

	c.Config.BaseURL = ""
	assert.Equal(t, "/admin/v1/file/abc123?variant=thumb", buildFileThumbnailURL("abc123"))
	assert.Equal(t, "", buildFileThumbnailURL(""))
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

func TestGetFileAccessPathReturnsThumbnailForLocalImageVariant(t *testing.T) {
	db := newFileResourceSQLiteDB(t)
	basePath := t.TempDir()
	imageDir := filepath.Join(basePath, "images")
	assert.NoError(t, os.MkdirAll(imageDir, 0o755))

	originalPath := filepath.Join(imageDir, "poster.jpg")
	writeLargeJPEG(t, originalPath, 1200, 800)

	record := &model.UploadFiles{
		UUID:          "1234567890abcdef1234567890abcdef",
		Path:          "images/poster.jpg",
		FileType:      "image",
		MimeType:      "image/jpeg",
		IsPublic:      global.Yes,
		StorageDriver: model.StorageDriverLocal,
		StorageBase:   basePath,
		StoragePath:   "images/poster.jpg",
		ObjectKey:     "images/poster.jpg",
		StorageStatus: model.StorageStatusStored,
	}
	assert.NoError(t, db.Create(record).Error)

	access, err := NewCommonService().SetDB(db).GetFileAccessPath(record.UUID, fileVariantThumb, false, 0)
	assert.NoError(t, err)
	assert.FileExists(t, access.LocalPath)
	assert.Empty(t, access.RedirectURL)
	assert.NotEqual(t, originalPath, access.LocalPath)

	imageFile, err := os.Open(access.LocalPath)
	assert.NoError(t, err)
	defer imageFile.Close()

	thumbImage, _, err := image.Decode(imageFile)
	assert.NoError(t, err)
	bounds := thumbImage.Bounds()
	assert.LessOrEqual(t, bounds.Dx(), thumbnailMaxDimension)
	assert.LessOrEqual(t, bounds.Dy(), thumbnailMaxDimension)
}

func TestGetFileAccessPathRejectsExpiredFile(t *testing.T) {
	db := newFileResourceSQLiteDB(t)
	record := &model.UploadFiles{
		UUID:          "abcdef1234567890abcdef1234567890",
		Path:          "expired.txt",
		IsPublic:      global.Yes,
		StorageDriver: model.StorageDriverLocal,
		StorageBase:   t.TempDir(),
		StoragePath:   "expired.txt",
		ObjectKey:     "expired.txt",
		StorageStatus: model.StorageStatusStored,
		ExpiresAt:     dateutils.FormatDate{Time: time.Now().Add(-time.Minute)},
	}
	assert.NoError(t, db.Create(record).Error)

	_, err := NewCommonService().SetDB(db).GetFileAccessPath(record.UUID, "", false, 0)
	var businessErr *e.BusinessError
	assert.ErrorAs(t, err, &businessErr)
	assert.Equal(t, e.FileAccessDenied, businessErr.GetCode())
}

func writeLargeJPEG(t *testing.T, path string, width int, height int) {
	t.Helper()
	imageData := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			imageData.Set(x, y, color.RGBA{R: uint8(x % 255), G: uint8(y % 255), B: 200, A: 255})
		}
	}
	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("create image file failed: %v", err)
	}
	defer file.Close()
	if err := jpeg.Encode(file, imageData, &jpeg.Options{Quality: 90}); err != nil {
		t.Fatalf("encode image file failed: %v", err)
	}
}
