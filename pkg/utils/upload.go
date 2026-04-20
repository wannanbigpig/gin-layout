package utils

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	"github.com/h2non/filetype"
	"github.com/h2non/filetype/types"
)

// FileInfo 描述上传文件的存储结果和对外展示字段。
type FileInfo struct {
	FileID        uint   `json:"-"`              // 文件ID（数据库ID，不返回给前端）
	Sha256        string `json:"sha256"`         // 文件SHA256哈希值（用于去重）
	UUID          string `json:"uuid"`           // 文件UUID（用于URL访问，32位十六进制字符串）
	Name          string `json:"name"`           // 存储的文件名（UUID+扩展名）
	OriginName    string `json:"origin_name"`    // 原始文件名
	Size          int64  `json:"size"`           // 文件大小（字节）
	Path          string `json:"path"`           // 文件路径
	Ext           string `json:"ext"`            // 文件扩展名
	MimeType      string `json:"mime_type"`      // MIME类型
	URL           string `json:"url"`            // 文件访问完整URL
	FailureReason string `json:"failure_reason"` // 失败原因
	Status        string `json:"status"`         // 上传状态：SUCCESS、ERROR
}

const fileHeaderSampleSize = 261
const uploadBufferSize = 32 * 1024

// ErrInvalidImageType 表示上传文件不是允许的图片类型。
var ErrInvalidImageType = errors.New("uploaded file is not an allowed image")

// UploadFile 接收上传文件并保存到目标目录。
func UploadFile(fileHeader *multipart.FileHeader, path ...string) (fileInfo *FileInfo, err error) {
	uploadSubDir := "default"
	if len(path) > 0 && path[0] != "" {
		uploadSubDir = path[0]
	}

	absolutePath, err := resolveUploadDir(uploadSubDir)
	if err != nil {
		return nil, err
	}
	return SaveUploadedFileWithUUID(fileHeader, absolutePath)
}

// SaveUploadedFileWithUUID 保存文件、计算摘要并生成 UUID 文件名。
func SaveUploadedFileWithUUID(fileHeader *multipart.FileHeader, uploadDir string) (*FileInfo, error) {
	return saveUploadedFile(fileHeader, uploadDir, false)
}

// SaveUploadedImageWithUUID 保存图片文件，拒绝非允许图片类型。
func SaveUploadedImageWithUUID(fileHeader *multipart.FileHeader, uploadDir string) (*FileInfo, error) {
	return saveUploadedFile(fileHeader, uploadDir, true)
}

// EnsureAbsPath 将相对路径转换为绝对路径。
func EnsureAbsPath(path string, baseDir ...string) (string, error) {
	if filepath.IsAbs(path) {
		return path, nil
	}

	var base string
	if len(baseDir) > 0 && baseDir[0] != "" {
		base = baseDir[0]
	} else {
		// 默认使用二进制文件所在目录
		var err error
		base, err = GetCurrentAbPathByExecutable()
		if err != nil {
			return "", fmt.Errorf("获取执行文件目录失败: %w", err)
		}
	}
	return filepath.Join(base, path), nil
}

// GetFileSha256AndSizeFromHeader 计算文件的 SHA-256 和大小。
func GetFileSha256AndSizeFromHeader(file io.ReadSeeker) (string, int64, error) {
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return "", 0, fmt.Errorf("指针重置失败: %w", err)
	}

	hash := sha256.New()
	size, err := io.Copy(hash, file)
	if err != nil {
		return "", 0, fmt.Errorf("计算SHA-256失败: %w", err)
	}

	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return "", 0, fmt.Errorf("指针重置失败: %w", err)
	}

	return hex.EncodeToString(hash.Sum(nil)), size, nil
}

// IsAllowedImage 判断文件头是否匹配允许的图片类型。
func IsAllowedImage(file io.ReadSeeker) (string, bool, error) {
	head := make([]byte, fileHeaderSampleSize)
	n, err := file.Read(head)
	if err != nil && !errors.Is(err, io.EOF) {
		return "", false, fmt.Errorf("读取文件头失败: %w", err)
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return "", false, fmt.Errorf("重置文件指针失败: %w", err)
	}
	if n == 0 {
		return "", false, nil
	}

	ext, allowed, _, err := detectAllowedImage(head[:n])
	if err != nil {
		return "", false, fmt.Errorf("检测文件类型失败: %w", err)
	}
	return ext, allowed, nil
}

// getMimeTypeByExt 根据扩展名获取MIME类型
func getMimeTypeByExt(ext string) string {
	extMap := map[string]string{
		".jpg":  "image/jpeg",
		".jpeg": "image/jpeg",
		".png":  "image/png",
		".gif":  "image/gif",
		".webp": "image/webp",
		".pdf":  "application/pdf",
		".doc":  "application/msword",
		".docx": "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
		".xls":  "application/vnd.ms-excel",
		".xlsx": "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
		".zip":  "application/zip",
		".txt":  "text/plain",
	}
	if mime, ok := extMap[strings.ToLower(ext)]; ok {
		return mime
	}
	return "application/octet-stream"
}

func resolveUploadDir(uploadSubDir string) (string, error) {
	if filepath.IsAbs(uploadSubDir) {
		if err := ensureUploadDir(uploadSubDir); err != nil {
			return "", err
		}
		return uploadSubDir, nil
	}

	baseDir, err := GetCurrentAbPathByExecutable()
	if err != nil {
		return "", fmt.Errorf("获取执行文件目录失败: %w", err)
	}
	absolutePath := filepath.Join(baseDir, "storage", uploadSubDir)
	if err := ensureUploadDir(absolutePath); err != nil {
		return "", err
	}
	return absolutePath, nil
}

