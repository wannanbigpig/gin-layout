package utils

import (
	"testing"
)

func TestGetRunPath(t *testing.T) {
	path := GetRunPath()
	if path == "" {
		t.Error("获取运行路径失败")
	}
}

func TestGetCurrentFileDirectory(t *testing.T) {
	path, ok := GetFileDirectoryToCaller()
	if !ok {
		t.Error("获取路径失败", path)
	}
}
