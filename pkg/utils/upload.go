package utils

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
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

// UploadFile 接收上传文件并保存为 SHA256 命名
// path 参数可以是相对路径或绝对路径
// 如果是相对路径，会基于二进制文件所在目录来构建完整路径
func UploadFile(fileHeader *multipart.FileHeader, path ...string) (fileInfo *FileInfo, err error) {
	uploadSubDir := "default"
	if len(path) > 0 && path[0] != "" {
		uploadSubDir = path[0]
	}

	var absolutePath string
	// 判断uploadSubDir是否绝对路径
	if filepath.IsAbs(uploadSubDir) {
		// 已经是绝对路径，直接使用
		absolutePath = uploadSubDir
	} else {
		// 相对路径，基于二进制文件所在目录构建完整路径
		baseDir, err := GetCurrentAbPathByExecutable()
		if err != nil {
			return nil, fmt.Errorf("获取执行文件目录失败: %w", err)
		}
		// 如果传入的是相对路径，默认放在 storage/default 目录下
		absolutePath = filepath.Join(baseDir, "storage", uploadSubDir)
	}

	// 确保目录存在
	if err := os.MkdirAll(absolutePath, os.ModePerm); err != nil {
		return nil, fmt.Errorf("创建上传目录失败: %w", err)
	}

	fileInfo, err = SaveUploadedFileWithUUID(fileHeader, absolutePath)

	return
}

// SaveUploadedFileWithUUID 保存文件，使用 UUID 命名，计算 SHA256
func SaveUploadedFileWithUUID(fileHeader *multipart.FileHeader, uploadDir string) (*FileInfo, error) {
	src, err := fileHeader.Open()
	if err != nil {
		return nil, fmt.Errorf("打开上传文件失败: %w", err)
	}
	defer src.Close()

	// 读取内容到 buffer，同时计算 SHA256
	buf := bytes.NewBuffer(nil)
	hash := sha256.New()

	size, err := io.Copy(io.MultiWriter(buf, hash), src)
	if err != nil {
		return nil, fmt.Errorf("读取文件失败: %w", err)
	}

	sum := hex.EncodeToString(hash.Sum(nil))
	ext := filepath.Ext(fileHeader.Filename)
	// 生成UUID（不带连字符，32位十六进制字符串）
	fileUUID := uuid.New()
	fileUUIDStr := strings.ReplaceAll(fileUUID.String(), "-", "")
	newFilename := fileUUID.String() + ext
	savePath := filepath.Join(uploadDir, newFilename)

	// 检测MIME类型
	var mimeType string
	kind, err := filetype.Match(buf.Bytes())
	if err == nil && kind != filetype.Unknown {
		mimeType = kind.MIME.Value
	} else {
		// 如果无法检测，使用扩展名推断
		mimeType = getMimeTypeByExt(ext)
	}

	// 写入临时文件再重命名
	tempPath := savePath + ".tmp"
	if err := os.WriteFile(tempPath, buf.Bytes(), 0644); err != nil {
		return nil, fmt.Errorf("写入临时文件失败: %w", err)
	}

	if err := os.Rename(tempPath, savePath); err != nil {
		_ = os.Remove(tempPath)
		return nil, fmt.Errorf("重命名文件失败: %w", err)
	}

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

// EnsureAbsPath 确保路径是绝对路径
// 如果 path 是相对路径，则基于 baseDir 转换为绝对路径
// 如果 baseDir 为空，则使用二进制文件所在目录
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

// GetFileSha256AndSizeFromHeader 计算文件的 SHA-256 和大小（需支持 ReadSeeker）
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

// IsAllowedImage 判断文件是否是允许的图片格式
func IsAllowedImage(file io.ReadSeeker) (string, bool, error) {
	head := make([]byte, 261)
	if _, err := file.Read(head); err != nil {
		return "", false, fmt.Errorf("读取文件头失败: %w", err)
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return "", false, fmt.Errorf("重置文件指针失败: %w", err)
	}

	kind, err := filetype.Match(head)
	if err != nil {
		return "", false, fmt.Errorf("检测文件类型失败: %w", err)
	}
	if kind == filetype.Unknown {
		return "", false, nil
	}

	allowed := map[string]types.Type{
		"jpg":  filetype.GetType("jpg"),
		"jpeg": filetype.GetType("jpeg"),
		"png":  filetype.GetType("png"),
		"gif":  filetype.GetType("gif"),
	}

	for _, t := range allowed {
		if kind.MIME.Value == t.MIME.Value {
			return kind.Extension, true, nil
		}
	}
	return kind.Extension, false, nil
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
