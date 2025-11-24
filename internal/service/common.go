package service

import (
	"fmt"
	"mime/multipart"
	"path/filepath"

	c "github.com/wannanbigpig/gin-layout/config"
	"github.com/wannanbigpig/gin-layout/internal/global"
	"github.com/wannanbigpig/gin-layout/internal/model"
	e "github.com/wannanbigpig/gin-layout/internal/pkg/errors"
	"github.com/wannanbigpig/gin-layout/pkg/utils"
)

// CommonService 通用服务
type CommonService struct {
	Base
}

func NewCommonService() *CommonService {
	return &CommonService{}
}

func (s CommonService) UploadFiles(files []*multipart.FileHeader, path string) ([]*utils.FileInfo, error) {
	var filesInfo []*utils.FileInfo
	for _, fileHeader := range files {
		file, err := s.UploadFile(fileHeader, true, path)
		if err != nil {
			// 记录错误但不中断
			fmt.Printf("文件上传失败: %s, 错误: %v\n", fileHeader.Filename, err)
		}
		filesInfo = append(filesInfo, file)
	}

	return filesInfo, nil
}

func (s CommonService) UploadFile(fileHeader *multipart.FileHeader, isPublic bool, path string) (*utils.FileInfo, error) {
	ext := filepath.Ext(fileHeader.Filename)
	fileInfo := &utils.FileInfo{
		OriginName: fileHeader.Filename,
		Size:       fileHeader.Size,
		Ext:        ext,
		Status:     global.ERROR,
	}

	// 1. 大小判断
	if fileHeader.Size > 10*1024*1024 {
		return setFileFailure(fileInfo, "文件大小不能大于10M", nil)
	}

	// 2. 打开文件
	file, err := fileHeader.Open()
	if err != nil {
		return setFileFailure(fileInfo, "文件读取失败", err)
	}
	defer file.Close()

	// 3. 校验 SHA-256 和大小
	sha256, _, err := utils.GetFileSha256AndSizeFromHeader(file)
	if err != nil {
		return setFileFailure(fileInfo, "文件校验失败", err)
	}
	fileInfo.Sha256 = sha256

	// 4. 格式校验
	_, allowed, err := utils.IsAllowedImage(file)
	if err != nil || !allowed {
		return setFileFailure(fileInfo, "暂不支持该格式文件", err)
	}

	// 校验文件是否已存在（通过hash判断，实现去重）
	uploadFile := model.NewUploadFiles()
	err = uploadFile.GetDetail(uploadFile, "hash = ? AND is_public = ?", sha256, func() uint8 {
		if isPublic {
			return global.Yes
		}
		return global.No
	}())
	if err == nil {
		// 文件已存在，直接返回（不重复保存文件）
		fileInfo.FileID = uploadFile.ID
		fileInfo.Path = uploadFile.Path
		fileInfo.Name = uploadFile.Name
		fileInfo.Size = int64(uploadFile.Size)
		fileInfo.Ext = uploadFile.Ext
		fileInfo.Sha256 = uploadFile.Hash
		fileInfo.UUID = uploadFile.UUID
		fileInfo.MimeType = uploadFile.MimeType
		fileInfo.Status = global.SUCCESS
		return fileInfo, nil
	}

	var basePath string
	// 5. 保存文件
	if isPublic {
		basePath = filepath.Join(c.Config.BasePath, "storage/public")
	} else {
		basePath = filepath.Join(c.Config.BasePath, "storage/private")
	}

	if path == "" {
		path = "default"
	}

	result, err := utils.UploadFile(fileHeader, filepath.Join(basePath, path))
	if err != nil {
		return setFileFailure(fileInfo, "文件保存失败", err)
	}

	// 计算相对路径
	relPath, err := filepath.Rel(basePath, result.Path)
	if err != nil {
		return setFileFailure(fileInfo, "上传路径获取异常", err)
	}

	result.Path = relPath

	// 6. 保存文件信息到数据库，获取文件ID
	uploadFileModel := model.UploadFiles{
		UID:        s.GetAdminUserId(),
		OriginName: result.OriginName,
		Name:       result.Name,
		Path:       result.Path,
		Size:       uint(result.Size),
		Ext:        result.Ext,
		Hash:       result.Sha256,
		UUID:       result.UUID,
		MimeType:   result.MimeType,
		IsPublic: func() uint8 {
			if isPublic {
				return global.Yes
			} else {
				return global.No
			}
		}(),
	}
	err = model.NewUploadFiles().DB().Create(&uploadFileModel).Error
	if err != nil {
		return setFileFailure(fileInfo, "保存文件信息失败", err)
	}

	result.FileID = uploadFileModel.ID
	fileInfo.FileID = uploadFileModel.ID
	fileInfo.Path = result.Path
	fileInfo.Name = result.Name
	fileInfo.Size = result.Size
	fileInfo.Ext = result.Ext
	fileInfo.Sha256 = result.Sha256
	fileInfo.UUID = result.UUID
	fileInfo.MimeType = result.MimeType
	fileInfo.Status = global.SUCCESS

	return fileInfo, nil
}

// GetFileAccessPath 获取文件访问路径
// fileUUID: 文件UUID（32位十六进制字符串，不带连字符），用于URL访问
// checkAuth: 是否检查权限（私有文件需要检查）
// currentUID: 当前用户ID（用于权限检查，0表示未登录）
func (s CommonService) GetFileAccessPath(fileUUID string, checkAuth bool, currentUID uint) (string, error) {
	if len(fileUUID) != 32 {
		return "", e.NewBusinessError(1, "文件标识格式错误，应使用32位UUID")
	}

	uploadFile := model.NewUploadFiles()
	// 通过UUID查询（更短，适合URL）
	err := uploadFile.GetDetail(uploadFile, "uuid = ?", fileUUID)
	if err != nil {
		return "", e.NewBusinessError(1, "文件不存在")
	}

	// 检查访问权限
	if uploadFile.IsPublic == global.No {
		// 私有文件需要权限检查
		if !checkAuth || currentUID == 0 {
			return "", e.NewBusinessError(1, "该文件为私有文件，需要登录认证")
		}
		// 检查是否是文件所有者
		if uploadFile.UID != currentUID {
			return "", e.NewBusinessError(1, "无权访问该文件")
		}
	}

	// 构建文件完整路径
	var basePath string
	if uploadFile.IsPublic == global.Yes {
		basePath = filepath.Join(c.Config.BasePath, "storage/public")
	} else {
		basePath = filepath.Join(c.Config.BasePath, "storage/private")
	}

	// 拼接完整文件路径
	fullPath := filepath.Join(basePath, uploadFile.Path, uploadFile.Name)
	return fullPath, nil
}

func setFileFailure(info *utils.FileInfo, reason string, err error) (*utils.FileInfo, error) {
	info.FailureReason = reason
	info.Status = global.ERROR
	return info, err
}
