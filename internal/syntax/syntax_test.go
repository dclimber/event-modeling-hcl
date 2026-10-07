// Package syntax owns the HCL grammar for the native Event Modeling
// language: the hcl.BodySchema definitions and the single parse of a
// document's source. These tests lock in grammar acceptance, diagnostics on
// violations, and range fidelity of the decoded content.
package syntax_test

import (
	"testing"

	"github.com/hashicorp/hcl/v2"

	"github.com/event-modeling-hcl/eventmodeling-hcl/internal/syntax"
)

func TestParse_AcceptsValidDocument(t *testing.T) {
	source := `actor "clinic_staff" {
  title         = "Clinic staff"
  auth_required = true
}
`
	document, diagnostics := syntax.Parse("model.em.hcl", []byte(source))
	if diagnostics.HasErrors() {
		t.Fatalf("diagnostics = %s, want no errors", diagnostics.Error())
	}
	if document == nil {
		t.Fatal("document = nil, want non-nil Document on a valid parse")
	}
}

func TestParse_ReturnsDiagnosticsOnMalformedSyntax(t *testing.T) {
	source := `actor "clinic_staff" {
  title = "Clinic staff"
`
	document, diagnostics := syntax.Parse("model.em.hcl", []byte(source))
	if !diagnostics.HasErrors() {
		t.Fatal("diagnostics has no errors, want a syntax error for the unclosed block")
	}
	if document != nil {
		t.Fatalf("document = %#v, want nil Document on a syntax error", document)
	}
}

func TestContent_ModelSchemaRejectsUnknownTopLevelBlock(t *testing.T) {
	source := `bogus_block "x" {}`
	document, diagnostics := syntax.Parse("model.em.hcl", []byte(source))
	if diagnostics.HasErrors() {
		t.Fatalf("unexpected parse diagnostics: %s", diagnostics.Error())
	}
	_, contentDiagnostics := syntax.Content(document.Body(), syntax.ModelSchema())
	if !contentDiagnostics.HasErrors() {
		t.Fatal("content diagnostics has no errors, want an unsupported-block-type diagnostic")
	}
	found := false
	for _, diagnostic := range contentDiagnostics {
		if diagnostic.Summary == "Unsupported block type" {
			found = true
		}
	}
	if !found {
		t.Fatalf("diagnostics = %#v, want an \"Unsupported block type\" diagnostic", contentDiagnostics)
	}
}

func TestContent_ActorSchemaRequiresAuthRequired(t *testing.T) {
	source := `actor "clinic_staff" {
  title = "Clinic staff"
}
`
	document, diagnostics := syntax.Parse("model.em.hcl", []byte(source))
	if diagnostics.HasErrors() {
		t.Fatalf("unexpected parse diagnostics: %s", diagnostics.Error())
	}
	content, modelDiagnostics := syntax.Content(document.Body(), syntax.ModelSchema())
	if modelDiagnostics.HasErrors() {
		t.Fatalf("unexpected model diagnostics: %s", modelDiagnostics.Error())
	}
	if len(content.Blocks) != 1 {
		t.Fatalf("len(content.Blocks) = %d, want 1", len(content.Blocks))
	}
	_, actorDiagnostics := syntax.Content(content.Blocks[0].Body, syntax.ActorSchema())
	if !actorDiagnostics.HasErrors() {
		t.Fatal("actor diagnostics has no errors, want a missing-required-argument diagnostic for auth_required")
	}
	found := false
	for _, diagnostic := range actorDiagnostics {
		if diagnostic.Summary == "Missing required argument" {
			found = true
		}
	}
	if !found {
		t.Fatalf("diagnostics = %#v, want a \"Missing required argument\" diagnostic", actorDiagnostics)
	}
}

