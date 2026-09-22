package errors

import (
	"testing"
)

func TestText(t *testing.T) {
	var errorText = NewErrorText("zh_CN")
	if errorText.Text(0) != "OK" {
		t.Error("text 返回 msg 不是预期的")
	}
	if errorText.Text(FileReferenced) != "文件存在引用，不能删除" {
		t.Error("文件引用错误码文案不是预期的")
	}
	if errorText.Text(NoExportData) != "暂无需要导出的数据" || NewErrorText("en").Text(NoExportData) != "No data to export" {
		t.Error("空导出错误码缺少中英文文案")
	}

	if errorText.Text(1202389) != "unknown error" {
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
