// Package app composes the emhcl pipeline — parse, validate,
// build, render/format — exactly once per operation, and defines the single
// plain Diagnostic type every entrypoint (CLI, WASM, serve) reports through.
// Each entrypoint used to assemble internal/syntax, internal/validator,
// internal/model, and internal/renderer/internal/formatter itself, which
// meant parsing a document twice per render (once to validate, once to
// build) and maintaining two separate hcl.Diagnostic-to-plain conversions
// that had to be kept in lockstep by hand. app exists so there is exactly
// one of each.
package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/event-modeling-hcl/eventmodeling-hcl/internal/formatter"
	"github.com/event-modeling-hcl/eventmodeling-hcl/internal/interchange"
	"github.com/event-modeling-hcl/eventmodeling-hcl/internal/model"
	"github.com/event-modeling-hcl/eventmodeling-hcl/internal/renderer"
	sourcepkg "github.com/event-modeling-hcl/eventmodeling-hcl/internal/source"
	"github.com/event-modeling-hcl/eventmodeling-hcl/internal/syntax"
	"github.com/event-modeling-hcl/eventmodeling-hcl/internal/validator"
	"github.com/hashicorp/hcl/v2"
)

// Profile re-exports validator.Profile so callers need only import app.
type Profile = validator.Profile

// Profile values, re-exported from validator for the same reason.
const (
	Workshop = validator.Workshop
	Valid    = validator.Valid
	Strict   = validator.Strict
)

// ExportResult is the output of Export.
type ExportResult struct {
	// JSON is the slice-based tool interchange document, or empty when
	// Diagnostics contains an error.
	JSON        string      `json:"json"`
	Warnings    []string    `json:"warnings,omitempty"`
	Diagnostics Diagnostics `json:"diagnostics"`
}

// Export validates one source document and converts it to slice-based Event
// Modeling tool JSON. Warnings disclose native data the interchange
// representation cannot carry.
func Export(filename string, source []byte) ExportResult {
	return ExportFiles(filename, []syntax.File{{Name: filename, Source: source}})
}

// ExportFiles validates the model made of files under the valid profile and
// converts it to slice-based Event Modeling tool JSON. name identifies the
// model in export errors. Warnings disclose native data the interchange
// representation cannot carry.
func ExportFiles(name string, files []syntax.File) ExportResult {
	built, diagnostics := ValidatedModelFiles(name, files, Valid)
	result := ExportResult{Diagnostics: diagnostics}
	if diagnostics.HasErrors() {
		return result
	}
	document, warnings, err := interchange.Export(built)
	result.Warnings = warnings
	if err != nil {
		result.Diagnostics = append(result.Diagnostics, Diagnostic{Severity: "Error", Summary: "Failed to export JSON", Detail: fmt.Sprintf("%s: %v", name, err)})
		return result
	}
	encoded, err := interchange.MarshalDocument(document)
	if err != nil {
		result.Diagnostics = append(result.Diagnostics, Diagnostic{Severity: "Error", Summary: "Failed to encode JSON", Detail: fmt.Sprintf("%s: %v", name, err)})
		return result
	}
	result.JSON = string(encoded)
	result.Warnings = warnings
	return result
}

// ExportPath reads and exports the model at path, a file or a folder.
func ExportPath(path string) ExportResult {
	files, diagnostics := ReadModel(path)
	if diagnostics.HasErrors() {
		return ExportResult{Diagnostics: diagnostics}
	}
	return ExportFiles(path, files)
}

// ImportResult is the output of Import.
type ImportResult struct {
	// Source is the formatted .em.hcl document. It is populated after decoding
	// and formatting succeed, even if native validation reports errors.
	Source      string      `json:"source"`
	Warnings    []string    `json:"warnings,omitempty"`
	Diagnostics Diagnostics `json:"diagnostics"`
}

// Import converts a real Event Modeling tool export into formatted native HCL
// and validates it under the valid profile. Warnings disclose unsupported
// source data; outputName names the generated document in diagnostics.
func Import(filename string, data []byte, outputName string) ImportResult {
	document, err := interchange.ParseDocument(data)
	if err != nil {
		return ImportResult{Diagnostics: Diagnostics{{Severity: "Error", Summary: "Failed to import JSON", Detail: fmt.Sprintf("%s: %v", filename, err)}}}
	}
	generated, warnings, err := interchange.Import(document)
	if err != nil {
		return ImportResult{Warnings: warnings, Diagnostics: Diagnostics{{Severity: "Error", Summary: "Failed to import JSON", Detail: fmt.Sprintf("%s: %v", filename, err)}}}
	}
	formatted := Format(outputName, generated)
	if formatted.Diagnostics.HasErrors() {
		return ImportResult{Warnings: warnings, Diagnostics: formatted.Diagnostics}
	}
	return ImportResult{Source: formatted.Source, Warnings: warnings, Diagnostics: Validate(outputName, []byte(formatted.Source), Valid)}
}

// ImportFile reads and imports one interchange JSON document.
func ImportFile(path, outputName string) ImportResult {
	data, diagnostics := readFile(path)
	if diagnostics.HasErrors() {
		return ImportResult{Diagnostics: diagnostics}
	}
	return Import(path, data, outputName)
}

