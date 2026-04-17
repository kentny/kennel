package sandbox

import (
	"reflect"
	"testing"
)

func TestBuildRunArgs(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		sandbox string
		plugin  []string
		extra   []string
		want    []string
	}{
		{
			name:    "no plugin, no extra — SANDBOX only",
			sandbox: "claude-tb",
			want:    []string{"sandbox", "run", "claude-tb"},
		},
		{
			name:    "plugin only — `--` then --plugin-dir pairs",
			sandbox: "claude-tb",
			plugin:  []string{"--plugin-dir", "/opt/plugins/a"},
			want:    []string{"sandbox", "run", "claude-tb", "--", "--plugin-dir", "/opt/plugins/a"},
		},
		{
			name:    "extra only — `--` then extras",
			sandbox: "claude-tb",
			extra:   []string{"--debug"},
			want:    []string{"sandbox", "run", "claude-tb", "--", "--debug"},
		},
		{
			name:    "both — plugins first, extras second, single `--`",
			sandbox: "claude-tb",
			plugin:  []string{"--plugin-dir", "/opt/plugins/a"},
			extra:   []string{"--debug"},
			want:    []string{"sandbox", "run", "claude-tb", "--", "--plugin-dir", "/opt/plugins/a", "--debug"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := buildRunArgs(tc.sandbox, tc.plugin, tc.extra)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("buildRunArgs = %v, want %v", got, tc.want)
			}
		})
	}
}
