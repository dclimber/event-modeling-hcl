package version

import (
	"runtime/debug"
	"testing"
)

func TestResolveToolVersion(t *testing.T) {
	tests := []struct {
		name     string
		injected string
		info     *debug.BuildInfo
		ok       bool
		want     string
	}{
		{name: "injected release takes precedence", injected: "v0.6.0", info: &debug.BuildInfo{Main: debug.Module{Version: "v0.5.0"}}, ok: true, want: "v0.6.0"},
		{name: "module version supports go install", injected: "dev", info: &debug.BuildInfo{Main: debug.Module{Version: "v0.6.0"}}, ok: true, want: "v0.6.0"},
		{name: "modified checkout stays dev", injected: "dev", info: &debug.BuildInfo{Main: debug.Module{Version: "v0.5.0+dirty"}, Settings: []debug.BuildSetting{{Key: "vcs.modified", Value: "true"}}}, ok: true, want: "dev"},
		{name: "development module stays dev", injected: "dev", info: &debug.BuildInfo{Main: debug.Module{Version: "(devel)"}}, ok: true, want: "dev"},
		{name: "empty module version stays dev", info: &debug.BuildInfo{}, ok: true, want: "dev"},
		{name: "unavailable build info stays dev", want: "dev"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := resolveToolVersion(test.injected, test.info, test.ok); got != test.want {
				t.Fatalf("resolveToolVersion() = %q, want %q", got, test.want)
			}
		})
	}
}
