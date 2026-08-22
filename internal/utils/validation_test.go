package utils

import "testing"

func TestIsValidUsername(t *testing.T) {
	cases := []struct {
		name     string
		username string
		want     bool
	}{
		// The hyphen is what the error message promises and what regressed when
		// the character class was written as `[a-z0-9.-@]`.
		{"hyphen", "claude-test", true},
		{"leading hyphen", "-claude", true},
		{"lowercase", "claudetest", true},
		{"digits", "claude2", true},
		{"dot", "claude.test", true},
		{"at sign, as provisioned for oauth emails", "claude@example.com", true},

		// These fell inside the accidental '.'-to-'@' range and were accepted.
		{"slash", "a/b", false},
		{"colon", "a:b", false},
		{"semicolon", "a;b", false},
		{"less than", "a<b", false},
		{"equals", "a=b", false},
		{"greater than", "a>b", false},
		{"question mark", "a?b", false},

		{"underscore", "claude_test", false},
		{"uppercase", "ClaudeTest", false},
		{"space", "claude test", false},
		{"empty", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := IsValidUsername(tc.username); got != tc.want {
				t.Errorf("IsValidUsername(%q) = %v, want %v", tc.username, got, tc.want)
			}
		})
	}
}
