package errors

func init() {
	mergeEnUSText(map[int]string{
		FileIdentifierInvalid:            "invalid file identifier",
		FilePrivateAuthNeeded:            "login required for private file access",
		FileAccessDenied:                 "no permission to access this file",
		FileUploadPartialFail:            "partial image upload failure",
		FileReferenced:                   "file is referenced and cannot be deleted",
		StorageTestFailed:                "storage configuration test failed",
		FolderNameExists:                 "a folder with the same name already exists in this directory",
		FolderNotEmpty:                   "folder is not empty, please delete its files and subfolders first",
		FolderMoveInvalid:                "cannot move a folder into itself or its descendants",
		FolderNameInvalid:                "invalid folder name",
		LocalStorageMultipartUnsupported: "local storage does not support multipart upload",
		FileTooLarge:                     "file size exceeds the limit",
		FileMimeTypeNotAllowed:           "file type is not allowed",
	})
}