func ensureUploadDir(uploadDir string) error {
	if err := os.MkdirAll(uploadDir, os.ModePerm); err != nil {
		return fmt.Errorf("创建上传目录失败: %w", err)
	}
	return nil
}

func saveUploadedFile(fileHeader *multipart.FileHeader, uploadDir string, imagesOnly bool) (*FileInfo, error) {
	if err := ensureUploadDir(uploadDir); err != nil {
		return nil, err
	}

	src, err := fileHeader.Open()
	if err != nil {
		return nil, fmt.Errorf("打开上传文件失败: %w", err)
	}
	defer src.Close()

	tempFile, err := os.CreateTemp(uploadDir, "upload-*")
	if err != nil {
		return nil, fmt.Errorf("创建临时文件失败: %w", err)
	}
	tempPath := tempFile.Name()
	keepTemp := false
	defer func() {
		_ = tempFile.Close()
		if !keepTemp {
			_ = os.Remove(tempPath)
		}
	}()

	headerBytes, size, sum, err := copyUploadedContent(tempFile, src)
	if err != nil {
		return nil, err
	}

	detectedExt, mimeType, err := detectMimeType(headerBytes)
	if err != nil {
		return nil, fmt.Errorf("检测文件类型失败: %w", err)
	}
	if imagesOnly {
		if _, allowed, detectedMime, detectErr := detectAllowedImage(headerBytes); detectErr != nil {
			return nil, fmt.Errorf("检测文件类型失败: %w", detectErr)
		} else if !allowed {
			return nil, ErrInvalidImageType
		} else if detectedMime != "" {
			mimeType = detectedMime
		}
	}

	ext := filepath.Ext(fileHeader.Filename)
	if detectedExt != "" {
		ext = "." + detectedExt
	}
	if mimeType == "" {
		mimeType = getMimeTypeByExt(ext)
	}

	fileUUID := uuid.New()
	fileUUIDStr := strings.ReplaceAll(fileUUID.String(), "-", "")
	newFilename := fileUUID.String() + ext
	savePath := filepath.Join(uploadDir, newFilename)

	if err := tempFile.Chmod(0644); err != nil {
		return nil, fmt.Errorf("设置文件权限失败: %w", err)
	}
	if err := tempFile.Close(); err != nil {
		return nil, fmt.Errorf("关闭临时文件失败: %w", err)
	}
	if err := os.Rename(tempPath, savePath); err != nil {
		return nil, fmt.Errorf("重命名文件失败: %w", err)
	}
	keepTemp = true

	return &FileInfo{
		OriginName: fileHeader.Filename,
		Name:       newFilename,
		Path:       savePath,
		Size:       size,
		Sha256:     sum,
		UUID:       fileUUIDStr,
		Ext:        ext,
		MimeType:   mimeType,
		Status:     "SUCCESS",
	}, nil
}

func copyUploadedContent(dst io.Writer, src io.Reader) ([]byte, int64, string, error) {
	hash := sha256.New()
	writer := io.MultiWriter(dst, hash)
	header := make([]byte, 0, fileHeaderSampleSize)
	buffer := make([]byte, uploadBufferSize)
	var total int64

	for {
		n, readErr := src.Read(buffer)
		if n > 0 {
			chunk := buffer[:n]
			header = appendHeaderSample(header, chunk)
			written, err := writeUploadChunk(writer, chunk)
			if err != nil {
				return nil, 0, "", err
			}
			total += int64(written)
		}

		done, err := shouldStopUploadRead(readErr)
		if !done {
			continue
		}
		if err == nil {
			return header, total, hex.EncodeToString(hash.Sum(nil)), nil
		}
		return nil, 0, "", fmt.Errorf("读取文件失败: %w", err)
	}
}

func appendHeaderSample(header []byte, chunk []byte) []byte {
	if len(header) >= fileHeaderSampleSize {
		return header
	}
	remaining := fileHeaderSampleSize - len(header)
	if remaining > len(chunk) {
		remaining = len(chunk)
	}
	return append(header, chunk[:remaining]...)
}

func writeUploadChunk(dst io.Writer, chunk []byte) (int, error) {
	written, err := dst.Write(chunk)
	if err != nil {
		return 0, fmt.Errorf("写入临时文件失败: %w", err)
	}
	return written, nil
}

func shouldStopUploadRead(err error) (bool, error) {
	if err == nil {
		return false, nil
	}
	if errors.Is(err, io.EOF) {
		return true, nil
	}
	return true, err
}

func detectMimeType(header []byte) (string, string, error) {
	if len(header) == 0 {
		return "", "", nil
	}
	kind, err := filetype.Match(header)
	if err != nil {
		return "", "", err
	}
	if kind == filetype.Unknown {
		return "", "", nil
	}
	return kind.Extension, kind.MIME.Value, nil
}

func detectAllowedImage(header []byte) (string, bool, string, error) {
	if len(header) == 0 {
		return "", false, "", nil
	}

	kind, err := filetype.Match(header)
	if err != nil {
		return "", false, "", err
	}
	if kind == filetype.Unknown {
		return "", false, "", nil
	}

	allowed := map[string]types.Type{
		"jpg":  filetype.GetType("jpg"),
		"jpeg": filetype.GetType("jpeg"),
		"png":  filetype.GetType("png"),
		"gif":  filetype.GetType("gif"),
	}

	for _, imageType := range allowed {
		if kind.MIME.Value == imageType.MIME.Value {
			return kind.Extension, true, kind.MIME.Value, nil
		}
	}
	return kind.Extension, false, kind.MIME.Value, nil
}
