package errors

// 文件相关错误码（11000-11999）。
const (
	FileIdentifierInvalid            = 11001
	FilePrivateAuthNeeded            = 11002
	FileAccessDenied                 = 11003
	FileUploadPartialFail            = 11004
	FileReferenced                   = 11005
	StorageTestFailed                = 11006
	FolderNameExists                 = 11007
	FolderNotEmpty                   = 11008
	FolderMoveInvalid                = 11009
	FolderNameInvalid                = 11010
	LocalStorageMultipartUnsupported = 11011
	FileTooLarge                     = 11012
	FileMimeTypeNotAllowed           = 11013
	FileNameExists                   = 11014
	FileNameInvalid                  = 11015
)
