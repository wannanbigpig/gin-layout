package utils

import (
	"fmt"
	"testing"
)

func TestCalculateChanges(t *testing.T) {
	existingIds := []int{1, 2, 3, 4, 5}
	ids := []int{2, 3, 6, 7}
	toDelete, toAdd, remainingList := CalculateChanges(existingIds, ids)
	fmt.Println("toDelete:", toDelete)
	fmt.Println("toAdd:", toAdd)
	fmt.Println("remainingList:", remainingList)
}

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

func TestDesensitizeRule(b *testing.T) {
	// 手机号脱敏
	phoneRule := &DesensitizeRule{KeepPrefixLen: 3, KeepSuffixLen: 4, MaskChar: '*'}
	if phoneRule.Apply("13812345678") != "138****5678" {
		b.Error("手机号码脱敏失败")
	}

	// 邮箱脱敏
	emailRule := &DesensitizeRule{KeepPrefixLen: 2, KeepSuffixLen: 0, MaskChar: '*', Separator: '@', FixedMaskLength: 3}
	if emailRule.Apply("test@example.com") != "te***@example.com" {
		b.Error("邮箱脱敏失败")
	}
}

func TestTrimPrefixAndSuffixAND(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "trims repeated boundary and", in: "   AND AND name = ? AND  ", want: "name = ?"},
		{name: "keeps word prefix", in: "android = ? AND", want: "android = ?"},
		{name: "keeps word suffix", in: "AND brand = ?", want: "brand = ?"},
		{name: "all and", in: " AND ", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := TrimPrefixAndSuffixAND(tt.in); got != tt.want {
				t.Fatalf("expected %q, got %q", tt.want, got)
			}
		})
	}
}

func BenchmarkTrimPrefixAndSuffixAND(b *testing.B) {
	input := "   AND AND name = 'Tom' AND age = 18 AND  "
	for i := 0; i < b.N; i++ {
		_ = TrimPrefixAndSuffixAND(input)
	}
}
