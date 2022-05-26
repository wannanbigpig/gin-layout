package utils

import (
	"os"
	"path"
	"path/filepath"
	"runtime"
)

// If 模拟简单的三元操作
func If(condition bool, trueVal, falseVal any) any {
	if condition {
		return trueVal
	}
	return falseVal
}

// GetRunPath 获取执行目录作为默认目录
func GetRunPath() string {
	currentPath, err := os.Getwd()
	if err != nil {
		return ""
	}
	return filepath.Join(currentPath, "/gin-layout")
}

// GetFileDirectoryToCaller 根据运行堆栈信息获取文件目录，skip 默认1
func GetFileDirectoryToCaller(opts ...int) (directory string, ok bool) {
	var filename string
	directory = ""
	skip := 1
	if opts != nil {
		skip = opts[0]
	}
	if _, filename, _, ok = runtime.Caller(skip); ok {
		directory = path.Dir(filename)
	}
	return
}
