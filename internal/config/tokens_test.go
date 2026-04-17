package config

import "testing"

func TestExpandTokens(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		in   string
		tok  Tokens
		want string
	}{
		{
			name: "replaces all three tokens",
			in:   "{agent}-{project}-{env}",
			tok:  Tokens{Project: "tb", Agent: "claude", Env: "3"},
			want: "claude-tb-3",
		},
		{
			name: "empty env leaves placeholder",
			in:   "{project}/env-{env}",
			tok:  Tokens{Project: "tb", Agent: "claude"},
			want: "tb/env-{env}",
		},
		{
			name: "no tokens in string is a no-op",
			in:   "plain-string",
			tok:  Tokens{Project: "x", Agent: "y", Env: "z"},
			want: "plain-string",
		},
		{
			name: "token appears multiple times",
			in:   "{project}-{project}",
			tok:  Tokens{Project: "k"},
			want: "k-k",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ExpandTokens(tc.in, tc.tok)
			if got != tc.want {
				t.Errorf("ExpandTokens(%q, %+v) = %q, want %q", tc.in, tc.tok, got, tc.want)
			}
		})
	}
}