func TestPartialContent_IgnoresAttributesOutsideSchema(t *testing.T) {
	source := `hotspot "open_question" {
  question    = "First?"
  unsupported = "ignored by PartialContent"
}
`
	document, diagnostics := syntax.Parse("model.em.hcl", []byte(source))
	if diagnostics.HasErrors() {
		t.Fatalf("unexpected parse diagnostics: %s", diagnostics.Error())
	}
	content, _, modelDiagnostics := syntax.PartialContent(document.Body(), syntax.ModelSchema())
	if modelDiagnostics.HasErrors() {
		t.Fatalf("unexpected model diagnostics: %s", modelDiagnostics.Error())
	}
	if len(content.Blocks) != 1 {
		t.Fatalf("len(content.Blocks) = %d, want 1", len(content.Blocks))
	}
	partialContent, _, hotspotDiagnostics := syntax.PartialContent(content.Blocks[0].Body, syntax.HotspotSchema())
	if hotspotDiagnostics.HasErrors() {
		t.Fatalf("PartialContent reported diagnostics for an attribute outside its schema: %s", hotspotDiagnostics.Error())
	}
	if partialContent.Attributes["question"] == nil {
		t.Fatal("partialContent.Attributes[\"question\"] = nil, want the known attribute to still decode")
	}
	if _, unsupported := partialContent.Attributes["unsupported"]; unsupported {
		t.Fatal("partialContent.Attributes contains \"unsupported\", want only schema-known attributes")
	}
}

func TestContent_PreservesSourceRanges(t *testing.T) {
	source := `bounded_context "pet_management" {
  title = "Pet Management"
}
`
	document, diagnostics := syntax.Parse("model.em.hcl", []byte(source))
	if diagnostics.HasErrors() {
		t.Fatalf("unexpected parse diagnostics: %s", diagnostics.Error())
	}
	content, modelDiagnostics := syntax.Content(document.Body(), syntax.ModelSchema())
	if modelDiagnostics.HasErrors() {
		t.Fatalf("unexpected model diagnostics: %s", modelDiagnostics.Error())
	}
	if len(content.Blocks) != 1 {
		t.Fatalf("len(content.Blocks) = %d, want 1", len(content.Blocks))
	}
	block := content.Blocks[0]
	wantBlockStart := hcl.Pos{Line: 1, Column: 1}
	if block.DefRange.Start != wantBlockStart {
		t.Fatalf("block.DefRange.Start = %#v, want %#v", block.DefRange.Start, wantBlockStart)
	}

	contextContent, contextDiagnostics := syntax.Content(block.Body, syntax.BoundedContextSchema())
	if contextDiagnostics.HasErrors() {
		t.Fatalf("unexpected bounded_context diagnostics: %s", contextDiagnostics.Error())
	}
	title := contextContent.Attributes["title"]
	if title == nil {
		t.Fatal("contextContent.Attributes[\"title\"] = nil, want the title attribute to decode")
	}
	wantTitleLine := 2
	if title.Expr.Range().Start.Line != wantTitleLine {
		t.Fatalf("title.Expr.Range().Start.Line = %d, want %d", title.Expr.Range().Start.Line, wantTitleLine)
	}
	wantNameRangeColumn := 3
	if title.NameRange.Start.Column != wantNameRangeColumn {
		t.Fatalf("title.NameRange.Start.Column = %d, want %d", title.NameRange.Start.Column, wantNameRangeColumn)
	}
}

func twoFileFixture() []syntax.File {
	return []syntax.File{
		{Name: "model/a.em.hcl", Source: []byte("actor \"first\" {\n  auth_required = false\n}\n\nactor \"second\" {\n  auth_required = true\n}\n")},
		{Name: "model/b.em.hcl", Source: []byte("actor \"third\" {\n  auth_required = false\n}\n")},
	}
}

func TestParseFiles_MergesBlocksInFileOrder(t *testing.T) {
	document, diagnostics := syntax.ParseFiles(twoFileFixture())
	if diagnostics.HasErrors() {
		t.Fatalf("diagnostics = %s, want no errors", diagnostics.Error())
	}
	if got, want := document.FileCount(), 2; got != want {
		t.Fatalf("FileCount() = %d, want %d", got, want)
	}
	content, modelDiagnostics := syntax.Content(document.Body(), syntax.ModelSchema())
	if modelDiagnostics.HasErrors() {
		t.Fatalf("model diagnostics = %s", modelDiagnostics.Error())
	}
	var ids []string
	files := map[string]string{}
	for _, block := range content.Blocks {
		ids = append(ids, block.Labels[0])
		files[block.Labels[0]] = block.DefRange.Filename
	}
	if got, want := ids, []string{"first", "second", "third"}; len(got) != len(want) || got[0] != want[0] || got[1] != want[1] || got[2] != want[2] {
		t.Fatalf("block order = %v, want %v", got, want)
	}
	if got, want := files["third"], "model/b.em.hcl"; got != want {
		t.Fatalf("third block file = %q, want %q", got, want)
	}
}

