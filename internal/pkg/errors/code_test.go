package errors

import (
	"testing"
)

func TestText(t *testing.T) {
	var errorText = NewErrorText("zh_CN")
	if "OK" != errorText.Text(0) {
		t.Error("text 返回 msg 不是预期的")
	}

	if "unknown error" != errorText.Text(1202389) {
		t.Error("text 返回 msg 不是预期的")
	}
}

func TestTextByKey(t *testing.T) {
	errorText := NewErrorText("zh_CN")
	msg, ok := errorText.TextByKey(MsgKeyAuthPermissionInitFailed)
	if !ok {
		t.Fatal("expected key exists")
	}
	if msg != "权限验证初始化失败" {
		t.Fatalf("unexpected msg: %s", msg)
	}

	if _, ok := errorText.TextByKey("not.exists"); ok {
		t.Fatal("expected missing key")
	}
}
