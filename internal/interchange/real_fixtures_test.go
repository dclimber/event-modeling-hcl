package interchange_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/event-modeling-hcl/eventmodeling-hcl/internal/app"
	"github.com/event-modeling-hcl/eventmodeling-hcl/internal/interchange"
	"github.com/event-modeling-hcl/eventmodeling-hcl/internal/model"
)

func TestImport_RealMartinDilgerDocuments(t *testing.T) {
	cases := []struct {
		name      string
		workflows int
		events    int
		scenarios int
		kinds     map[model.WorkflowKind]int
	}{
		{"config-pet-management-detailed.json", 7, 4, 11, map[model.WorkflowKind]int{model.StateChange: 4, model.StateView: 3}},
		{"axoniq-vsa-sample-news.json", 5, 6, 1, map[model.WorkflowKind]int{model.Automation: 3, model.Translation: 1, model.StateView: 1}},
		{"implmeneting-event-sourcing.json", 11, 11, 6, map[model.WorkflowKind]int{model.StateChange: 4, model.StateView: 2, model.Automation: 3, model.Translation: 2}},
		{"todolist.json", 6, 4, 0, map[model.WorkflowKind]int{model.StateChange: 4, model.StateView: 2}},
		{"understanding-event-sourcing.json", 11, 11, 7, map[model.WorkflowKind]int{model.StateChange: 4, model.StateView: 2, model.Automation: 3, model.Translation: 2}},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join("..", "..", "testdata", "valid", test.name)
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			imported := app.Import(path, data, "imported.em.hcl")
			if imported.Diagnostics.HasErrors() {
				t.Fatalf("import diagnostics: %+v\n%s", imported.Diagnostics, imported.Source)
			}
			built, diagnostics := app.ValidatedModel("imported.em.hcl", []byte(imported.Source), app.Valid)
			if diagnostics.HasErrors() {
				t.Fatalf("model diagnostics: %+v", diagnostics)
			}
			kinds := map[model.WorkflowKind]int{}
			scenarios, events := 0, 0
			for _, context := range built.Contexts {
				events += len(context.Events)
			}
			for _, workflow := range built.Workflows {
				kinds[workflow.Kind]++
				scenarios += len(workflow.Scenarios)
			}
			if len(built.Workflows) != test.workflows || events != test.events || scenarios != test.scenarios || !reflect.DeepEqual(kinds, test.kinds) {
				t.Fatalf("model has workflows=%d events=%d scenarios=%d kinds=%v; want %d %d %d %v", len(built.Workflows), events, scenarios, kinds, test.workflows, test.events, test.scenarios, test.kinds)
			}
			exported := app.Export("imported.em.hcl", []byte(imported.Source))
			if exported.Diagnostics.HasErrors() {
				t.Fatalf("export diagnostics: %+v", exported.Diagnostics)
			}
			reimported := app.Import("exported.json", []byte(exported.JSON), "again.em.hcl")
			if reimported.Diagnostics.HasErrors() {
				t.Fatalf("reimport diagnostics: %+v\n%s", reimported.Diagnostics, reimported.Source)
			}
			reexported := app.Export("again.em.hcl", []byte(reimported.Source))
			first, err := interchange.ParseDocument([]byte(exported.JSON))
			if err != nil {
				t.Fatal(err)
			}
			second, err := interchange.ParseDocument([]byte(reexported.JSON))
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(first, second) {
				t.Fatalf("canonical JSON changed on reimport\n%s\n%s", exported.JSON, reexported.JSON)
			}
		})
	}
}

