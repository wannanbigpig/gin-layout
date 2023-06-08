package utils

import (
	"testing"
)

func TestMaskSensitiveInfo(t *testing.T) {
	mobile := "13200000000"
	m := MaskSensitiveInfo(mobile, 3, 4)
	if m != "132****0000" {
		t.Error("手机号脱敏失败")
	}
	m1 := MaskSensitiveInfo(mobile, -1, 15)
	if m1 != "***********" {
		t.Error("手机号脱敏失败")
	}
	idNumber := "110101199001010010"
	id := MaskSensitiveInfo(idNumber, 6, 8)
	if id != "110101********0010" {
		t.Error("身份证脱敏失败")
	}
}
