package model

import (
	"encoding/json"
	"testing"

	sourcepkg "github.com/event-modeling-hcl/eventmodeling-hcl/internal/source"
	"github.com/event-modeling-hcl/eventmodeling-hcl/internal/syntax"
	"github.com/event-modeling-hcl/eventmodeling-hcl/internal/validator"
)

// buildFiles follows the production parse, decode, validate, and build
// sequence for a model made of several files. Only validation errors block the
// build; warnings such as an unchaptered workflow are allowed.
func buildFiles(t *testing.T, files []syntax.File) *Model {
	t.Helper()
	doc, diagnostics := syntax.ParseFiles(files)
	if diagnostics.HasErrors() {
		t.Fatalf("syntax.ParseFiles diagnostics = %s", diagnostics.Error())
	}
	validated, validationDiagnostics := validator.ValidateDecodedDocument(sourcepkg.Decode(doc), validator.Valid)
	if validationDiagnostics.HasErrors() {
		t.Fatalf("validation diagnostics = %s", validationDiagnostics.Error())
	}
	return Build(validated)
}

func workflowIDs(built *Model) []string {
	ids := make([]string, 0, len(built.Workflows))
	for _, workflow := range built.Workflows {
		ids = append(ids, workflow.ID)
	}
	return ids
}

func TestBuild_MultiFileOrdersWorkflowsByChaptersThenModelOrder(t *testing.T) {
	built := buildFiles(t, []syntax.File{
		{Name: "model/00-chapters.em.hcl", Source: []byte(`bounded_context "shop" {
  event "happened" {}
}

chapter "first" {
  workflows = [workflow.w_c, workflow.w_a]
}

chapter "second" {
  workflows = [workflow.w_b]
}
`)},
		{Name: "model/10-first.em.hcl", Source: []byte(`state_change "w_a" {
  command "a_cmd" {
    external_trigger = true
    to               = [event.shop.happened]
  }
}

state_change "w_u1" {}
`)},
		{Name: "model/20-second.em.hcl", Source: []byte(`state_change "w_b" {}

state_change "w_c" {
  command "c_cmd" {
    external_trigger = true
    to               = [event.shop.happened]
  }
}

state_change "w_u2" {}
`)},
	})

	if got, want := workflowIDs(built), []string{"w_c", "w_a", "w_b", "w_u1", "w_u2"}; !sameStrings(got, want) {
		t.Fatalf("workflow order = %v, want %v", got, want)
	}
	if got, want := len(built.Edges), 2; got != want {
		t.Fatalf("edge count = %d, want %d", got, want)
	}
	if got, want := built.Edges[0].WorkflowID, "w_c"; got != want {
		t.Fatalf("first edge workflow = %q, want %q", got, want)
	}
	if got, want := built.Edges[1].WorkflowID, "w_a"; got != want {
		t.Fatalf("second edge workflow = %q, want %q", got, want)
	}
}

func TestBuild_MultiFileWithoutChaptersKeepsModelOrder(t *testing.T) {
	built := buildFiles(t, []syntax.File{
		{Name: "model/a.em.hcl", Source: []byte(`state_change "w_a" {}
state_change "w_b" {}
`)},
		{Name: "model/b.em.hcl", Source: []byte(`state_change "w_c" {}
`)},
	})

	if got, want := workflowIDs(built), []string{"w_a", "w_b", "w_c"}; !sameStrings(got, want) {
		t.Fatalf("workflow order = %v, want %v", got, want)
	}
}

func TestBuild_OneFileKeepsSourceOrderWithUnchapteredWorkflow(t *testing.T) {
	built := buildFixture(t, "model.em.hcl", []byte(`state_change "w_u" {}
state_change "w_a" {}
state_change "w_b" {}

chapter "main" {
  workflows = [workflow.w_a, workflow.w_b]
}
`))

	if got, want := workflowIDs(built), []string{"w_u", "w_a", "w_b"}; !sameStrings(got, want) {
		t.Fatalf("workflow order = %v, want %v", got, want)
	}
}

func TestBuild_MultiFileDecodesQuotedUnicodeKeysFromTheirOwnFile(t *testing.T) {
	// The key is written with a combining accent. HCL evaluation would
	// normalize it, so the exact spelling can only come from the source bytes
	// of the file that holds the expression.
	const key = "e\u0301clair"
	built := buildFiles(t, []syntax.File{
		{Name: "model/a.em.hcl", Source: []byte(`bounded_context "shop" {
  event "happened" {}
}

state_change "w_a" {}
state_change "w_b" {}
`)},
		{Name: "model/b.em.hcl", Source: []byte(`state_change "w_c" {
  command "c_cmd" {
    external_trigger = true
    prototype        = { "` + key + `" = 1 }
    to               = [event.shop.happened]
  }
}
`)},
	})

	data := workflowByID(t, built, "w_c").Elements[0].Presentation.PrototypeData
	var decoded map[string]int
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal prototype data %q: %v", data, err)
	}
	if value, ok := decoded[key]; !ok || value != 1 {
		t.Fatalf("prototype data = %q, want exact key %q with value 1", data, key)
	}
}
