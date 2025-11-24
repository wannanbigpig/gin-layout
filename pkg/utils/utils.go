package utils

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// If 模拟简单的三元操作
func If[T any](condition bool, trueVal, falseVal T) T {
	if condition {
		return trueVal
	}
	return falseVal
}

// WouldCauseCycle 检查新的父节点是否是当前节点的子节点，防止循环引用
func WouldCauseCycle(id, parentPid uint, parentPids string) bool {
	if id == 0 {
		return false
	}
	if parentPid == id {
		return true
	}
	// 检测循环引用
	idStr := fmt.Sprintf("%d", id)
	pidsSlice := strings.Split(parentPids, ",")
	for _, pid := range pidsSlice {
		if pid == idStr {
			return true
		}
	}
	return false
}

// GetRunPath 获取执行目录作为默认目录
func GetRunPath() string {
	currentPath, err := os.Getwd()
	if err != nil {
		return ""
	}
	return currentPath
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
		directory = filepath.Dir(filename)
	}
	return
}

// GetCurrentAbPathByExecutable 获取当前执行文件所在目录的绝对路径
// 这是最可靠的获取二进制文件所在目录的方法，适用于所有环境
func GetCurrentAbPathByExecutable() (string, error) {
	exePath, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("获取执行文件路径失败: %w", err)
	}

	// 解析符号链接，获取真实路径
	realPath, err := filepath.EvalSymlinks(exePath)
	if err != nil {
		// 如果解析符号链接失败，使用原始路径
		realPath = exePath
	}

	// 获取目录路径并转换为绝对路径
	dir := filepath.Dir(realPath)
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return "", fmt.Errorf("获取绝对路径失败: %w", err)
	}

	return absDir, nil
}

// GetCurrentPath 获取当前执行文件路径（始终使用二进制文件所在目录）
// 这是统一的路径获取方法，确保所有环境行为一致
func GetCurrentPath() (dir string, err error) {
	return GetCurrentAbPathByExecutable()
}

// GetDefaultPath 获取当前执行文件路径，如果是临时目录则获取运行命令的工作目录
func GetDefaultPath() (dir string, err error) {
	if os.Getenv("GO_ENV") != "development" {
		dir, err = GetCurrentAbPathByExecutable()
		if err != nil {
			return "", err
		}
	} else {
		dir = GetRunPath()
	}

	return dir, nil
}

// MD5 计算字符串的 MD5 值
func MD5(str string) string {
	// 计算 MD5 哈希
	hash := md5.Sum([]byte(str))

	// 将哈希值转换为十六进制字符串
	return hex.EncodeToString(hash[:])
}
