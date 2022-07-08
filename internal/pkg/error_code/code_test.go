package error_code

import (
	"testing"
)

func TestText(t *testing.T) {
	var errorText = &ErrorText{
		Language: "zh_CN",
	}
	if "OK" != errorText.Text(0) {
		t.Error("text 返回 msg 不是预期的")
	}

	if "unknown error" != errorText.Text(1202389) {
		t.Error("text 返回 msg 不是预期的")
	}
}
