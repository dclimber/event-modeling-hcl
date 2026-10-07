// Package validator implements the strict validator for the native HCL Event
// Modeling Specification defined in eventmodeling.hclspec.md.
package validator

import (
	"github.com/hashicorp/hcl/v2"
	"github.com/zclconf/go-cty/cty"

	"github.com/event-modeling-hcl/eventmodeling-hcl/internal/source"
	"github.com/event-modeling-hcl/eventmodeling-hcl/internal/syntax"
)

// ValidatedDocument is the capability required to construct the canonical
// model. Its source cannot be supplied without first passing validation.
type ValidatedDocument struct {
	source *source.Document
}

// Source returns the decoded source document that passed validation.
func (d *ValidatedDocument) Source() *source.Document {
	return d.source
}

// ValidateSource parses and validates one in-memory Event Modeling document.
func ValidateSource(filename string, source []byte) hcl.Diagnostics {
	return ValidateSourceWithProfile(filename, source, Valid)
}

// ValidateSourceWithProfile parses and validates one in-memory Event Modeling
// document using the requested validation profile.
func ValidateSourceWithProfile(filename string, source []byte, profile Profile) hcl.Diagnostics {
	document, diagnostics := syntax.Parse(filename, source)
	if diagnostics.HasErrors() {
		return applyProfile(diagnostics, profile)
	}
	return ValidateDocument(document, profile)
}

// ValidateDocument validates an already-parsed Document using the requested
// validation profile. Callers that need to both validate and further process
// a document (for example, building it into internal/model's canonical
// Model) can parse once with syntax.Parse and pass the result here, instead
// of parsing again through ValidateSourceWithProfile.
func ValidateDocument(doc *syntax.Document, profile Profile) hcl.Diagnostics {
	_, diagnostics := ValidateDecodedDocument(source.Decode(doc), profile)
	return diagnostics
}

// ValidateDecodedDocument validates decoded source and returns a construction
// capability only when the selected profile contains no error diagnostics.
func ValidateDecodedDocument(doc *source.Document, profile Profile) (*ValidatedDocument, hcl.Diagnostics) {
	diagnostics := applyProfile(validateBody(doc), profile)
	if diagnostics.HasErrors() {
		return nil, diagnostics
	}
	return &ValidatedDocument{source: doc}, diagnostics
}

type fieldTypeRef struct {
	fieldType   string
	cardinality string
}

type workflowIndex struct {
	elements map[string]map[string]bool
}

type modelIndex struct {
	actors           map[string]bool
	aggregates       map[string]bool
	boundedContexts  map[string]bool
	catalogEvents    map[string]bool
	fieldTypes       map[string]fieldTypeRef
	chapters         map[string]bool
	hotspots         map[string]bool
	systems          map[string]bool
	teams            map[string]bool
	workflows        map[string]workflowIndex
	workflowOrder    []string
	scenarioCounts   map[string]int
	definitionRanges map[string]hcl.Range
	declarations     map[string]hcl.Range
	externalContexts map[string]bool
}

type modelValidator struct {
	index    modelIndex
	document *source.Document
}

type contextValidator struct {
	model     *modelValidator
	contextID string
}

// validateBody runs every validation pass over a parsed document body: it
// builds the symbol index (index.go), then the structural/block-eligibility
// checks per top-level block (structure.go, which in turn covers scenarios
// in scenarios.go and per-workflow smells in smells.go), and finally the
// whole-model shelf-anti-pattern smell check.
func validateBody(document *source.Document) hcl.Diagnostics {
	index, diagnostics := buildIndex(document)
	validator := &modelValidator{index: index, document: document}
	content, contentDiagnostics := syntax.Content(document.Parsed().Body(), syntax.ModelSchema())
	diagnostics = append(diagnostics, contentDiagnostics...)
	for _, block := range content.Blocks {
		switch block.Type {
		case "bounded_context":
			diagnostics = append(diagnostics, validator.validateBoundedContext(block)...)
		case "actor":
			diagnostics = append(diagnostics, validator.validateActor(block)...)
		case "team", "system":
			diagnostics = append(diagnostics, validator.validateOwner(block)...)
		case "chapter":
			diagnostics = append(diagnostics, validator.validateChapter(block)...)
		case "hotspot":
			diagnostics = append(diagnostics, validator.validateHotspot(block)...)
		case "state_change", "state_view", "automation", "translation":
			diagnostics = append(diagnostics, validator.validateWorkflow(block)...)
		}
	}
	if document.Parsed().FileCount() > 1 {
		diagnostics = append(diagnostics, validator.validateComposition(content.Blocks)...)
	}
	diagnostics = append(diagnostics, validator.validateShelfSmell()...)
	return diagnostics
}

func literalString(attribute *hcl.Attribute) (string, bool) {
	if attribute == nil {
		return "", false
	}
	value, diagnostics := attribute.Expr.Value(nil)
	if diagnostics.HasErrors() || value.IsNull() || !value.Type().Equals(cty.String) {
		return "", false
	}
	return value.AsString(), true
}

func literalBool(attribute *hcl.Attribute) (bool, bool) {
	if attribute == nil {
		return false, false
	}
	value, diagnostics := attribute.Expr.Value(nil)
	if diagnostics.HasErrors() || value.IsNull() || !value.Type().Equals(cty.Bool) {
		return false, false
	}
	return value.True(), true
}
