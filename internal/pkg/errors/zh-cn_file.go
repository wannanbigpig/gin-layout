package errors

func init() {
	mergeZhCNText(map[int]string{
		FileIdentifierInvalid:            "文件标识错误",
		FilePrivateAuthNeeded:            "访问私有文件需要登录认证",
		FileAccessDenied:                 "无权访问该文件",
		FileUploadPartialFail:            "部分图片上传失败",
		FileReferenced:                   "文件存在引用，不能删除",
		StorageTestFailed:                "存储配置测试失败",
		FolderNameExists:                 "同级目录下已存在同名文件夹",
		FolderNotEmpty:                   "文件夹不为空，请先删除其中的文件和子文件夹",
		FolderMoveInvalid:                "不能将文件夹移动到自身或其子文件夹",
		FolderNameInvalid:                "文件夹名称不合法",
		LocalStorageMultipartUnsupported: "本地存储不支持分片上传",
		FileTooLarge:                     "文件大小超出限制",
		FileMimeTypeNotAllowed:           "不支持的文件类型",
	})
}
