package validator

import (
	"strings"
	"testing"

	"github.com/hashicorp/hcl/v2"

	"github.com/event-modeling-hcl/eventmodeling-hcl/internal/source"
	"github.com/event-modeling-hcl/eventmodeling-hcl/internal/syntax"
)

func validateFiles(t *testing.T, profile Profile, files ...syntax.File) hcl.Diagnostics {
	t.Helper()
	document, parseDiagnostics := syntax.ParseFiles(files)
	if parseDiagnostics.HasErrors() {
		t.Fatalf("ParseFiles diagnostics = %s, want none", parseDiagnostics.Error())
	}
	_, diagnostics := ValidateDecodedDocument(source.Decode(document), profile)
	return diagnostics
}

func diagnosticsWithCode(diagnostics hcl.Diagnostics, code string) []*hcl.Diagnostic {
	var matches []*hcl.Diagnostic
	for _, diagnostic := range diagnostics {
		if DiagnosticCode(diagnostic) == code {
			matches = append(matches, diagnostic)
		}
	}
	return matches
}

func requireOneDiagnostic(t *testing.T, diagnostics hcl.Diagnostics, code string, severity hcl.DiagnosticSeverity, filename string) *hcl.Diagnostic {
	t.Helper()
	matches := diagnosticsWithCode(diagnostics, code)
	if len(matches) != 1 {
		t.Fatalf("%s diagnostics = %d in %#v, want exactly 1", code, len(matches), diagnostics)
	}
	match := matches[0]
	if match.Severity != severity {
		t.Fatalf("%s severity = %v, want %v", code, match.Severity, severity)
	}
	if match.Subject == nil || match.Subject.Filename != filename {
		t.Fatalf("%s subject = %v, want file %s", code, match.Subject, filename)
	}
	return match
}

func requireNoDiagnosticWithCode(t *testing.T, diagnostics hcl.Diagnostics, code string) {
	t.Helper()
	if matches := diagnosticsWithCode(diagnostics, code); len(matches) != 0 {
		t.Fatalf("%s diagnostics = %#v, want none", code, matches)
	}
}

func modelFile(name, content string) syntax.File {
	return syntax.File{Name: name, Source: []byte(content)}
}

func TestValidateFiles_ResolvesReferencesAcrossFiles(t *testing.T) {
	diagnostics := validateFiles(t, Valid,
		modelFile("a.em.hcl", `chapter "pets" {
  workflows = [workflow.add_pet]
}
`),
		modelFile("b.em.hcl", `state_change "add_pet" { title = "Add Pet" }
`),
	)

	requireNoDiagnosticWithCode(t, diagnostics, "EM102")
	requireNoErrors(t, diagnostics)
}

func TestValidateFiles_ReportsDuplicateWorkflowAcrossFilesWithFirstLocation(t *testing.T) {
	diagnostics := validateFiles(t, Valid,
		modelFile("a.em.hcl", "state_change \"other\" { title = \"Other\" }\nstate_change \"add_pet\" { title = \"Add Pet\" }\n"),
		modelFile("b.em.hcl", "state_change \"add_pet\" { title = \"Add Pet\" }\n"),
	)

	duplicate := requireOneDiagnostic(t, diagnostics, "EM002", hcl.DiagError, "b.em.hcl")
	want := `workflow "add_pet" is declared more than once in its namespace. First declared at a.em.hcl:2:1.`
	if duplicate.Detail != want {
		t.Fatalf("EM002 detail = %q, want %q", duplicate.Detail, want)
	}
}

func TestValidateFiles_ReportsDuplicateCatalogIDAcrossFiles(t *testing.T) {
	diagnostics := validateFiles(t, Valid,
		modelFile("a.em.hcl", "actor \"customer\" { auth_required = true }\n"),
		modelFile("b.em.hcl", "bounded_context \"clinic\" { title = \"Clinic\" }\nactor \"customer\" { auth_required = false }\n"),
	)

	duplicate := requireOneDiagnostic(t, diagnostics, "EM002", hcl.DiagError, "b.em.hcl")
	if !strings.HasSuffix(duplicate.Detail, "First declared at a.em.hcl:1:1.") {
		t.Fatalf("EM002 detail = %q, want first declaration a.em.hcl:1:1", duplicate.Detail)
	}
}