// ParseProfile converts a profile name ("workshop", "valid", "strict") into
// its typed Profile, so callers such as cmd/wasm and internal/serve need not
// import internal/validator themselves just to accept a profile as a plain
// string. Reports false, with Valid, for any other input.
func ParseProfile(value string) (Profile, bool) {
	return validator.ParseProfile(value)
}

// Diagnostic is a plain, serialization-friendly rendering of an HCL
// diagnostic: no interfaces or pointers into HCL internals, so it marshals
// cleanly to JSON and to a JavaScript value from within WebAssembly.
type Diagnostic struct {
	Code     string `json:"code"`
	Severity string `json:"severity"`
	Summary  string `json:"summary"`
	Detail   string `json:"detail"`
	Line     int    `json:"line"`
	Column   int    `json:"column"`

	// Filename is the source file the diagnostic points at. The CLI prints it
	// as "file:line:column: ...", serve shows it on the diagnostics page, and
	// cmd/wasm passes it to JavaScript as "file". JSON marshaling omits it
	// (json:"-").
	Filename string `json:"-"`
}

// RenderResult is the output of Render.
type RenderResult struct {
	// HTML is the self-contained interactive canvas document — the same
	// bytes `emhcl diagram` would write to a file — or empty
	// when Diagnostics contains an error.
	HTML        string      `json:"html"`
	Diagnostics Diagnostics `json:"diagnostics"`
}

// Render parses and validates one source document under profile and, if it
// contains no errors, renders it to a self-contained interactive HTML canvas.
// This mirrors the diagram command: a model with errors is refused and no HTML
// is produced; modeling-smell warnings are reported but do not block
// rendering. source is parsed exactly once, whether or not it validates.
func Render(filename string, source []byte, profile Profile) RenderResult {
	return RenderFiles(filename, []syntax.File{{Name: filename, Source: source}}, profile)
}

// RenderFiles parses and validates the model made of files under profile and,
// if it contains no errors, renders it to a self-contained interactive HTML
// canvas titled from name. Refusal and warning behavior match Render. Each file
// is parsed exactly once, whether or not the model validates.
func RenderFiles(name string, files []syntax.File, profile Profile) RenderResult {
	built, diagnostics := ValidatedModelFiles(name, files, profile)
	result := RenderResult{Diagnostics: diagnostics}
	if diagnostics.HasErrors() {
		return result
	}
	html, err := renderer.Render(name, built)
	if err != nil {
		result.Diagnostics = append(result.Diagnostics, Diagnostic{
			Severity: "Error",
			Summary:  "Failed to render diagram",
			Detail:   err.Error(),
			Filename: name,
		})
		return result
	}
	result.HTML = html
	return result
}

// RenderPath reads and renders the model at path, a file or a folder.
func RenderPath(path string, profile Profile) RenderResult {
	files, diagnostics := ReadModel(path)
	if diagnostics.HasErrors() {
		return RenderResult{Diagnostics: diagnostics}
	}
	return RenderFiles(path, files, profile)
}

// FormatResult is the output of Format.
type FormatResult struct {
	// Source is the canonicalized document, or empty when Diagnostics
	// contains an error.
	Source      string      `json:"source"`
	Diagnostics Diagnostics `json:"diagnostics"`
}

// Format canonicalizes source's whitespace and attribute order, the same
// transformation `emhcl fmt` applies. Formatting only requires
// source to parse as HCL; it does not run Event Modeling validation.
func Format(filename string, source []byte) FormatResult {
	formatted, diagnostics := formatter.Format(filename, source)
	return FormatResult{
		Source:      string(formatted),
		Diagnostics: toDiagnostics(diagnostics),
	}
}

// FormatFile reads and formats one Event Modeling document.
func FormatFile(path string) FormatResult {
	source, diagnostics := readFile(path)
	if diagnostics.HasErrors() {
		return FormatResult{Diagnostics: diagnostics}
	}
	return Format(path, source)
}

// Validate parses and validates one source document under profile, returning
// its diagnostics as the plain Diagnostic type. source is parsed exactly once.
func Validate(filename string, source []byte, profile Profile) Diagnostics {
	return ValidateFiles(filename, []syntax.File{{Name: filename, Source: source}}, profile)
}

// ValidateFiles parses and validates the model made of files under profile,
// returning its diagnostics as the plain Diagnostic type. Each file is parsed
// exactly once.
func ValidateFiles(name string, files []syntax.File, profile Profile) Diagnostics {
	_, diagnostics := validateFiles(files, profile)
	return diagnostics
}

// ValidatePath reads and validates the model at path, a file or a folder.
func ValidatePath(path string, profile Profile) Diagnostics {
	files, diagnostics := ReadModel(path)
	if diagnostics.HasErrors() {
		return diagnostics
	}
	return ValidateFiles(path, files, profile)
}

// Diagnostics is an ordered collection of application diagnostics.
type Diagnostics []Diagnostic

