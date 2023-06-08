package utils

import (
	"testing"
)

func TestRandString(t *testing.T) {
	s := RandString(12)
	if s == "" {
		t.Error("获取运行路径失败")
	}
}

func BenchmarkRandString(b *testing.B) {
	// 基准函数会运行目标代码b.N次。
	for i := 0; i < b.N; i++ {
		RandString(12)
	}
}
