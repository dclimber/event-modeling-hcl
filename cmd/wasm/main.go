// Command wasm compiles the emhcl core to WebAssembly and
// exposes it to a browser as two global functions, eventModelingRender and
// eventModelingFormat. eventModelingRender takes either one source string or
// an array of {name, source} files that form one folder model. It holds no
// logic of its own beyond marshaling
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
	"sort"
	"syscall/js"

	"github.com/event-modeling-hcl/eventmodeling-hcl/internal/app"
	"github.com/event-modeling-hcl/eventmodeling-hcl/internal/syntax"
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
			"file":     diagnostic.Filename,
		}
	}
	return items
}

// render implements eventModelingRender(input, profile?) -> {html, diagnostics}.
//
// input is either a string — one source rendered under the name
// "playground.em.hcl" — or an array of {name, source} objects, which the
// playground treats as a folder model: the files are sorted by name
// (byte-wise, like a directory listing) and rendered together as the model
// "playground". Each diagnostic carries its source file name as "file".
func render(_ js.Value, args []js.Value) interface{} {
	if len(args) < 1 {
		return map[string]interface{}{"error": "eventModelingRender: missing source string argument"}
	}

	var result app.RenderResult
	switch {
	case args[0].Type() == js.TypeString:
		result = app.Render("playground.em.hcl", []byte(args[0].String()), profileArg(args, 1))
	case js.Global().Get("Array").Call("isArray", args[0]).Bool():
		files, message := filesArg(args[0])
		if message != "" {
			return map[string]interface{}{"error": message}
		}
		result = app.RenderFiles("playground", files, profileArg(args, 1))
	default:
		return map[string]interface{}{"error": "eventModelingRender: missing source string argument"}
	}
	return map[string]interface{}{
		"html":        result.HTML,
		"diagnostics": diagnosticsToJS(result.Diagnostics),
	}
}

// filesArg converts a JS array of {name, source} objects into files sorted
// by name. A non-empty message is the error to report to the caller.
func filesArg(array js.Value) ([]syntax.File, string) {
	length := array.Length()
	if length == 0 {
		return nil, "eventModelingRender: no files"
	}
	files := make([]syntax.File, length)
	seen := make(map[string]bool, length)
	for index := range files {
		entry := array.Index(index)
		if entry.Type() != js.TypeObject {
			return nil, "eventModelingRender: each file needs a string name and source"
		}
		name, source := entry.Get("name"), entry.Get("source")
		if name.Type() != js.TypeString || source.Type() != js.TypeString {
			return nil, "eventModelingRender: each file needs a string name and source"
		}
		if seen[name.String()] {
			return nil, "eventModelingRender: duplicate file name " + name.String()
		}
		seen[name.String()] = true
		files[index] = syntax.File{Name: name.String(), Source: []byte(source.String())}
	}
	sort.Slice(files, func(left, right int) bool { return files[left].Name < files[right].Name })
	return files, ""
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
