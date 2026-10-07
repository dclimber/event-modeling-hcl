// Package syntax is the single owner of the HCL grammar for the native
// Event Modeling language. Every hcl.BodySchema describing which blocks and
// attributes the language allows lives here, and Parse performs exactly one
// HCL parse of a document's source. Later stages decode the same parsed content
// through Content/PartialContent instead of re-declaring the grammar.
package syntax

import (
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclparse"
)

// Document is one native HCL Event Modeling source file, parsed exactly
// once. It carries no interpretation of the content — callers decode
// bodies (the root body from Body, or a nested block's Body) against the
// schema functions below via Content/PartialContent.
type Document struct {
	file *hcl.File
}

// Parse parses filename's source as HCL. On a syntax error it returns a nil
// Document along with the diagnostics describing the error; callers must
// not call methods on a nil Document. On success it returns a Document
// wrapping the parsed file and no diagnostics.
func Parse(filename string, source []byte) (*Document, hcl.Diagnostics) {
	parser := hclparse.NewParser()
	file, diagnostics := parser.ParseHCL(source, filename)
	if diagnostics.HasErrors() {
		return nil, diagnostics
	}
	return &Document{file: file}, diagnostics
}

// Body returns the document's root HCL body — the body decoded against
// ModelSchema.
func (d *Document) Body() hcl.Body {
	return d.file.Body
}

// Source returns the exact bytes that were parsed. Callers use it to recover
// literal spelling that HCL's evaluated values normalize, such as the Unicode
// form of an object key.
func (d *Document) Source() []byte {
	return d.file.Bytes
}

// Content decodes body against schema, exactly like body.Content(&schema):
// every block or attribute outside the schema, and every missing Required
// attribute, is reported as a diagnostic. The returned blocks' and
// attributes' ranges point back into the original source.
func Content(body hcl.Body, schema hcl.BodySchema) (*hcl.BodyContent, hcl.Diagnostics) {
	return body.Content(&schema)
}

// PartialContent decodes body against schema, exactly like
// body.PartialContent(&schema): blocks and attributes outside the schema
// are silently omitted from the returned content (and left in the returned
// remainder body) rather than reported as diagnostics. Used where a caller
// needs to read a subset of a body's content without yet committing to
// full validation of that body.
func PartialContent(body hcl.Body, schema hcl.BodySchema) (*hcl.BodyContent, hcl.Body, hcl.Diagnostics) {
	return body.PartialContent(&schema)
}

// ModelSchema is the root schema of an Event Modeling document.
func ModelSchema() hcl.BodySchema {
	return hcl.BodySchema{Blocks: []hcl.BlockHeaderSchema{
		{Type: "bounded_context", LabelNames: []string{"id"}},
		{Type: "actor", LabelNames: []string{"id"}},
		{Type: "team", LabelNames: []string{"id"}},
		{Type: "system", LabelNames: []string{"id"}},
		{Type: "chapter", LabelNames: []string{"id"}},
		{Type: "hotspot", LabelNames: []string{"id"}},
		{Type: "state_change", LabelNames: []string{"id"}},
		{Type: "state_view", LabelNames: []string{"id"}},
		{Type: "automation", LabelNames: []string{"id"}},
		{Type: "translation", LabelNames: []string{"id"}},
	}}
}

// BoundedContextSchema is the schema of a bounded_context block's body.
func BoundedContextSchema() hcl.BodySchema {
	return hcl.BodySchema{
		Attributes: []hcl.AttributeSchema{{Name: "title"}, {Name: "description"}, {Name: "external"}, {Name: "owner"}},
		Blocks: []hcl.BlockHeaderSchema{
			{Type: "aggregate", LabelNames: []string{"name"}},
			{Type: "field_type", LabelNames: []string{"name"}},
			{Type: "event", LabelNames: []string{"id"}},
		},
	}
}

// AggregateSchema is the schema of an aggregate block's body.
func AggregateSchema() hcl.BodySchema {
	return hcl.BodySchema{Attributes: []hcl.AttributeSchema{{Name: "title"}, {Name: "description"}}}
}

// EventSchema is the schema of an event block's body.
func EventSchema() hcl.BodySchema {
	return hcl.BodySchema{
		Attributes: []hcl.AttributeSchema{
			{Name: "group_id"}, {Name: "tags"}, {Name: "title"}, {Name: "description"},
			{Name: "aggregate"}, {Name: "aggregate_dependencies"}, {Name: "service"}, {Name: "sketched"},
			{Name: "prototype"}, {Name: "list_element"}, {Name: "fields"},
		},
		Blocks: []hcl.BlockHeaderSchema{{Type: "field", LabelNames: []string{"name"}}},
	}
}

// WorkflowSchema is the schema of a state_change/state_view/automation/
// translation block's body.
func WorkflowSchema() hcl.BodySchema {
	return hcl.BodySchema{
		Attributes: []hcl.AttributeSchema{{Name: "title"}, {Name: "status"}, {Name: "description"}, {Name: "owner"}},
		Blocks: []hcl.BlockHeaderSchema{
			{Type: "command", LabelNames: []string{"id"}}, {Type: "readmodel", LabelNames: []string{"id"}},
			{Type: "screen", LabelNames: []string{"id"}}, {Type: "screen_image", LabelNames: []string{"id"}},
			{Type: "processor", LabelNames: []string{"id"}}, {Type: "table", LabelNames: []string{"id"}},
			{Type: "scenario", LabelNames: []string{"id"}},
		},
	}
}

