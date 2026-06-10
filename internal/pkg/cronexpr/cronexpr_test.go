package cronexpr

import "testing"

func TestDescribeCommonSpecs(t *testing.T) {
	tests := []struct {
		spec string
		want string
	}{
		{spec: "0/5 * * * * *", want: "每 5 秒执行一次"},
		{spec: "0 * * * * *", want: "每分钟执行一次"},
		{spec: "0 0/5 * * * *", want: "每 5 分钟执行一次"},
		{spec: "0 0 4 * * *", want: "每天 04:00 执行"},
		{spec: "0 15 * * * *", want: "每小时第 15 分 0 秒执行"},
		{spec: "0 30 4 * * 1", want: "每周 周一 04:30 执行"},
		{spec: "0 30 4 1 * *", want: "每月 1 日 04:30 执行"},
		{spec: "@hourly", want: "每小时整点执行"},
	}

	for _, tt := range tests {
		if got := Describe(tt.spec); got != tt.want {
			t.Fatalf("Describe(%q)=%q want %q", tt.spec, got, tt.want)
		}
	}
}

func TestValidateRejectsInvalidSpec(t *testing.T) {
	if err := Validate("bad spec"); err == nil {
		t.Fatal("expected invalid cron spec to be rejected")
	}
}