func TestParse_IsOneFileDocument(t *testing.T) {
	document, diagnostics := syntax.Parse("model.em.hcl", []byte("actor \"first\" {\n  auth_required = false\n}\n"))
	if diagnostics.HasErrors() {
		t.Fatalf("diagnostics = %s, want no errors", diagnostics.Error())
	}
	if got, want := document.FileCount(), 1; got != want {
		t.Fatalf("FileCount() = %d, want %d", got, want)
	}
}

func TestParseFiles_NamesTheFileWithTheSyntaxError(t *testing.T) {
	files := twoFileFixture()
	files[1].Source = []byte("actor \"third\" {\n  auth_required = false\n")

	document, diagnostics := syntax.ParseFiles(files)

	if !diagnostics.HasErrors() {
		t.Fatal("diagnostics has no errors, want a syntax error for the unclosed block")
	}
	if document != nil {
		t.Fatalf("document = %#v, want nil Document on a syntax error", document)
	}
	for _, diagnostic := range diagnostics {
		if diagnostic.Subject == nil || diagnostic.Subject.Filename != "model/b.em.hcl" {
			t.Fatalf("diagnostic subject = %v, want file model/b.em.hcl", diagnostic.Subject)
		}
	}
}

func TestParseFiles_ReportsSyntaxErrorsOfEveryFile(t *testing.T) {
	files := []syntax.File{
		{Name: "a.em.hcl", Source: []byte("actor \"first\" {\n")},
		{Name: "b.em.hcl", Source: []byte("actor \"second\" {\n")},
	}

	_, diagnostics := syntax.ParseFiles(files)

	seen := map[string]bool{}
	for _, diagnostic := range diagnostics {
		if diagnostic.Subject != nil {
			seen[diagnostic.Subject.Filename] = true
		}
	}
	if !seen["a.em.hcl"] || !seen["b.em.hcl"] {
		t.Fatalf("diagnostics name files %v, want both a.em.hcl and b.em.hcl", seen)
	}
}

func TestParseFiles_RejectsRepeatedFileNames(t *testing.T) {
	files := []syntax.File{
		{Name: "a.em.hcl", Source: nil},
		{Name: "a.em.hcl", Source: []byte("actor \"second\" {\n")},
	}

	document, diagnostics := syntax.ParseFiles(files)

	if document != nil {
		t.Fatalf("document = %#v, want nil for repeated file names", document)
	}
	if len(diagnostics) != 1 {
		t.Fatalf("diagnostics = %s, want exactly one", diagnostics.Error())
	}
	diagnostic := diagnostics[0]
	if diagnostic.Severity != hcl.DiagError || diagnostic.Summary != "Duplicate file name" || diagnostic.Detail != "the model has two files named a.em.hcl." || diagnostic.Subject != nil {
		t.Fatalf("diagnostic = %#v, want an error Duplicate file name for a.em.hcl without a range", diagnostic)
	}
}

func TestSourceOf_ReturnsTheBytesOfEachFile(t *testing.T) {
	files := twoFileFixture()
	document, diagnostics := syntax.ParseFiles(files)
	if diagnostics.HasErrors() {
		t.Fatalf("diagnostics = %s, want no errors", diagnostics.Error())
	}

	for _, file := range files {
		got := document.SourceOf(hcl.Range{Filename: file.Name})
		if string(got) != string(file.Source) {
			t.Fatalf("SourceOf(%q) = %q, want %q", file.Name, got, file.Source)
		}
	}
	if got := document.SourceOf(hcl.Range{Filename: "missing.em.hcl"}); got != nil {
		t.Fatalf("SourceOf(missing) = %q, want nil", got)
	}
}
