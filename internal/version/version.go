// Package version holds the Event Modeling HCL specification version and
// resolves the CLI tool version embedded by GoReleaser or the Go module build.
package version

import "runtime/debug"

// Spec is the version of the Event Modeling HCL Specification that this
// renderer targets.
const Spec = "v0.3.0"

// ToolVersion returns an injected release version when present, otherwise the
// main module version recorded by a versioned go install build. Local builds
// without either source report dev.
func ToolVersion(injected string) string {
	info, ok := debug.ReadBuildInfo()
	return resolveToolVersion(injected, info, ok)
}

func resolveToolVersion(injected string, info *debug.BuildInfo, ok bool) string {
	if injected != "" && injected != "dev" {
		return injected
	}
	if !ok || info == nil || buildIsModified(info.Settings) {
		return "dev"
	}
	if info.Main.Version != "" && info.Main.Version != "(devel)" {
		return info.Main.Version
	}
	return "dev"
}

func buildIsModified(settings []debug.BuildSetting) bool {
	for _, setting := range settings {
		if setting.Key == "vcs.modified" && setting.Value == "true" {
			return true
		}
	}
	return false
}