// ElementSchema is the schema of a command/readmodel/screen/processor
// block's body. kind selects the block type so readmodel can require
// question and screen can accept actor.
func ElementSchema(kind string) hcl.BodySchema {
	attributes := []hcl.AttributeSchema{
		{Name: "group_id"}, {Name: "tags"}, {Name: "title"}, {Name: "description"},
		{Name: "aggregate"}, {Name: "aggregate_dependencies"}, {Name: "api_endpoint"}, {Name: "service"},
		{Name: "creates_aggregate"}, {Name: "external_trigger"}, {Name: "triggers"}, {Name: "sketched"}, {Name: "prototype"},
		{Name: "list_element"}, {Name: "from"}, {Name: "to"}, {Name: "fields"},
	}
	if kind == "readmodel" {
		attributes = append(attributes, hcl.AttributeSchema{Name: "question", Required: true})
	}
	if kind == "screen" {
		attributes = append(attributes, hcl.AttributeSchema{Name: "actor"})
	}
	return hcl.BodySchema{Attributes: attributes, Blocks: []hcl.BlockHeaderSchema{{Type: "field", LabelNames: []string{"name"}}}}
}

// FieldSchema is the schema of a field/subfield/field_type block's body.
func FieldSchema() hcl.BodySchema {
	return hcl.BodySchema{
		Attributes: []hcl.AttributeSchema{
			{Name: "type"}, {Name: "example"}, {Name: "mapping"}, {Name: "optional"},
			{Name: "technical_attribute"}, {Name: "generated"}, {Name: "id_attribute"}, {Name: "pii"},
			{Name: "schema"}, {Name: "cardinality"},
		},
		Blocks: []hcl.BlockHeaderSchema{{Type: "subfield", LabelNames: []string{"name"}}},
	}
}

// TableSchema is the schema of a table block's body.
func TableSchema() hcl.BodySchema {
	return hcl.BodySchema{Attributes: []hcl.AttributeSchema{{Name: "title"}, {Name: "fields"}}, Blocks: []hcl.BlockHeaderSchema{{Type: "field", LabelNames: []string{"name"}}}}
}

// ScenarioSchema is the schema of a scenario block's body.
func ScenarioSchema() hcl.BodySchema {
	return hcl.BodySchema{
		Attributes: []hcl.AttributeSchema{{Name: "title"}, {Name: "description"}},
		Blocks:     []hcl.BlockHeaderSchema{{Type: "given"}, {Type: "when"}, {Type: "then"}, {Type: "comment"}},
	}
}

// ScenarioStepSchema is the schema of a given/when/then block's body.
func ScenarioStepSchema() hcl.BodySchema {
	return hcl.BodySchema{
		Attributes: []hcl.AttributeSchema{
			{Name: "title"}, {Name: "tags"}, {Name: "examples"}, {Name: "event"}, {Name: "command"},
			{Name: "readmodel"}, {Name: "processor"}, {Name: "error"}, {Name: "expect_empty_list"}, {Name: "fields"},
		},
		Blocks: []hcl.BlockHeaderSchema{{Type: "field", LabelNames: []string{"name"}}},
	}
}

// ActorSchema is the schema of an actor block's body.
func ActorSchema() hcl.BodySchema {
	return hcl.BodySchema{Attributes: []hcl.AttributeSchema{{Name: "title"}, {Name: "auth_required", Required: true}, {Name: "description"}}}
}

// OwnerSchema is the schema of a team/system block's body. kind selects the
// block type so system can accept external.
func OwnerSchema(kind string) hcl.BodySchema {
	attributes := []hcl.AttributeSchema{{Name: "title"}, {Name: "description"}}
	if kind == "system" {
		attributes = append(attributes, hcl.AttributeSchema{Name: "external"})
	}
	return hcl.BodySchema{Attributes: attributes}
}

// ChapterSchema is the schema of a chapter block's body.
func ChapterSchema() hcl.BodySchema {
	return hcl.BodySchema{Attributes: []hcl.AttributeSchema{{Name: "title"}, {Name: "description"}, {Name: "workflows", Required: true}}}
}

// HotspotSchema is the schema of a hotspot block's body.
func HotspotSchema() hcl.BodySchema {
	return hcl.BodySchema{Attributes: []hcl.AttributeSchema{{Name: "question", Required: true}, {Name: "description"}, {Name: "on"}, {Name: "status"}}}
}

// ScreenImageSchema is the schema of a screen_image block's body.
func ScreenImageSchema() hcl.BodySchema {
	return hcl.BodySchema{Attributes: []hcl.AttributeSchema{{Name: "title"}, {Name: "url"}}}
}

// CommentSchema is the schema of a comment block's body.
func CommentSchema() hcl.BodySchema {
	return hcl.BodySchema{Attributes: []hcl.AttributeSchema{{Name: "description", Required: true}}}
}