// HasErrors reports whether at least one diagnostic has error severity.
func (d Diagnostics) HasErrors() bool {
	for _, diagnostic := range d {
		if diagnostic.Severity == "Error" {
			return true
		}
	}
	return false
}

// ReadModel loads the member files of the model at path. A path to a regular
// file is a one-file model. A path to a folder is a folder model made of every
// regular file directly in it whose name ends in ".em.hcl" and does not start
// with "."; symlinks that resolve to regular files count. Subfolders, other
// files and entries that resolve to a directory or a device are ignored. An
// entry with a member name that cannot be inspected, such as a broken symlink
// or a file without permission, is an EM001 error, because skipping it would
// silently give a partial model. Members are returned in file name order,
// compared byte by byte, which is the order os.ReadDir yields. Each file is
// named by joining path and its entry name. A folder without members is an
// EM001 error.
func ReadModel(path string) ([]syntax.File, Diagnostics) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, readFailure(path)
	}
	if !info.IsDir() {
		source, diagnostics := readFile(path)
		if diagnostics.HasErrors() {
			return nil, diagnostics
		}
		return []syntax.File{{Name: path, Source: source}}, nil
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, readFailure(path)
	}
	var files []syntax.File
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasSuffix(name, ".em.hcl") || strings.HasPrefix(name, ".") {
			continue
		}
		member := filepath.Join(path, name)
		info, err := os.Stat(member)
		if err != nil {
			return nil, readFailure(member)
		}
		if !info.Mode().IsRegular() {
			continue
		}
		source, diagnostics := readFile(member)
		if diagnostics.HasErrors() {
			return nil, diagnostics
		}
		files = append(files, syntax.File{Name: member, Source: source})
	}
	if len(files) == 0 {
		return nil, Diagnostics{{
			Code:     "EM001",
			Severity: "Error",
			Summary:  "Failed to read model",
			Detail:   fmt.Sprintf("folder %s has no .em.hcl files", path),
		}}
	}
	return files, nil
}

// readFile reads one file, reporting a failure as an EM001 diagnostic.
func readFile(path string) ([]byte, Diagnostics) {
	source, err := os.ReadFile(path)
	if err != nil {
		return nil, readFailure(path)
	}
	return source, nil
}

func readFailure(path string) Diagnostics {
	return Diagnostics{{
		Code:     "EM001",
		Severity: "Error",
		Summary:  "Failed to read file",
		Detail:   fmt.Sprintf("The configuration file %q could not be read.", path),
	}}
}

// ValidatedModel parses, decodes, validates, and builds one source document
// exactly once. Invalid input returns a nil model and the diagnostics that
// rejected it.
func ValidatedModel(filename string, input []byte, profile Profile) (*model.Model, Diagnostics) {
	return ValidatedModelFiles(filename, []syntax.File{{Name: filename, Source: input}}, profile)
}

// ValidatedModelFiles parses, decodes, validates, and builds the model made of
// files exactly once. files are in model order. Invalid input returns a nil
// model and the diagnostics that rejected it.
func ValidatedModelFiles(name string, files []syntax.File, profile Profile) (*model.Model, Diagnostics) {
	validated, diagnostics := validateFiles(files, profile)
	if diagnostics.HasErrors() {
		return nil, diagnostics
	}
	return model.Build(validated), diagnostics
}

func validateFiles(files []syntax.File, profile Profile) (*validator.ValidatedDocument, Diagnostics) {
	doc, parseDiagnostics := syntax.ParseFiles(files)
	if parseDiagnostics.HasErrors() {
		return nil, toDiagnostics(parseDiagnostics)
	}
	validated, validationDiagnostics := validator.ValidateDecodedDocument(sourcepkg.Decode(doc), profile)
	return validated, toDiagnostics(validationDiagnostics)
}

// toDiagnostics converts HCL diagnostics to the plain application contract.
func toDiagnostics(diagnostics hcl.Diagnostics) Diagnostics {
	if len(diagnostics) == 0 {
		return nil
	}
	converted := make(Diagnostics, len(diagnostics))
	for index, diagnostic := range diagnostics {
		converted[index] = toDiagnostic(diagnostic)
	}
	return converted
}

// toDiagnostic converts a single *hcl.Diagnostic, extracting the stable
// EMxxx code and the source position the same way the CLI's own
// formatDiagnostic does (internal/cli/cli.go), so a diagnostic
// looks identical whether it reached you via the terminal, the browser
// editor, or `serve`.
func toDiagnostic(diagnostic *hcl.Diagnostic) Diagnostic {
	severity := ""
	switch diagnostic.Severity {
	case hcl.DiagInvalid:
		severity = "Info"
	case hcl.DiagError:
		severity = "Error"
	case hcl.DiagWarning:
		severity = "Warning"
	}

	converted := Diagnostic{
		Code:     validator.DiagnosticCode(diagnostic),
		Severity: severity,
		Summary:  diagnostic.Summary,
		Detail:   diagnostic.Detail,
	}
	if diagnostic.Subject != nil {
		converted.Line = diagnostic.Subject.Start.Line
		converted.Column = diagnostic.Subject.Start.Column
		converted.Filename = diagnostic.Subject.Filename
	}
	return converted
}