func TestValidateSource_DuplicateIDNamesFirstDeclarationInSameFile(t *testing.T) {
	model := "state_change \"add_pet\" { title = \"Add Pet\" }\nstate_change \"add_pet\" { title = \"Again\" }\n"

	duplicate := requireOneDiagnostic(t, validateModel(t, model), "EM002", hcl.DiagError, "model.em.hcl")
	want := `workflow "add_pet" is declared more than once in its namespace. First declared at model.em.hcl:1:1.`
	if duplicate.Detail != want {
		t.Fatalf("EM002 detail = %q, want %q", duplicate.Detail, want)
	}
}

func TestValidateFiles_ReportsChaptersInSeveralFiles(t *testing.T) {
	diagnostics := validateFiles(t, Valid,
		modelFile("a.em.hcl", `state_change "first" { title = "First" }
chapter "one" {
  workflows = [workflow.first]
}
`),
		modelFile("b.em.hcl", `state_change "second" { title = "Second" }
chapter "two" {
  workflows = [workflow.second]
}
`),
	)

	chapters := requireOneDiagnostic(t, diagnostics, "EM013", hcl.DiagError, "b.em.hcl")
	if want := "all chapter blocks of a multi-file model must be in one file; first chapter file is a.em.hcl."; chapters.Detail != want {
		t.Fatalf("EM013 detail = %q, want %q", chapters.Detail, want)
	}
	if chapters.Summary != "Chapters in several files" {
		t.Fatalf("EM013 summary = %q", chapters.Summary)
	}
}

func TestValidateFiles_AcceptsChaptersInOneFile(t *testing.T) {
	diagnostics := validateFiles(t, Valid,
		modelFile("a.em.hcl", `state_change "first" { title = "First" }
`),
		modelFile("b.em.hcl", `state_change "second" { title = "Second" }
chapter "one" {
  workflows = [workflow.first, workflow.second]
}
chapter "two" {
  workflows = [workflow.second]
}
`),
	)

	requireNoDiagnosticWithCode(t, diagnostics, "EM013")
}

func TestValidateFiles_ReportsWorkflowInSeveralChapters(t *testing.T) {
	diagnostics := validateFiles(t, Valid,
		modelFile("a.em.hcl", `chapter "one" {
  workflows = [workflow.add_pet]
}
chapter "two" {
  workflows = [workflow.add_pet]
}
`),
		modelFile("b.em.hcl", `state_change "add_pet" { title = "Add Pet" }
`),
	)

	several := requireOneDiagnostic(t, diagnostics, "EM014", hcl.DiagError, "a.em.hcl")
	if want := "workflow.add_pet is already in chapter.one."; several.Detail != want {
		t.Fatalf("EM014 detail = %q, want %q", several.Detail, want)
	}
	if several.Subject.Start.Line != 5 {
		t.Fatalf("EM014 subject line = %d, want 5 (the later chapter)", several.Subject.Start.Line)
	}
	if several.Summary != "Workflow in several chapters" {
		t.Fatalf("EM014 summary = %q", several.Summary)
	}
}