func TestExport_RealNewsPrototypesAndSpecificationTargets(t *testing.T) {
	path := filepath.Join("..", "..", "testdata", "valid", "axoniq-vsa-sample-news.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var original interchange.Document
	if err := json.Unmarshal(data, &original); err != nil {
		t.Fatal(err)
	}
	imported := app.Import(path, data, "news.em.hcl")
	if imported.Diagnostics.HasErrors() {
		t.Fatalf("import diagnostics: %+v", imported.Diagnostics)
	}
	exported := app.Export("news.em.hcl", []byte(imported.Source))
	converted, err := interchange.ParseDocument([]byte(exported.JSON))
	if err != nil {
		t.Fatal(err)
	}
	byTitle := map[string]interchange.Slice{}
	for _, slice := range converted.Slices {
		byTitle[slice.Title] = slice
	}
	for _, slice := range original.Slices {
		for _, group := range []struct{ before, after []interchange.Element }{
			{slice.Commands, byTitle[slice.Title].Commands}, {slice.ReadModels, byTitle[slice.Title].ReadModels}, {slice.Screens, byTitle[slice.Title].Screens}, {slice.Processors, byTitle[slice.Title].Processors},
		} {
			if len(group.before) > 0 && group.before[0].Type == "AUTOMATION" && slice.SliceType == "STATE_CHANGE" {
				continue // declared STATE_CHANGE forbids processors; import omits them with a warning
			}
			if len(group.before) != len(group.after) {
				t.Fatalf("element cardinality changed in %s: %d to %d", slice.Title, len(group.before), len(group.after))
			}
			for index, element := range group.before {
				if index >= len(group.after) {
					t.Fatalf("missing %s in %s", element.Title, slice.Title)
				}
				assertJSONValue(t, group.after[index].Prototype, element.Prototype)
			}
		}
	}
	spec := byTitle["slice: Validate News"].Specifications[0]
	if got, want := spec.When[0].LinkedID, "command.slice_schedule_publication.validate_publication"; got != want {
		t.Fatalf("spec target=%q, want %q", got, want)
	}
	if got := spec.Given[0].Fields[4]; got.Name != "mapping_date" || string(got.Example) != `"2025-10-05 10:00:00"` {
		t.Fatalf("mappingDate example changed: %+v", got)
	}
	if spec.Given[0].LinkedID != "event.news.news_mapped" || spec.Given[1].LinkedID != "event.news.news_validated" {
		t.Fatalf("specification event order no longer follows supplied indices: %+v", spec.Given)
	}
}

func TestImport_DistinctSameTitleEventsAndLinkedCopies(t *testing.T) {
	input := []byte(`{"slices":[
		{"id":"workflow.create","title":"Create","sliceType":"STATE_CHANGE","commands":[{"id":"cmd","title":"Create","type":"COMMAND","fields":[],"dependencies":[{"id":"first","type":"OUTBOUND","title":"Changed","elementType":"EVENT"},{"id":"second","type":"OUTBOUND","title":"Changed","elementType":"EVENT"}]}],"events":[{"id":"first","title":"Changed","type":"EVENT","modelContext":"Orders","fields":[{"name":"firstValue","type":"String","example":"one"}],"dependencies":[]},{"id":"second","title":"Changed","type":"EVENT","modelContext":"Orders","fields":[{"name":"secondValue","type":"String","example":"two"}],"dependencies":[]}],"readmodels":[],"screens":[],"processors":[],"tables":[],"specifications":[]},
		{"id":"workflow.view","title":"View","sliceType":"STATE_VIEW","commands":[],"events":[{"id":"copy","title":"Changed","type":"EVENT","elementCopy":true,"linkedId":"second","fields":[],"dependencies":[]}],"readmodels":[{"id":"read","title":"Changes","type":"READMODEL","fields":[],"dependencies":[{"id":"copy","type":"INBOUND","title":"Changed","elementType":"EVENT"}]}],"screens":[],"processors":[],"tables":[],"specifications":[]}
	]}`)
	imported := app.Import("identity.json", input, "identity.em.hcl")
	if imported.Diagnostics.HasErrors() {
		t.Fatalf("diagnostics: %+v\n%s", imported.Diagnostics, imported.Source)
	}
	built, _ := app.ValidatedModel("identity.em.hcl", []byte(imported.Source), app.Valid)
	if len(built.Contexts[0].Events) != 2 {
		t.Fatalf("same-title events merged: %+v", built.Contexts[0].Events)
	}
	if got, want := built.Workflows[1].Elements[0].From, []string{"event.orders.changed_2"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("copy resolves to %v, want %v", got, want)
	}
	if got := built.Contexts[0].Events[1].Fields[0]; got.Name != "second_value" || string(got.Example) != `"two"` {
		t.Fatalf("event data changed: %+v", got)
	}
}

func TestImport_ExamplesPreserveTypedValuesAndDiscloseUnrepresentableSamples(t *testing.T) {
	input := []byte(`{"slices":[{"id":"workflow.values","title":"Values","sliceType":"STATE_CHANGE","commands":[{"id":"command.values.save","title":"Save","type":"COMMAND","fields":[
		{"name":"amount","type":"Double","example":"9.99"},
		{"name":"active","type":"Boolean","example":"true"},
		{"name":"absent","type":"Int","example":"null"},
		{"name":"blank","type":"Int","example":""},
		{"name":"items","type":"Custom","cardinality":"List","example":{"label":"original","nested":{"value":2}}}
	],"dependencies":[]}],"events":[],"readmodels":[],"screens":[],"processors":[],"tables":[],"specifications":[]}]}`)
	imported := app.Import("values.json", input, "values.em.hcl")
	if imported.Diagnostics.HasErrors() {
		t.Fatalf("diagnostics: %+v\n%s", imported.Diagnostics, imported.Source)
	}
	built, _ := app.ValidatedModel("values.em.hcl", []byte(imported.Source), app.Valid)
	fields := built.Workflows[0].Elements[0].Fields
	assertJSONValue(t, fields[0].Example, json.RawMessage(`9.99`))
	assertJSONValue(t, fields[1].Example, json.RawMessage(`true`))
	assertJSONValue(t, fields[2].Example, json.RawMessage(`null`))
	if fields[3].Example != nil {
		t.Fatalf("blank numeric example invented: %s", fields[3].Example)
	}
	assertJSONValue(t, fields[4].Example, json.RawMessage(`[{"label":"original","nested":{"value":2}}]`))
	warnings := strings.Join(imported.Warnings, "\n")
	if !strings.Contains(warnings, "blank") || !strings.Contains(warnings, "example") || !strings.Contains(warnings, "items") {
		t.Fatalf("loss/adaptation warnings missing: %v", imported.Warnings)
	}
	exported := app.Export("values.em.hcl", []byte(imported.Source))
	reimported := app.Import("exported.json", []byte(exported.JSON), "again.em.hcl")
	again, diagnostics := app.ValidatedModel("again.em.hcl", []byte(reimported.Source), app.Valid)
	if diagnostics.HasErrors() {
		t.Fatalf("reimport: %+v", diagnostics)
	}
	if !reflect.DeepEqual(fields, again.Workflows[0].Elements[0].Fields) {
		t.Fatalf("typed samples changed on roundtrip: %+v", again.Workflows[0].Elements[0].Fields)
	}
}

func TestExport_PrototypeObjectAndStepTagsSurviveNativeLowering(t *testing.T) {
	input := []byte(`bounded_context "orders" {
  event "saved" { prototype = { topic = "saved", nested = { enabled = false }, items = [1, null] } }
}
state_change "save" {
  command "save" {
    api_endpoint = "POST /orders"
    prototype = { route = "/orders", method = "POST", template = "${literal}" }
    to = [event.orders.saved]
  }
  scenario "success" {
    when {
      command = command.save
      tags = ["business-rule"]
      examples = [{ itemId = "a", nested = { ok = true } }]
    }
    then { event = event.orders.saved }
  }
}`)
	input = []byte(strings.ReplaceAll(string(input), "${literal}", "$${literal}"))
	exported := app.Export("native.em.hcl", input)
	if exported.Diagnostics.HasErrors() {
		t.Fatalf("diagnostics: %+v", exported.Diagnostics)
	}
	document, err := interchange.ParseDocument([]byte(exported.JSON))
	if err != nil {
		t.Fatal(err)
	}
	assertJSONValue(t, document.Slices[0].Commands[0].Prototype, json.RawMessage(`{"route":"/orders","method":"POST","template":"${literal}"}`))
	assertJSONValue(t, document.Slices[0].Events[0].Prototype, json.RawMessage(`{"topic":"saved","nested":{"enabled":false},"items":[1,null]}`))
	when := document.Slices[0].Specifications[0].When[0]
	if !reflect.DeepEqual(when.Tags, []string{"business-rule"}) {
		t.Fatalf("step tags lost: %v", when.Tags)
	}
	assertJSONValue(t, when.Examples[0], json.RawMessage(`{"itemId":"a","nested":{"ok":true}}`))
	imported := app.Import("native.json", []byte(exported.JSON), "again.em.hcl")
	reexported := app.Export("again.em.hcl", []byte(imported.Source))
	if reexported.Diagnostics.HasErrors() {
		t.Fatalf("roundtrip: %+v", reexported.Diagnostics)
	}
	again, err := interchange.ParseDocument([]byte(reexported.JSON))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(document, again) {
		t.Fatalf("native instance data changed\n%s\n%s", exported.JSON, reexported.JSON)
	}
}

func TestImport_WarnsAboutUnsupportedSchemaMetadataAndInvalidScenarioTargets(t *testing.T) {
	input := []byte(`{"slices":[{"id":"workflow.save","title":"Save","sliceType":"STATE_CHANGE","assignee":"alice","commands":[{"id":"command.save.save","title":"Save","type":"COMMAND","fields":[],"dependencies":[{"id":"missing","type":"OUTBOUND","title":"Missing","elementType":"EVENT"}]}],"events":[],"readmodels":[],"screens":[],"processors":[],"tables":[],"specifications":[{"id":"scenario.save.invalid","title":"Invalid target","linkedId":"workflow.save","given":[],"when":[{"id":"w","type":"SPEC_EVENT","linkedId":"command.save.save","title":"Wrong kind"}],"then":[{"id":"t","type":"SPEC_ERROR","title":"Rejected"}]}]}]}`)
	imported := app.Import("unsupported.json", input, "unsupported.em.hcl")
	if imported.Diagnostics.HasErrors() {
		t.Fatalf("diagnostics: %+v\n%s", imported.Diagnostics, imported.Source)
	}
	warnings := strings.Join(imported.Warnings, "\n")
	for _, want := range []string{"assignee", "unknown element", "Invalid target", "SPEC_EVENT"} {
		if !strings.Contains(warnings, want) {
			t.Errorf("warnings missing %q: %s", want, warnings)
		}
	}
	built, _ := app.ValidatedModel("unsupported.em.hcl", []byte(imported.Source), app.Valid)
	if len(built.Workflows[0].Scenarios) != 0 {
		t.Fatalf("invalid scenario target silently retargeted: %+v", built.Workflows[0].Scenarios)
	}
}

func TestExportImport_CourseActorsRemainUnambiguous(t *testing.T) {
	exported := app.ExportFile(filepath.Join("..", "..", "examples", "course_subscriptions.em.hcl"))
	if exported.Diagnostics.HasErrors() {
		t.Fatalf("export diagnostics: %+v", exported.Diagnostics)
	}
	document, err := interchange.ParseDocument([]byte(exported.JSON))
	if err != nil {
		t.Fatal(err)
	}
	for _, slice := range document.Slices {
		if len(slice.Actors) > 1 {
			t.Fatalf("multiple actors exported in %s: %+v", slice.Title, slice.Actors)
		}
	}
	imported := app.Import("courses.json", []byte(exported.JSON), "courses.em.hcl")
	if imported.Diagnostics.HasErrors() {
		t.Fatalf("import diagnostics: %+v", imported.Diagnostics)
	}
	_, diagnostics := app.ValidatedModel("courses.em.hcl", []byte(imported.Source), app.Valid)
	if diagnostics.HasErrors() {
		t.Fatalf("diagnostics: %+v", diagnostics)
	}
}

func assertJSONValue(t *testing.T, got, want json.RawMessage) {
	t.Helper()
	var actual, expected any
	if err := json.Unmarshal(got, &actual); err != nil {
		t.Fatalf("decode actual %s: %v", got, err)
	}
	if err := json.Unmarshal(want, &expected); err != nil {
		t.Fatalf("decode expected %s: %v", want, err)
	}
	if !reflect.DeepEqual(actual, expected) {
		t.Fatalf("JSON value=%s, want %s", got, want)
	}
}

func TestExport_FieldTypeFalseAndEmptyOverridesWin(t *testing.T) {
	source := []byte(`bounded_context "orders" {
  field_type "identifier" {
    type = "String"
    optional = true
    generated = true
    technical_attribute = true
    id_attribute = true
    pii = true
    mapping = "inherited"
    schema = "inherited"
    example = "inherited"
  }
}
state_change "save" {
  command "save" {
    api_endpoint = "POST /orders"
    field "identifier" {
      type = field_type.orders.identifier
      optional = false
      generated = false
      technical_attribute = false
      id_attribute = false
      pii = false
      mapping = ""
      schema = ""
      example = null
    }
  }
}`)
	exported := app.Export("overrides.em.hcl", source)
	if exported.Diagnostics.HasErrors() {
		t.Fatalf("diagnostics: %+v", exported.Diagnostics)
	}
	document, err := interchange.ParseDocument([]byte(exported.JSON))
	if err != nil {
		t.Fatal(err)
	}
	field := document.Slices[0].Commands[0].Fields[0]
	if field.Optional || field.Generated || field.TechnicalAttribute || field.IDAttribute || field.PII || field.Mapping != "" || field.Schema != "" {
		t.Fatalf("explicit overrides lost to inherited metadata: %+v", field)
	}
	assertJSONValue(t, field.Example, json.RawMessage(`"null"`))
}

func TestImport_RealSpecificationsPreserveRepeatedInstancesAndCausalLinks(t *testing.T) {
	path := filepath.Join("..", "..", "testdata", "valid", "understanding-event-sourcing.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	imported := app.Import(path, data, "cart.em.hcl")
	if imported.Diagnostics.HasErrors() {
		t.Fatalf("diagnostics: %+v", imported.Diagnostics)
	}
	built, diagnostics := app.ValidatedModel("cart.em.hcl", []byte(imported.Source), app.Valid)
	if diagnostics.HasErrors() {
		t.Fatalf("diagnostics: %+v", diagnostics)
	}
	var limit model.Scenario
	var cartItems model.Element
	for _, workflow := range built.Workflows {
		for _, scenario := range workflow.Scenarios {
			if scenario.Title == "spec: At most 3 Items" {
				limit = scenario
			}
		}
		for _, element := range workflow.Elements {
			if element.Kind == model.ReadModel && element.Title == "cart items" {
				cartItems = element
			}
		}
	}
	var given []model.Step
	var when, then model.Step
	for _, step := range limit.Steps {
		switch step.Kind {
		case model.Given:
			given = append(given, step)
		case model.When:
			when = step
		case model.Then:
			then = step
		}
	}
	if len(given) != 3 {
		t.Fatalf("three supplied event instances collapsed: %+v", given)
	}
	for _, step := range given {
		if step.Target != "event" || step.Ref != "event.cart.item_added" {
			t.Fatalf("given target changed: %+v", step)
		}
	}
	if when.Target != "command" || when.Ref != "command.add_item" || then.Target != "error" || then.Error != "Error-Case" {
		t.Fatalf("business-rule trigger/result changed: when=%+v then=%+v", when, then)
	}
	consumed := map[string]bool{}
	for _, reference := range cartItems.From {
		consumed[reference] = true
	}
	for _, reference := range []string{"event.cart.item_added", "event.cart.item_removed", "event.cart.cart_cleared", "event.cart.item_archived"} {
		if !consumed[reference] {
			t.Fatalf("cart projection lost causal event %s: %v", reference, cartItems.From)
		}
	}
}

func TestImport_RealPetContractPreservesNestedExamplesAndAuthentication(t *testing.T) {
	path := filepath.Join("..", "..", "testdata", "valid", "config-pet-management-detailed.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	imported := app.Import(path, data, "pets.em.hcl")
	if imported.Diagnostics.HasErrors() {
		t.Fatalf("diagnostics: %+v", imported.Diagnostics)
	}
	built, _ := app.ValidatedModel("pets.em.hcl", []byte(imported.Source), app.Valid)
	if len(built.Actors) != 1 || built.Actors[0].ID != "clinic_staff" || built.Actors[0].AuthRequired {
		t.Fatalf("authRequired=false changed: %+v", built.Actors)
	}
	readModel := built.Workflows[1].Elements[1]
	if readModel.Kind != model.ReadModel || readModel.Title != "Owner with Pets" {
		t.Fatalf("read model changed: %+v", readModel)
	}
	pets := readModel.Fields[6]
	if pets.Name != "pets" || pets.Type != "Custom" || pets.Cardinality != "List" {
		t.Fatalf("pet list contract changed: %+v", pets)
	}
	assertJSONValue(t, pets.Example, json.RawMessage(`[{"id":1,"name":"Leo","birthDate":"2010-09-07","type":"cat"}]`))
	if got := pets.Fields[2]; got.Name != "birth_date" || got.Type != "Date" || string(got.Example) != `"2010-09-07"` {
		t.Fatalf("nested pet date changed: %+v", got)
	}
	// Every specification now has a native shape: the Owner Registered given
	// resolves, and the pet-type listing has Pet Type Added givens.
	warnings := strings.Join(imported.Warnings, "\n")
	for _, omitted := range []string{"evt-001", "entire specification omitted"} {
		if strings.Contains(warnings, omitted) {
			t.Fatalf("specification content omitted (%q): %s", omitted, warnings)
		}
	}
	var listing *model.Scenario
	for index, workflow := range built.Workflows {
		for scenario := range workflow.Scenarios {
			if workflow.Scenarios[scenario].Title == "Display all pet types sorted by name" {
				listing = &built.Workflows[index].Scenarios[scenario]
			}
		}
	}
	if listing == nil {
		t.Fatal("pet-type listing scenario missing")
	}
	givens := 0
	for _, step := range listing.Steps {
		if step.Kind == model.Given && step.Ref == "event.maintain_the_list_of_pet_types_offered_in_forms.pet_type_added" {
			givens++
		}
	}
	if givens != 6 {
		t.Fatalf("pet-type listing has %d Pet Type Added givens, want 6: %+v", givens, listing.Steps)
	}
}

func TestImport_UnicodeTitlesAndCollidingFieldNamesRemainValidAndDisclosed(t *testing.T) {
	input := []byte(`{"slices":[{"id":"foreign","title":"Édit","context":"État","sliceType":"STATE_CHANGE","commands":[{"id":"command","title":"Édit","type":"COMMAND","fields":[{"name":"ÅValue","type":"String","example":"first"},{"name":"value","type":"String","example":"second"}],"dependencies":[]}],"events":[],"readmodels":[],"screens":[],"processors":[],"tables":[],"specifications":[]}]}`)
	imported := app.Import("unicode.json", input, "unicode.em.hcl")
	if imported.Diagnostics.HasErrors() {
		t.Fatalf("diagnostics: %+v\n%s", imported.Diagnostics, imported.Source)
	}
	built, _ := app.ValidatedModel("unicode.em.hcl", []byte(imported.Source), app.Valid)
	if built.Workflows[0].Title != "Édit" || built.Contexts[0].Title != "État" {
		t.Fatalf("display titles changed: %+v", built)
	}
	fields := built.Workflows[0].Elements[0].Fields
	if fields[0].Name != "value" || fields[1].Name != "value_2" || string(fields[0].Example) != `"first"` || string(fields[1].Example) != `"second"` {
		t.Fatalf("field collision merged or changed values: %+v", fields)
	}
	warnings := strings.Join(imported.Warnings, "\n")
	if !strings.Contains(warnings, "ÅValue") || !strings.Contains(warnings, "value_2") {
		t.Fatalf("renames not disclosed: %s", warnings)
	}
}

func TestImport_IDLessAndRepeatedPresentationIDsRetainEachInstance(t *testing.T) {
	input := []byte(`{"slices":[{"id":"workflow.view","title":"View","sliceType":"STATE_VIEW","commands":[],"events":[],"screens":[],"processors":[],"specifications":[],"readmodels":[{"id":"read","title":"View","type":"READMODEL","fields":[],"dependencies":[]}],"screenImages":[
		{"id":"","title":"Original image","url":"https://example.com/original.png"},
		{"id":"repeated","title":"Second image","url":"https://example.com/second.png"},
		{"id":"repeated","title":"Third image","url":"https://example.com/third.png"}
	],"tables":[
		{"id":"","title":"Original table","fields":[{"name":"value","type":"String","example":"original"}]},
		{"id":"repeated","title":"Second table","fields":[{"name":"value","type":"String","example":"second"}]},
		{"id":"repeated","title":"Third table","fields":[{"name":"value","type":"String","example":"third"}]}
	]}]}`)
	imported := app.Import("presentations.json", input, "presentations.em.hcl")
	if imported.Diagnostics.HasErrors() {
		t.Fatalf("diagnostics: %+v\n%s", imported.Diagnostics, imported.Source)
	}
	exported := app.Export("presentations.em.hcl", []byte(imported.Source))
	document, err := interchange.ParseDocument([]byte(exported.JSON))
	if err != nil {
		t.Fatal(err)
	}
	images := document.Slices[0].ScreenImages
	wantImages := []interchange.ScreenImage{
		{ID: "screen_image.view.original_image", Title: "Original image", URL: "https://example.com/original.png"},
		{ID: "screen_image.view.second_image", Title: "Second image", URL: "https://example.com/second.png"},
		{ID: "screen_image.view.third_image", Title: "Third image", URL: "https://example.com/third.png"},
	}
	if !reflect.DeepEqual(images, wantImages) {
		t.Fatalf("presentation occurrences changed: %+v", images)
	}
	wantTables := []interchange.Table{
		{ID: "table.view.original_table", Title: "Original table", Fields: []interchange.Field{{Name: "value", Type: "String", Example: json.RawMessage(`"original"`)}}},
		{ID: "table.view.second_table", Title: "Second table", Fields: []interchange.Field{{Name: "value", Type: "String", Example: json.RawMessage(`"second"`)}}},
		{ID: "table.view.third_table", Title: "Third table", Fields: []interchange.Field{{Name: "value", Type: "String", Example: json.RawMessage(`"third"`)}}},
	}
	tables := document.Slices[0].Tables
	if len(tables) != len(wantTables) {
		t.Fatalf("table occurrences changed: %+v", tables)
	}
	for index, want := range wantTables {
		got := tables[index]
		if got.ID != want.ID || got.Title != want.Title || len(got.Fields) != 1 || got.Fields[0].Name != "value" || got.Fields[0].Type != "String" {
			t.Fatalf("table occurrence %d changed: %+v", index, got)
		}
		assertJSONValue(t, got.Fields[0].Example, want.Fields[0].Example)
	}
}

func TestExportImport_ContextFreeWorkflowDoesNotInventAContext(t *testing.T) {
	source := []byte(`state_change "save" {
		command "save" { api_endpoint = "POST /save" }
	}`)
	first := app.Export("context_free.em.hcl", source)
	if first.Diagnostics.HasErrors() {
		t.Fatalf("diagnostics: %+v", first.Diagnostics)
	}
	imported := app.Import("context_free.json", []byte(first.JSON), "again.em.hcl")
	if imported.Diagnostics.HasErrors() {
		t.Fatalf("diagnostics: %+v\n%s", imported.Diagnostics, imported.Source)
	}
	built, _ := app.ValidatedModel("again.em.hcl", []byte(imported.Source), app.Valid)
	if len(built.Contexts) != 0 {
		t.Fatalf("context-free command acquired bounded contexts: %+v", built.Contexts)
	}
	second := app.Export("again.em.hcl", []byte(imported.Source))
	assertJSONValue(t, json.RawMessage(second.JSON), json.RawMessage(first.JSON))
}

func TestImport_SelfLinkedEventCopyCannotSupersedeItsOriginal(t *testing.T) {
	input := []byte(`{"slices":[
		{"id":"workflow.view","title":"View","index":0,"sliceType":"STATE_VIEW","commands":[],"events":[{"id":"changed","linkedId":"changed","elementCopy":true,"title":"Changed","type":"EVENT","modelContext":"Copies","prototype":{"origin":"copy"},"fields":[{"name":"value","type":"String","example":"copy"}],"dependencies":[]}],"readmodels":[{"id":"read","title":"Changes","type":"READMODEL","fields":[],"dependencies":[{"id":"changed","type":"INBOUND","title":"Changed","elementType":"EVENT"}]}],"screens":[],"processors":[],"tables":[],"specifications":[]},
		{"id":"workflow.create","title":"Create","index":1,"sliceType":"STATE_CHANGE","events":[{"id":"changed","title":"Changed","type":"EVENT","modelContext":"Originals","prototype":{"origin":"original"},"fields":[{"name":"value","type":"String","example":"original"}],"dependencies":[]}],"commands":[{"id":"create","title":"Create","type":"COMMAND","fields":[],"dependencies":[{"id":"changed","type":"OUTBOUND","title":"Changed","elementType":"EVENT"}]}],"readmodels":[],"screens":[],"processors":[],"tables":[],"specifications":[]}
	]}`)
	imported := app.Import("self_linked.json", input, "self_linked.em.hcl")
	if imported.Diagnostics.HasErrors() {
		t.Fatalf("diagnostics: %+v\n%s", imported.Diagnostics, imported.Source)
	}
	built, _ := app.ValidatedModel("self_linked.em.hcl", []byte(imported.Source), app.Valid)
	if len(built.Contexts) != 1 || built.Contexts[0].ID != "originals" {
		t.Fatalf("copy replaced the original context: %+v", built.Contexts)
	}
	event := built.Contexts[0].Events[0]
	if string(event.Fields[0].Example) != `"original"` {
		t.Fatalf("copy replaced original event data: %+v", event.Fields)
	}
	assertJSONValue(t, event.Presentation.PrototypeData, json.RawMessage(`{"origin":"original"}`))
	if got := built.Workflows[0].Elements[0].From; !reflect.DeepEqual(got, []string{"event.originals.changed"}) {
		t.Fatalf("copy identity disconnected: %v", got)
	}
	if !strings.Contains(strings.Join(imported.Warnings, "\n"), "different supplied data") {
		t.Fatalf("different copy snapshot not disclosed: %v", imported.Warnings)
	}
}

func TestImport_DeclaredSliceTypeIsAuthoritative(t *testing.T) {
	input := `{"slices":[{"id":"workflow.change","title":"Change","sliceType":"STATE_CHANGE",
		"commands":[{"id":"cmd","title":"Do it","type":"COMMAND","fields":[],"dependencies":[]}],
		"processors":[{"id":"proc","title":"Auto","type":"AUTOMATION","fields":[],"dependencies":[]}],
		"events":[],"readmodels":[],"screens":[],"tables":[],"specifications":[]}]}`
	imported := app.Import("model.json", []byte(input), "model.em.hcl")
	if imported.Diagnostics.HasErrors() {
		t.Fatalf("diagnostics: %+v\n%s", imported.Diagnostics, imported.Source)
	}
	built, diagnostics := app.ValidatedModel("model.em.hcl", []byte(imported.Source), app.Valid)
	if diagnostics.HasErrors() {
		t.Fatalf("diagnostics: %+v", diagnostics)
	}
	workflow := built.Workflows[0]
	if workflow.Kind != model.StateChange {
		t.Fatalf("kind = %s, want declared state_change", workflow.Kind)
	}
	for _, element := range workflow.Elements {
		if element.Kind == model.Processor {
			t.Fatalf("processor kept in state_change workflow")
		}
	}
	warned := false
	for _, warning := range imported.Warnings {
		warned = warned || strings.Contains(warning, "processor") && strings.Contains(warning, "omitted")
	}
	if !warned {
		t.Fatalf("warnings %v do not disclose the omitted processor", imported.Warnings)
	}
}

func TestRealFixturePendingWorkAndRegistrationContracts(t *testing.T) {
	for _, name := range []string{"implmeneting-event-sourcing.json", "understanding-event-sourcing.json", "config-pet-management-detailed.json"} {
		t.Run(name, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "valid", name))
			if err != nil {
				t.Fatal(err)
			}
			document, err := interchange.ParseDocument(data)
			if err != nil {
				t.Fatal(err)
			}
			hasDependency := func(element interchange.Element, id, direction string) bool {
				for _, dependency := range element.Dependencies {
					if dependency.ID == id && dependency.Type == direction {
						return true
					}
				}
				return false
			}
			eventsByID := map[string]interchange.Element{}
			for _, slice := range document.Slices {
				for _, event := range slice.Events {
					eventsByID[event.ID] = event
				}
			}
			translations := 0
			for _, slice := range document.Slices {
				if len(slice.Actors) > 1 {
					t.Fatalf("%s has multiple actors", slice.Title)
				}
				if slice.Title == "slice: Change Price" || slice.Title == "slice: Inventory changed" {
					translations++
					if slice.SliceType != "TRANSLATION" || len(slice.ReadModels) != 1 || len(slice.Processors) != 1 || len(slice.Commands) != 1 {
						t.Fatalf("translation shape changed: %+v", slice)
					}
					pending, processor, command := slice.ReadModels[0], slice.Processors[0], slice.Commands[0]
					if !pending.ListElement || len(pending.Fields) != 2 || !strings.Contains(pending.Description, "completion/removal semantics are not specified") {
						t.Fatalf("pending contract/disclosure missing: %+v", pending)
					}
					var external interchange.Element
					for _, event := range slice.Events {
						if event.Context == "EXTERNAL" {
							external = event
						}
					}
					if external.ID == "" || !hasDependency(external, pending.ID, "OUTBOUND") || !hasDependency(pending, external.ID, "INBOUND") || !hasDependency(pending, processor.ID, "OUTBOUND") || !hasDependency(processor, pending.ID, "INBOUND") || !hasDependency(processor, command.ID, "OUTBOUND") {
						t.Fatalf("translation pending-work chain missing: %+v", slice)
					}
					producesInternal := false
					for _, dependency := range command.Dependencies {
						if dependency.Type == "OUTBOUND" && dependency.ElementType == "EVENT" && eventsByID[dependency.ID].Context == "INTERNAL" {
							producesInternal = true
						}
					}
					if !producesInternal || len(pending.Dependencies) != 2 {
						t.Fatalf("translation result missing or speculative closing edge added: %+v", slice)
					}
					for index, field := range pending.Fields {
						source := external.Fields[index]
						if field.Name != source.Name || field.Type != source.Type || field.Cardinality != source.Cardinality || field.Optional != source.Optional || field.Generated != source.Generated || !reflect.DeepEqual(field.Example, source.Example) {
							t.Fatalf("translation pending field no longer follows external source: %+v versus %+v", field, source)
						}
					}
					if hasDependency(processor, external.ID, "INBOUND") {
						t.Fatalf("external event bypasses pending work: %+v", processor)
					}
				}
				for _, pending := range slice.ReadModels {
					if pending.Title == "Items to be archived" && (!pending.ListElement || !strings.Contains(pending.Description, "Pending-list question")) {
						t.Fatalf("archive pending-list uncertainty missing: %+v", pending)
					}
					if pending.Title == "Carts to be published" && !hasDependency(pending, "3458764620568353726", "INBOUND") {
						t.Fatalf("publication completion event missing: %+v", pending)
					}
				}
				if slice.Title == "Register Owner" {
					for _, element := range append(append([]interchange.Element{}, slice.Screens...), append(slice.Commands, slice.Events...)...) {
						for fieldName, example := range map[string]string{"address": `"110 W. Liberty St."`, "city": `"Madison"`, "telephone": `"6085551023"`} {
							found := false
							for _, field := range element.Fields {
								if field.Name == fieldName {
									found = field.Type == "String" && !field.Optional && string(field.Example) == example
								}
							}
							if !found || !strings.Contains(element.Description, "projection") {
								t.Fatalf("%s lacks documented registration field %s", element.Title, fieldName)
							}
						}
					}
				}
			}
			if name != "config-pet-management-detailed.json" && translations != 2 {
				t.Fatalf("got %d translation slices, want 2", translations)
			}
			imported := app.Import(name, data, "fixture.em.hcl")
			if imported.Diagnostics.HasErrors() {
				t.Fatalf("import diagnostics: %+v", imported.Diagnostics)
			}
			built, diagnostics := app.ValidatedModel("fixture.em.hcl", []byte(imported.Source), app.Valid)
			if diagnostics.HasErrors() {
				t.Fatalf("model diagnostics: %+v", diagnostics)
			}
			pendingCount := 0
			for _, workflow := range built.Workflows {
				if workflow.Kind != model.Translation {
					continue
				}
				for _, element := range workflow.Elements {
					if element.Kind == model.ReadModel && strings.HasPrefix(element.Title, "Pending ") {
						pendingCount++
						if len(element.From) != 1 || len(element.To) != 1 || len(element.Fields) != 2 || !strings.HasPrefix(element.To[0], "processor.") {
							t.Fatalf("native pending-work flow lost: %+v", element)
						}
					}
				}
			}
			if pendingCount != translations {
				t.Fatalf("native pending readmodels=%d, want %d", pendingCount, translations)
			}
		})
	}
}
