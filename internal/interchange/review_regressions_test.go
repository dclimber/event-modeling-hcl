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

func TestImport_PreservesWholeJSONObjects(t *testing.T) {
	for _, object := range []string{`{"for":{"in":1,"true":false,"false":null,"null":"x","a-b":{"for":2}}}`, `{"e\u0301":{"a-b":1},"é":2}`} {
		input := sliceDocument("0", `{"id":"c","title":"C","type":"COMMAND","prototype":`+object+`,"fields":[{"name":"v","type":"Custom","example":`+object+`}],"dependencies":[]}`, "", "")
		imported := app.Import("model.json", []byte(input), "model.em.hcl")
		if imported.Diagnostics.HasErrors() {
			t.Fatalf("import diagnostics: %#v\n%s", imported.Diagnostics, imported.Source)
		}
		exported := app.Export("model.em.hcl", []byte(imported.Source))
		if exported.Diagnostics.HasErrors() {
			t.Fatalf("export diagnostics: %#v", exported.Diagnostics)
		}
		command := parsed(t, exported.JSON).Slices[0].Commands[0]
		var want any
		if err := json.Unmarshal([]byte(object), &want); err != nil {
			t.Fatal(err)
		}
		for name, raw := range map[string]json.RawMessage{"prototype": command.Prototype, "example": command.Fields[0].Example} {
			var got any
			if err := json.Unmarshal(raw, &got); err != nil {
				t.Fatalf("%s missing or invalid: %s", name, raw)
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("%s = %#v, want %#v", name, got, want)
			}
		}
	}
}

func TestImport_ExternalContextDoesNotCaptureFallbackEvents(t *testing.T) {
	input := sliceDocument("0", conformingCommand, "", "")
	input = strings.Replace(input, `"events":[]`, `"events":[{"id":"external","title":"External Changed","type":"EVENT","context":"EXTERNAL","modelContext":"Remote","fields":[],"dependencies":[]},{"id":"internal","title":"Internal Changed","type":"EVENT","fields":[],"dependencies":[]}]`, 1)
	imported := app.Import("model.json", []byte(input), "model.em.hcl")
	if imported.Diagnostics.HasErrors() {
		t.Fatalf("diagnostics: %#v\n%s", imported.Diagnostics, imported.Source)
	}
	built, diagnostics := app.ValidatedModel("model.em.hcl", []byte(imported.Source), app.Valid)
	if diagnostics.HasErrors() {
		t.Fatal(diagnostics)
	}
	for _, context := range built.Contexts {
		for _, event := range context.Events {
			if event.Title == "Internal Changed" && context.External {
				t.Fatal("internal fallback event absorbed by external context")
			}
		}
	}
}

func TestExport_UnproducedEventsAreOriginals(t *testing.T) {
	document, _, err := interchange.Export(&model.Model{Contexts: []model.Context{{ID: "remote", External: true, Events: []model.Event{{ID: "changed", Title: "Changed"}}}}, Workflows: []model.Workflow{{ID: "view", Kind: model.StateView, Elements: []model.Element{{ID: "view", Kind: model.ReadModel, From: []string{"event.remote.changed"}}}}}, Edges: []model.Edge{{WorkflowID: "view", From: "event.remote.changed", To: "readmodel.view"}}})
	if err != nil {
		t.Fatal(err)
	}
	if len(document.Slices) != 1 || len(document.Slices[0].Events) != 1 {
		t.Fatalf("exported event cardinality: %#v", document)
	}
	if event := document.Slices[0].Events[0]; event.ElementCopy || event.LinkedID != "" {
		t.Fatalf("unproduced event exported as copy: %#v", event)
	}
}

func TestExport_SpecificationLinksItsSlice(t *testing.T) {
	document, _, err := interchange.Export(&model.Model{Workflows: []model.Workflow{{ID: "save", Kind: model.StateChange, Scenarios: []model.Scenario{{ID: "success", Steps: []model.Step{{Kind: model.When, Target: "command", Ref: "command.save"}}}}}}})
	if err != nil {
		t.Fatal(err)
	}
	if specification := document.Slices[0].Specifications[0]; specification.LinkedID != document.Slices[0].ID {
		t.Fatalf("specification linkedId = %q, want slice id", specification.LinkedID)
	}
}

func TestImport_EmptyEventIDsRetainOccurrenceFlows(t *testing.T) {
	input := sliceDocument("0", conformingCommand, "", "")
	input = strings.Replace(input, `"events":[]`, `"events":[{"id":"","title":"First","type":"EVENT","fields":[],"dependencies":[{"id":"c","type":"INBOUND","title":"C","elementType":"COMMAND"}]},{"id":"","title":"Second","type":"EVENT","fields":[],"dependencies":[{"id":"c","type":"INBOUND","title":"C","elementType":"COMMAND"}]}]`, 1)
	imported := app.Import("model.json", []byte(input), "model.em.hcl")
	if imported.Diagnostics.HasErrors() {
		t.Fatalf("diagnostics: %#v\n%s", imported.Diagnostics, imported.Source)
	}
	built, diagnostics := app.ValidatedModel("model.em.hcl", []byte(imported.Source), app.Valid)
	if diagnostics.HasErrors() {
		t.Fatal(diagnostics)
	}
	if got := built.Workflows[0].Elements[0].To; len(got) != 2 {
		t.Fatalf("command outputs = %v, want both empty-id occurrences", got)
	}
}