func TestValidateFiles_ReportsUnchapteredWorkflowPerProfile(t *testing.T) {
	files := []syntax.File{
		modelFile("a.em.hcl", `state_change "first" { title = "First" }
chapter "one" {
  workflows = [workflow.first]
}
`),
		modelFile("b.em.hcl", `state_change "second" { title = "Second" }
`),
	}
	tests := []struct {
		profile Profile
		want    hcl.DiagnosticSeverity
	}{
		{Workshop, hcl.DiagInvalid},
		{Valid, hcl.DiagWarning},
		{Strict, hcl.DiagError},
	}
	for _, test := range tests {
		t.Run(test.profile.String(), func(t *testing.T) {
			unchaptered := requireOneDiagnostic(t, validateFiles(t, test.profile, files...), "EM407", test.want, "b.em.hcl")
			want := "workflow.second is in no chapter; it is placed after chaptered workflows in file name order. Add it to a chapter."
			if unchaptered.Detail != want {
				t.Fatalf("EM407 detail = %q, want %q", unchaptered.Detail, want)
			}
			if unchaptered.Summary != "Workflow outside every chapter" {
				t.Fatalf("EM407 summary = %q", unchaptered.Summary)
			}
		})
	}
}

func TestValidateFiles_ReportsEveryWorkflowWhenModelHasNoChapters(t *testing.T) {
	diagnostics := validateFiles(t, Valid,
		modelFile("a.em.hcl", "state_change \"first\" { title = \"First\" }\n"),
		modelFile("b.em.hcl", "state_view \"second\" { title = \"Second\" }\n"),
	)

	matches := diagnosticsWithCode(diagnostics, "EM407")
	if len(matches) != 2 {
		t.Fatalf("EM407 diagnostics = %#v, want 2", matches)
	}
	for index, filename := range []string{"a.em.hcl", "b.em.hcl"} {
		if matches[index].Subject.Filename != filename || matches[index].Severity != hcl.DiagWarning {
			t.Fatalf("EM407[%d] = %v severity %v, want warning in %s", index, matches[index].Subject, matches[index].Severity, filename)
		}
	}
}

func TestValidateFiles_UnresolvedChapterReferenceDoesNotChapterAWorkflow(t *testing.T) {
	diagnostics := validateFiles(t, Valid,
		modelFile("a.em.hcl", `state_change "first" { title = "First" }
chapter "one" {
  workflows = [workflow.missing]
}
`),
		modelFile("b.em.hcl", "state_change \"second\" { title = \"Second\" }\n"),
	)

	requireOneDiagnostic(t, diagnostics, "EM102", hcl.DiagError, "a.em.hcl")
	if matches := diagnosticsWithCode(diagnostics, "EM407"); len(matches) != 2 {
		t.Fatalf("EM407 diagnostics = %#v, want both workflows reported", matches)
	}
	requireNoDiagnosticWithCode(t, diagnostics, "EM014")
}

const outOfOrderChapter = `state_change "first" { title = "First" }
state_change "second" { title = "Second" }
state_change "third" { title = "Third" }
chapter "one" {
  workflows = [workflow.third, workflow.first]
}
`

func TestValidateFiles_SkipsContiguousChapterCheckForMultiFileModels(t *testing.T) {
	diagnostics := validateFiles(t, Valid,
		modelFile("a.em.hcl", outOfOrderChapter),
		modelFile("b.em.hcl", "state_change \"fourth\" { title = \"Fourth\" }\n"),
	)

	requireNoDiagnosticWithCode(t, diagnostics, "EM006")
}

func TestValidateFiles_KeepsContiguousChapterCheckForOneFile(t *testing.T) {
	diagnostics := validateFiles(t, Valid, modelFile("a.em.hcl", outOfOrderChapter))

	requireOneDiagnostic(t, diagnostics, "EM006", hcl.DiagError, "a.em.hcl")
}

func TestValidateFiles_OneFileModelHasNoCompositionDiagnostics(t *testing.T) {
	diagnostics := validateFiles(t, Strict, modelFile("a.em.hcl", `state_change "first" { title = "First" }
state_change "second" { title = "Second" }
chapter "one" {
  workflows = [workflow.first]
}
chapter "two" {
  workflows = [workflow.first]
}
`))

	for _, code := range []string{"EM013", "EM014", "EM407"} {
		requireNoDiagnosticWithCode(t, diagnostics, code)
	}
}
