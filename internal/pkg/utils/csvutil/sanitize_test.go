package csvutil

import "testing"

func TestSanitizeCell(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"formula =1+1", "=1+1", "'=1+1"},
		{"formula +cmd", "+cmd", "'+cmd"},
		{"formula -cmd", "-cmd", "'-cmd"},
		{"formula @cmd", "@cmd", "'@cmd"},
		{"leading spaces then formula", "  =1+1", "'  =1+1"},
		{"leading newline then formula", "\n=1+1", "'\n=1+1"},
		{"plain text", "hello", "hello"},
		{"empty string", "", ""},
		{"tab prefix", "\t=cmd", "'\t=cmd"},
		{"carriage return prefix", "\r=cmd", "'\r=cmd"},
		{"tab plain prefix", "\tplain", "'\tplain"},
		{"carriage return plain prefix", "\rplain", "'\rplain"},
		{"number string", "123", "123"},
		{"space only", "   ", "   "},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SanitizeCell(tt.input)
			if got != tt.want {
				t.Errorf("SanitizeCell(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
