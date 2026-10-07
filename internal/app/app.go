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

// Export validates source and converts it to slice-based Event Modeling tool
// JSON. Warnings disclose native data the interchange representation cannot carry.
func Export(filename string, source []byte) ExportResult {
	built, diagnostics := ValidatedModel(filename, source, Valid)
	result := ExportResult{Diagnostics: diagnostics}
	if diagnostics.HasErrors() {
		return result
	}
	document, warnings, err := interchange.Export(built)
	result.Warnings = warnings
	if err != nil {
		result.Diagnostics = append(result.Diagnostics, Diagnostic{Severity: "Error", Summary: "Failed to export JSON", Detail: fmt.Sprintf("%s: %v", filename, err)})
		return result
	}
	encoded, err := interchange.MarshalDocument(document)
	if err != nil {
		result.Diagnostics = append(result.Diagnostics, Diagnostic{Severity: "Error", Summary: "Failed to encode JSON", Detail: fmt.Sprintf("%s: %v", filename, err)})
		return result
	}
	result.JSON = string(encoded)
	result.Warnings = warnings
	return result
}

// ExportFile reads and exports one Event Modeling document.
func ExportFile(path string) ExportResult {
	source, diagnostics := readSource(path)
	if diagnostics.HasErrors() {
		return ExportResult{Diagnostics: diagnostics}
	}
	return Export(path, source)
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
	data, diagnostics := readSource(path)
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

	// Filename is the source file the diagnostic points at, used by the CLI
	// to print "file:line:column: ..." text lines. It is not part of the
	// JSON/JS contract (json:"-"): callers that marshal Diagnostic to JSON
	// (or, in cmd/wasm, build the JS object field-by-field) never see it,
	// so adding it here changes no observable output.
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

// Render parses and validates source under profile and, if it contains no
// errors, renders it to a self-contained interactive HTML canvas. This
// mirrors the diagram command: a model with errors is refused and no HTML
// is produced; modeling-smell warnings are reported but do not block
// rendering. source is parsed exactly once, whether or not it validates.
func Render(filename string, source []byte, profile Profile) RenderResult {
	built, diagnostics := ValidatedModel(filename, source, profile)
	result := RenderResult{Diagnostics: diagnostics}
	if diagnostics.HasErrors() {
		return result
	}
	html, err := renderer.Render(filename, built)
	if err != nil {
		result.Diagnostics = append(result.Diagnostics, Diagnostic{
			Severity: "Error",
			Summary:  "Failed to render diagram",
			Detail:   err.Error(),
			Filename: filename,
		})
		return result
	}
	result.HTML = html
	return result
}

// RenderFile reads and renders one Event Modeling document.
func RenderFile(path string, profile Profile) RenderResult {
	source, diagnostics := readSource(path)
	if diagnostics.HasErrors() {
		return RenderResult{Diagnostics: diagnostics}
	}
	return Render(path, source, profile)
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
	source, diagnostics := readSource(path)
	if diagnostics.HasErrors() {
		return FormatResult{Diagnostics: diagnostics}
	}
	return Format(path, source)
}

// Validate parses and validates source under profile, returning its
// diagnostics as the plain Diagnostic type. source is parsed exactly once.
func Validate(filename string, source []byte, profile Profile) Diagnostics {
	_, diagnostics := validateSource(filename, source, profile)
	return diagnostics
}

// ValidateFile reads and validates one Event Modeling document.
func ValidateFile(path string, profile Profile) Diagnostics {
	source, diagnostics := readSource(path)
	if diagnostics.HasErrors() {
		return diagnostics
	}
	return Validate(path, source, profile)
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

func readSource(path string) ([]byte, Diagnostics) {
	source, err := os.ReadFile(path)
	if err == nil {
		return source, nil
	}
	return nil, Diagnostics{{
		Code:     "EM001",
		Severity: "Error",
		Summary:  "Failed to read file",
		Detail:   fmt.Sprintf("The configuration file %q could not be read.", path),
	}}
}

// ValidatedModel parses, decodes, validates, and builds source exactly once.
// Invalid input returns a nil model and the diagnostics that rejected it.
func ValidatedModel(filename string, input []byte, profile Profile) (*model.Model, Diagnostics) {
	validated, diagnostics := validateSource(filename, input, profile)
	if diagnostics.HasErrors() {
		return nil, diagnostics
	}
	return model.Build(validated), diagnostics
}

func validateSource(filename string, input []byte, profile Profile) (*validator.ValidatedDocument, Diagnostics) {
	doc, parseDiagnostics := syntax.Parse(filename, input)
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
