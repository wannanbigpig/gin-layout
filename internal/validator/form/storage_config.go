package form

import "github.com/wannanbigpig/gin-layout/internal/filestorage"

type StorageConfigPayload struct {
	ActiveDriver string             `form:"active_driver" json:"active_driver" label:"存储驱动" binding:"required,oneof=local aliyun_oss s3"`
	Config       filestorage.Config `form:"config" json:"config" label:"存储配置" binding:"required"`
}

func NewStorageConfigPayload() *StorageConfigPayload {
	return &StorageConfigPayload{}
}
