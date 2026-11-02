package guard

import "testing"

func TestRedactPII(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"cpf", "user 123.456.789-09 logged in", "user [REDACTED] logged in"},
		{"email", "contact foo@bar.com please", "contact [REDACTED] please"},
		{"openai key", "key sk-abc123def456ghi789jkl012mno", "key [REDACTED]"},
		{"odotoken key", "secret odt_abcdefghij123456 here", "secret [REDACTED] here"},
		{"no pii", "hello world", "hello world"},
		{"multiple", "cpf 111.222.333-44 and mail a@b.co", "cpf [REDACTED] and mail [REDACTED]"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := string(RedactPII([]byte(tt.in)))
			if got != tt.want {
				t.Errorf("RedactPII(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}