func TestImport_CanonicalEventContextNamesShareIdentity(t *testing.T) {
	input := sliceDocument("0", conformingCommand, "", "")
	input = strings.Replace(input, `"events":[]`, `"events":[{"id":"event.orders.placed","title":"Placed","type":"EVENT","modelContext":"Orders","fields":[],"dependencies":[]},{"id":"foreign","title":"Accepted","type":"EVENT","modelContext":"Orders","fields":[],"dependencies":[]}]`, 1)
	imported := app.Import("model.json", []byte(input), "model.em.hcl")
	if imported.Diagnostics.HasErrors() {
		t.Fatalf("diagnostics: %#v\n%s", imported.Diagnostics, imported.Source)
	}
	built, diagnostics := app.ValidatedModel("model.em.hcl", []byte(imported.Source), app.Valid)
	if diagnostics.HasErrors() {
		t.Fatal(diagnostics)
	}
	if len(built.Contexts) != 1 || len(built.Contexts[0].Events) != 2 {
		t.Fatalf("contexts = %#v, want one shared context", built.Contexts)
	}
}

func TestImport_NativeNullAndTextNullRemainDistinct(t *testing.T) {
	input := sliceDocument("0", `{"id":"c","title":"C","type":"COMMAND","fields":[{"name":"number","type":"Int","example":"null"},{"name":"text","type":"String","example":"null"}],"dependencies":[]}`, "", "")
	imported := app.Import("model.json", []byte(input), "model.em.hcl")
	if imported.Diagnostics.HasErrors() {
		t.Fatalf("diagnostics: %#v\n%s", imported.Diagnostics, imported.Source)
	}
	built, diagnostics := app.ValidatedModel("model.em.hcl", []byte(imported.Source), app.Valid)
	if diagnostics.HasErrors() {
		t.Fatal(diagnostics)
	}
	fields := built.Workflows[0].Elements[0].Fields
	if string(fields[0].Example) != "null" || string(fields[1].Example) != `"null"` {
		t.Fatalf("null examples = %#v", fields)
	}
}

func TestImport_RejectsMultipleActorsEvenWithoutScreens(t *testing.T) {
	for _, screens := range []string{`[]`, `[{"id":"screen","title":"Screen","type":"SCREEN","fields":[],"dependencies":[]}]`} {
		input := sliceDocument("0", conformingCommand, "", `,"actors":[{"name":"Staff","authRequired":false},{"name":"Customer","authRequired":false}]`)
		input = strings.Replace(input, `"screens":[]`, `"screens":`+screens, 1)
		imported := app.Import("model.json", []byte(input), "model.em.hcl")
		if !imported.Diagnostics.HasErrors() || imported.Source != "" {
			t.Fatalf("multiple actors accepted: %#v", imported)
		}
	}
}

func TestImport_ScenarioObjectsKeepAllKeys(t *testing.T) {
	object := `{"for":{"in":1,"a-b":2},"e\u0301":3,"é":4}`
	specification := `{"id":"p","title":"P","linkedId":"s","given":[],"when":[{"id":"w","title":"W","type":"SPEC_COMMAND","linkedId":"c","examples":[` + object + `]}],"then":[{"id":"t","title":"Failure","type":"SPEC_ERROR"}]}`
	imported := app.Import("model.json", []byte(sliceDocument("0", conformingCommand, specification, "")), "model.em.hcl")
	if imported.Diagnostics.HasErrors() {
		t.Fatalf("diagnostics: %#v\n%s", imported.Diagnostics, imported.Source)
	}
	exported := app.Export("model.em.hcl", []byte(imported.Source))
	if exported.Diagnostics.HasErrors() {
		t.Fatal(exported.Diagnostics)
	}
	example := parsed(t, exported.JSON).Slices[0].Specifications[0].When[0].Examples[0]
	var got, want any
	if err := json.Unmarshal(example, &got); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(object), &want); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("scenario example = %#v, want %#v", got, want)
	}
}

func TestImport_InternalNameDoesNotAliasExternalCanonicalContext(t *testing.T) {
	input := sliceDocument("0", conformingCommand, "", "")
	input = strings.Replace(input, `"events":[]`, `"events":[{"id":"event.remote.changed","title":"Remote Changed","type":"EVENT","context":"EXTERNAL","modelContext":"Remote Service","fields":[],"dependencies":[]},{"id":"internal","title":"Internal Changed","type":"EVENT","modelContext":"remote","fields":[],"dependencies":[]}]`, 1)
	imported := app.Import("model.json", []byte(input), "model.em.hcl")
	if imported.Diagnostics.HasErrors() {
		t.Fatalf("diagnostics: %#v\n%s", imported.Diagnostics, imported.Source)
	}
	built, diagnostics := app.ValidatedModel("model.em.hcl", []byte(imported.Source), app.Valid)
	if diagnostics.HasErrors() {
		t.Fatal(diagnostics)
	}
	if len(built.Contexts) != 2 {
		t.Fatalf("contexts = %#v, want distinct internal and external contexts", built.Contexts)
	}
}
