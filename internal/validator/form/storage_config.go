package form

import "github.com/wannanbigpig/gin-layout/internal/filestorage"

type StorageConfigPayload struct {
	ActiveDriver string             `form:"active_driver" json:"active_driver" label:"存储驱动" binding:"required,oneof=local aliyun_oss"`
	Config       filestorage.Config `form:"config" json:"config" label:"存储配置" binding:"required"`
}

func NewStorageConfigPayload() *StorageConfigPayload {
	return &StorageConfigPayload{}
}

type StorageSecretQuery struct {
	Driver string `form:"driver" json:"driver" label:"存储驱动" binding:"required,oneof=aliyun_oss"`
	Field  string `form:"field" json:"field" label:"敏感字段" binding:"required,oneof=access_key_secret"`
}

func NewStorageSecretQuery() *StorageSecretQuery {
	return &StorageSecretQuery{}
}
