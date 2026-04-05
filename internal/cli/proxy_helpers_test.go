package cli

import (
	"testing"
)

func TestFormatTokenCount(t *testing.T) {
	cases := []struct {
		n    int
		want string
	}{
		{0, "0"},
		{1, "1"},
		{999, "999"},
		{1000, "1,000"},
		{1234, "1,234"},
		{12345, "12,345"},
		{123456, "123,456"},
		{1234567, "1,234,567"},
		{185234, "185,234"},
		{142000, "142,000"},
	}
	for _, tc := range cases {
		got := formatTokenCount(tc.n)
		if got != tc.want {
			t.Errorf("formatTokenCount(%d) = %q, want %q", tc.n, got, tc.want)
		}
	}
}

func TestTruncateLabel(t *testing.T) {
	cases := []struct {
		s      string
		maxLen int
		want   string
	}{
		{"short", 10, "short"},
		{"exactly10!", 10, "exactly10!"},
		{"toolongstring", 10, "toolong..."},
		{"abc", 3, "abc"},
		{"abcd", 3, "abc"},
		{"File: /home/user/dev/hitch/internal/proxy/server.go", 60, "File: /home/user/dev/hitch/internal/proxy/server.go"},
		{"File: /home/user/dev/hitch/internal/proxy/server.go", 20, "File: /home/user/..."},
		{"", 10, ""},
	}
	for _, tc := range cases {
		got := truncateLabel(tc.s, tc.maxLen)
		if got != tc.want {
			t.Errorf("truncateLabel(%q, %d) = %q, want %q", tc.s, tc.maxLen, got, tc.want)
		}
		if len(got) > tc.maxLen {
			t.Errorf("truncateLabel(%q, %d): result len %d exceeds maxLen", tc.s, tc.maxLen, len(got))
		}
	}
}
