package sensitive

import (
	"strings"
	"testing"
)

func TestMaskQueryString_JWTReadable(t *testing.T) {
	jwt := "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.aHEQq0SignaturePart"
	raw := "client=admin-web&lang=zh-CN&token=" + jwt
	out := MaskQueryString(raw)

	// url.Values.Encode 会将 * 编码为 %2A，这是标准行为
	if !strings.Contains(out, "token=eyJhbG%2A%2A%2A") {
		t.Fatalf("expected JWT to be masked with maskToken (6 prefix + *** + 6 suffix), got %q", out)
	}
	if strings.Contains(out, "eyJhbGciOiJI") {
		t.Fatalf("expected JWT alg header to be masked, got %q", out)
	}
}

func TestMaskDefault_FoldsLongStrings(t *testing.T) {
	short := "abcdefghij"
	if got := maskDefault(short); got != "a********j" {
		t.Fatalf("short string mask: want a********j, got %q", got)
	}

	long := strings.Repeat("x", 64)
	got := maskDefault(long)
	if got != "x***x" {
		t.Fatalf("long string should fold middle stars, want x***x, got %q", got)
	}
}
