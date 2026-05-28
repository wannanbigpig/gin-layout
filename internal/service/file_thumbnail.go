package service

import (
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"strings"

	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"

	"golang.org/x/image/draw"
	_ "golang.org/x/image/webp"
)

const (
	fileVariantThumb      = "thumb"
	thumbnailMaxDimension = 320
)

func normalizeFileVariant(variant string) string {
	return strings.ToLower(strings.TrimSpace(variant))
}

func isImageMimeType(mimeType string) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(mimeType)), "image/")
}

func ensureLocalImageThumbnail(sourcePath string, mimeType string) (string, error) {
	sourcePath = strings.TrimSpace(sourcePath)
	if sourcePath == "" {
		return "", fmt.Errorf("source path is empty")
	}
	if !isImageMimeType(mimeType) {
		return sourcePath, nil
	}
	thumbnailPath := localThumbnailPath(sourcePath, mimeType)
	if currentThumbnailValid(sourcePath, thumbnailPath) {
		return thumbnailPath, nil
	}

	if err := os.MkdirAll(filepath.Dir(thumbnailPath), 0o755); err != nil {
		return "", fmt.Errorf("create thumbnail dir: %w", err)
	}

	sourceFile, err := os.Open(sourcePath)
	if err != nil {
		return "", fmt.Errorf("open source image: %w", err)
	}
	defer sourceFile.Close()

	imageData, _, err := image.Decode(sourceFile)
	if err != nil {
		return "", fmt.Errorf("decode source image: %w", err)
	}

	targetImage := resizeImageIfNeeded(imageData)
	if targetImage == imageData {
		if err := copyFile(sourcePath, thumbnailPath); err != nil {
			return "", fmt.Errorf("copy source image: %w", err)
		}
		return thumbnailPath, nil
	}

	outputFile, err := os.Create(thumbnailPath)
	if err != nil {
		return "", fmt.Errorf("create thumbnail file: %w", err)
	}
	defer outputFile.Close()

	if strings.HasSuffix(thumbnailPath, ".png") {
		if err := png.Encode(outputFile, targetImage); err != nil {
			return "", fmt.Errorf("encode thumbnail png: %w", err)
		}
		return thumbnailPath, nil
	}
	if err := jpeg.Encode(outputFile, targetImage, &jpeg.Options{Quality: 85}); err != nil {
		return "", fmt.Errorf("encode thumbnail jpeg: %w", err)
	}
	return thumbnailPath, nil
}

func localThumbnailPath(sourcePath string, mimeType string) string {
	ext := ".jpg"
	switch strings.ToLower(strings.TrimSpace(mimeType)) {
	case "image/png", "image/gif", "image/webp":
		ext = ".png"
	}
	return sourcePath + ".thumb" + ext
}

func currentThumbnailValid(sourcePath string, thumbnailPath string) bool {
	sourceInfo, err := os.Stat(sourcePath)
	if err != nil {
		return false
	}
	thumbnailInfo, err := os.Stat(thumbnailPath)
	if err != nil {
		return false
	}
	return !thumbnailInfo.ModTime().Before(sourceInfo.ModTime())
}

func resizeImageIfNeeded(src image.Image) image.Image {
	bounds := src.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()
	if width <= thumbnailMaxDimension && height <= thumbnailMaxDimension {
		return src
	}

	targetWidth := width
	targetHeight := height
	if width >= height {
		targetWidth = thumbnailMaxDimension
		targetHeight = maxInt(1, height*thumbnailMaxDimension/width)
	} else {
		targetHeight = thumbnailMaxDimension
		targetWidth = maxInt(1, width*thumbnailMaxDimension/height)
	}

	dst := image.NewRGBA(image.Rect(0, 0, targetWidth, targetHeight))
	draw.CatmullRom.Scale(dst, dst.Bounds(), src, bounds, draw.Over, nil)
	return dst
}

func copyFile(sourcePath string, targetPath string) error {
	sourceFile, err := os.Open(sourcePath)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	targetFile, err := os.Create(targetPath)
	if err != nil {
		return err
	}
	defer targetFile.Close()

	if _, err := io.Copy(targetFile, sourceFile); err != nil {
		return err
	}
	return targetFile.Sync()
}

func maxInt(a int, b int) int {
	if a > b {
		return a
	}
	return b
}
