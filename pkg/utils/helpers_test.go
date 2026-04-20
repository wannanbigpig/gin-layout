package utils

import (
	"testing"

	"golang.org/x/crypto/bcrypt"
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

	// 空可选参数切片不应触发 panic
	emptyMaskChars := []string{}
	noPanicMasked := MaskSensitiveInfo(mobile, 3, 4, emptyMaskChars...)
	if noPanicMasked != "132****0000" {
		t.Error("空掩码参数脱敏失败")
	}

	// start 超出长度时应直接返回原值
	outOfRange := MaskSensitiveInfo(mobile, len(mobile)+3, 2)
	if outOfRange != mobile {
		t.Error("start 越界处理失败")
	}

	// maskNumber 非正时应直接返回原值
	unchanged := MaskSensitiveInfo(mobile, 3, 0)
	if unchanged != mobile {
		t.Error("maskNumber 非正处理失败")
	}
}

func TestPasswordHashUsesStrongerCost(t *testing.T) {
	hashed, err := PasswordHash("hello-password")
	if err != nil {
		t.Fatalf("password hash failed: %v", err)
	}

	cost, err := bcrypt.Cost([]byte(hashed))
	if err != nil {
		t.Fatalf("read bcrypt cost failed: %v", err)
	}
	if cost < 12 {
		t.Fatalf("expected bcrypt cost >= 12, got %d", cost)
	}
}
