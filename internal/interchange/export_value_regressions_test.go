package interchange_test

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/event-modeling-hcl/eventmodeling-hcl/internal/app"
	"github.com/event-modeling-hcl/eventmodeling-hcl/internal/interchange"
	"github.com/event-modeling-hcl/eventmodeling-hcl/internal/model"
)

func TestExportRejectsDistinctScreenActors(t *testing.T) {
	m := &model.Model{Actors: []model.Actor{{ID: "a", Title: "A"}, {ID: "b", Title: "B"}}, Workflows: []model.Workflow{{ID: "w", Kind: model.StateChange, Elements: []model.Element{{ID: "a", Kind: model.Screen, Semantic: model.Semantic{Actor: "actor.a"}}, {ID: "b", Kind: model.Screen, Semantic: model.Semantic{Actor: "actor.b"}}}}}}
	_, _, err := interchange.Export(m)
	if err == nil || !strings.Contains(err.Error(), "actors") {
		t.Fatalf("expected ambiguous actors error, got %v", err)
	}
}

func TestExportStopsRecursiveFieldTypeExpansion(t *testing.T) {
	m := &model.Model{Contexts: []model.Context{{ID: "c", FieldTypes: []model.FieldType{{ID: "node", Type: "Object", Fields: []model.Field{{Name: "child", Type: "field_type.node"}}}}}}, Workflows: []model.Workflow{{ID: "w", Kind: model.StateChange, Elements: []model.Element{{ID: "c", Kind: model.Command, Fields: []model.Field{{Name: "root", Type: "field_type.c.node"}}}}}}}
	doc, warnings, err := interchange.Export(m)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(strings.Join(warnings, "\n"), "cyclic") {
		t.Fatalf("missing cycle warning: %v", warnings)
	}
	if len(doc.Slices[0].Commands[0].Fields[0].Subfields) != 1 {
		t.Fatal("noncyclic parent field lost")
	}
}

func TestExportProducerlessEventsAreNotCopies(t *testing.T) {
	m := &model.Model{Contexts: []model.Context{{ID: "c", External: true, Events: []model.Event{{ID: "e", Title: "E"}}}}, Workflows: []model.Workflow{{ID: "w", Kind: model.StateView, Scenarios: []model.Scenario{{ID: "s", Steps: []model.Step{{Kind: model.Given, Target: "event", Ref: "event.c.e"}}}}}}}
	doc, _, err := interchange.Export(m)
	if err != nil {
		t.Fatal(err)
	}
	e := doc.Slices[0].Events[0]
	if e.ElementCopy || e.LinkedID != "" {
		t.Fatalf("producerless event became self-copy: %#v", e)
	}
}

func TestExportSpecificationLinksSliceAndDisclosesLosses(t *testing.T) {
	m := &model.Model{Contexts: []model.Context{{ID: "unused", Title: "unused", TitleExplicit: true}, {ID: "c", Title: "c", TitleExplicit: true}}, Workflows: []model.Workflow{{ID: "w", Owner: "bounded_context.c", Kind: model.StateChange, Scenarios: []model.Scenario{{ID: "s", Steps: []model.Step{{Kind: model.When, Target: "command", Ref: "command.c"}, {Kind: model.Then, Target: "error", Title: "Separate title", Error: "Failure"}}}}}}}
	doc, warnings, err := interchange.Export(m)
	if err != nil {
		t.Fatal(err)
	}
	if doc.Slices[0].Specifications[0].LinkedID != "workflow.w" {
		t.Fatal("specification linkedId must name slice")
	}
	for _, want := range []string{"bounded_context.unused", "bounded_context.c", "Separate title"} {
		if !strings.Contains(strings.Join(warnings, "\n"), want) {
			t.Fatalf("missing %q warning: %v", want, warnings)
		}
	}
}

func TestAppImportExportPreservesLiteralJSONKeysAndValues(t *testing.T) {
	value := `{"e\u0301":{"é":1,"e\u0301":2,"nested":[{},[],null,{"empty":""}]},"é":3}`
	command := `{"id":"command.w.c","title":"C","type":"COMMAND","fields":[{"name":"payload","type":"Custom","example":` + value + `},{"name":"text","type":"String","example":"null"}],"dependencies":[],"prototype":` + value + `}`
	input := sliceDocument("0", command, "", "")
	imported := app.Import("literal.json", []byte(input), "literal.em.hcl")
	if imported.Diagnostics.HasErrors() {
		t.Fatalf("import: %#v\n%s", imported.Diagnostics, imported.Source)
	}
	exported := app.Export("literal.em.hcl", []byte(imported.Source))
	if exported.Diagnostics.HasErrors() {
		t.Fatalf("export: %#v", exported.Diagnostics)
	}
	doc := parsed(t, exported.JSON)
	var want, got any
	if err := json.Unmarshal([]byte(value), &want); err != nil {
		t.Fatal(err)
	}
	for _, raw := range []json.RawMessage{doc.Slices[0].Commands[0].Fields[0].Example, doc.Slices[0].Commands[0].Prototype} {
		if err := json.Unmarshal(raw, &got); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(want, got) {
			t.Fatalf("literal JSON changed: got %s, want %s", raw, value)
		}
	}
	if string(doc.Slices[0].Commands[0].Fields[1].Example) != `"null"` {
		t.Fatal("String null changed")
	}
}
