// Command wasm compiles the emhcl core to WebAssembly and
// exposes it to a browser as two global functions, eventModelingRender and
// eventModelingFormat. It holds no logic of its own beyond marshaling
// js.Value arguments into internal/app calls and its plain result structs
// back into JS values — every real behavior (parsing, validating, rendering,
// formatting, diagnostic codes) lives in app, where it is unit-tested under
// the normal (non-wasm) Go toolchain. This file cannot be
// covered by `go test` at all (it only builds under GOOS=js GOARCH=wasm),
// which is exactly why it is kept this thin.
//
//go:build js && wasm

package main

import (
	"syscall/js"

	"github.com/event-modeling-hcl/eventmodeling-hcl/internal/app"
)

// profileArg reads an optional profile-name argument (args[index]), falling
// back to app.Valid — the same default the CLI's `validate` uses —
// when absent, not a string, or unrecognized.
func profileArg(args []js.Value, index int) app.Profile {
	if len(args) <= index || args[index].Type() != js.TypeString {
		return app.Valid
	}
	profile, ok := app.ParseProfile(args[index].String())
	if !ok {
		return app.Valid
	}
	return profile
}

func diagnosticsToJS(diagnostics []app.Diagnostic) []interface{} {
	items := make([]interface{}, len(diagnostics))
	for index, diagnostic := range diagnostics {
		items[index] = map[string]interface{}{
			"code":     diagnostic.Code,
			"severity": diagnostic.Severity,
			"summary":  diagnostic.Summary,
			"detail":   diagnostic.Detail,
			"line":     diagnostic.Line,
			"column":   diagnostic.Column,
		}
	}
	return items
}

// render implements eventModelingRender(source, profile?) -> {html, diagnostics}.
func render(_ js.Value, args []js.Value) interface{} {
	if len(args) < 1 || args[0].Type() != js.TypeString {
		return map[string]interface{}{"error": "eventModelingRender: missing source string argument"}
	}

	result := app.Render("playground.em.hcl", []byte(args[0].String()), profileArg(args, 1))
	return map[string]interface{}{
		"html":        result.HTML,
		"diagnostics": diagnosticsToJS(result.Diagnostics),
	}
}

// format implements eventModelingFormat(source) -> {source, diagnostics}.
func format(_ js.Value, args []js.Value) interface{} {
	if len(args) < 1 || args[0].Type() != js.TypeString {
		return map[string]interface{}{"error": "eventModelingFormat: missing source string argument"}
	}

	result := app.Format("playground.em.hcl", []byte(args[0].String()))
	return map[string]interface{}{
		"source":      result.Source,
		"diagnostics": diagnosticsToJS(result.Diagnostics),
	}
}

func main() {
	js.Global().Set("eventModelingRender", js.FuncOf(render))
	js.Global().Set("eventModelingFormat", js.FuncOf(format))

	if ready := js.Global().Get("onEventModelingReady"); ready.Truthy() {
		ready.Invoke()
	}

	select {}
}
