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

	path, ok = GetFileDirectoryToCaller(1)
	if !ok {
		t.Error("获取路径失败", path)
	}
}

func TestIf(t *testing.T) {
	a := 1
	if 3 != If(a != 1, 1, 3) {
		t.Error("模拟三元操作失败")
	}

	if 1 != If(a == 1, 1, 3) {
		t.Error("模拟三元操作失败")
	}
}
