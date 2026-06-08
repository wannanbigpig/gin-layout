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

func TestGetCurrentPath(t *testing.T) {
	_, err := GetCurrentPath()
	if err != nil {
		t.Error("获取运行路径失败")
	}
}

func TestGetCurrentAbPathByExecutable(t *testing.T) {
	_, err := GetCurrentAbPathByExecutable()
	if err != nil {
		t.Error("获取路径失败")
	}
}

func TestGetCurrentFileDirectory(t *testing.T) {
	path, ok := GetFileDirectoryToCaller()
	if !ok {
		t.Error("获取路径失败", path)
	}

	path, ok = GetFileDirectoryToCaller(1)
	if !ok {
		t.Error("获取路径失败", path)
	}
}

func TestIf(t *testing.T) {
	if If(false, 1, 3) != 3 {
		t.Error("模拟三元操作失败")
	}

	if If(true, 1, 3) != 1 {
		t.Error("模拟三元操作失败")
	}
}
