package interchange

import (
	"encoding/json"
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/event-modeling-hcl/eventmodeling-hcl/internal/model"
)

// Export converts a canonical model into the interchange JSON document.
// Identifiers are the model's fully qualified references (for example
// "command.register_pet.register_pet_command" or "event.clinic.pet_registered")
// so Import can restore the original block labels. Warnings name model
// content the interchange format cannot represent.
func Export(m *model.Model) (*Document, []string, error) {
	e := newExporter(m)
	document := &Document{Slices: make([]Slice, 0, len(m.Workflows))}
	for _, actor := range m.Actors {
		if actor.Description != "" {
			e.warn("actor.%s description has no interchange equivalent and was not exported", actor.ID)
		}
		if slug(actor.Title, "actor") != actor.ID {
			e.warn("actor.%s label cannot be carried by the JSON actor name; importing regenerates it from %q", actor.ID, actor.Title)
		}
	}
	for index, workflow := range m.Workflows {
		slice, err := e.slice(index, workflow)
		if err != nil {
			return nil, nil, err
		}
		document.Slices = append(document.Slices, slice)
	}
	exportedActors := map[string]bool{}
	for _, slice := range document.Slices {
		for _, actor := range slice.Actors {
			exportedActors[actor.Name] = true
		}
	}
	for _, actor := range m.Actors {
		if !exportedActors[actor.Title] {
			e.warn("actor.%s has no exported slice screen association; its catalog declaration was not exported", actor.ID)
		}
	}
	for _, context := range m.Contexts {
		for _, aggregate := range context.Aggregates {
			e.warn("aggregate.%s.%s catalog identity has no interchange equivalent; only referenced titles are exported in slices", context.ID, aggregate.ID)
			if aggregate.Description != "" {
				e.warn("aggregate.%s.%s description has no interchange equivalent and was not exported", context.ID, aggregate.ID)
			}
		}
		if context.Description != "" || context.Owner != "" {
			e.warn("bounded_context.%s description and owner metadata have no interchange catalog equivalent and were not exported", context.ID)
		}
		names := e.emittedContextNames[context.ID]
		if len(names) > 0 && context.Title != model.Humanize(context.ID) && !names[context.Title] {
			e.warn("bounded_context.%s title %q cannot be carried by a JSON context name that imports as the same label; the label was exported instead", context.ID, context.Title)
		}
		if context.TitleExplicit && context.Title == context.ID {
			e.warn("bounded_context.%s has an explicit title equal to its label; the JSON context name cannot record that the title was written, so importing leaves it implicit", context.ID)
		}
		if len(names) == 0 {
			e.warn("bounded_context.%s is not exposed by any slice; its catalog declaration was not exported", context.ID)
		}
		exportedEvent := false
		for _, event := range context.Events {
			exportedEvent = exportedEvent || e.exportedEvents["event."+context.ID+"."+event.ID]
		}
		if context.External && len(names) > 0 && !exportedEvent {
			e.warn("bounded_context.%s is external, but no exported event carries that; importing restores it as an internal context", context.ID)
		}
		for _, event := range context.Events {
			if !e.exportedEvents["event."+context.ID+"."+event.ID] {
				e.warn("event.%s.%s is not referenced by any workflow and was not exported", context.ID, event.ID)
			}
		}
	}
	if len(m.Owners) > 0 {
		e.warn("teams and systems have no interchange equivalent and were not exported")
	}
	if len(m.Chapters) > 0 {
		e.warn("chapters have no interchange equivalent and were not exported")
	}
	if len(m.Hotspots) > 0 {
		e.warn("hotspots have no interchange equivalent and were not exported")
	}
	return document, e.warnings, nil
}

// contextName is the JSON context string for a bounded context: its title when
// importing that title restores the same label, otherwise the label itself.
func (e *exporter) contextName(context model.Context) string {
	if contextLabel(context.Title, context.External, e.internalContextNames) == context.ID {
		return context.Title
	}
	return context.ID
}

// emitContext returns the JSON name written for context and records it. A
// slice context is always imported as internal, so an external owner is
// written by its label, which the importer resolves to the same context that
// its EXTERNAL events mark external.
func (e *exporter) emitContext(context model.Context, owner bool) string {
	name := e.contextName(context)
	if owner && context.External {
		name = context.ID
	}
	if e.emittedContextNames[context.ID] == nil {
		e.emittedContextNames[context.ID] = map[string]bool{}
	}
	e.emittedContextNames[context.ID][name] = true
	return name
}

// MarshalDocument encodes a document as indented JSON with a trailing newline.
// It fails rather than emit JSON that violates the interchange schema.
func MarshalDocument(document *Document) ([]byte, error) {
	encoded, err := json.MarshalIndent(document, "", "  ")
	if err != nil {
		return nil, err
	}
	if violations := CheckConformance(encoded); len(violations) > 0 {
		return nil, fmt.Errorf("exported JSON does not conform to the interchange schema:\n  %s", strings.Join(violations, "\n  "))
	}
	return append(encoded, '\n'), nil
}

type node struct {
	title       string
	elementType string
}

type exporter struct {
	nodes          map[string]node
	fieldTypes     map[string]model.FieldType
	aggregates     map[string]string
	contexts       map[string]model.Context
	actors         map[string]model.Actor
	edges          []model.Edge
	exportedEvents map[string]bool
	// internalContextNames are the JSON names written for internal contexts;
	// an external context sharing one imports with an "_external" suffix.
	internalContextNames map[string]bool
	// emittedContextNames records, per context label, the names written.
	emittedContextNames map[string]map[string]bool
	warnings            []string
	// expanding holds the field_type addresses whose subfields are being
	// exported, so a field_type reaching itself is detected.
	expanding map[string]bool
}

func newExporter(m *model.Model) *exporter {
	e := &exporter{nodes: map[string]node{}, fieldTypes: map[string]model.FieldType{}, aggregates: map[string]string{}, contexts: map[string]model.Context{}, actors: map[string]model.Actor{}, exportedEvents: map[string]bool{}, internalContextNames: map[string]bool{}, emittedContextNames: map[string]map[string]bool{}, expanding: map[string]bool{}}
	for _, actor := range m.Actors {
		e.actors["actor."+actor.ID] = actor
	}
	for _, context := range m.Contexts {
		e.contexts[context.ID] = context
		for _, aggregate := range context.Aggregates {
			e.aggregates[context.ID+"."+aggregate.ID] = aggregate.Title
		}
		for _, fieldType := range context.FieldTypes {
			e.fieldTypes[context.ID+"."+fieldType.ID] = fieldType
		}
		for _, event := range context.Events {
			e.nodes["event."+context.ID+"."+event.ID] = node{title: event.Title, elementType: "EVENT"}
		}
	}
	for _, workflow := range m.Workflows {
		for _, element := range workflow.Elements {
			if elementType := elementTypeOf(element.Kind); elementType != "" {
				e.nodes[qualify(string(element.Kind)+"."+element.ID, workflow.ID)] = node{title: element.Title, elementType: elementType}
			}
		}
	}
	for _, edge := range m.Edges {
		e.edges = append(e.edges, model.Edge{WorkflowID: edge.WorkflowID, From: qualify(edge.From, edge.WorkflowID), To: qualify(edge.To, edge.WorkflowID)})
	}
	// Only names the document will expose count as internal: those of
	// internal workflow owners and of internal events a slice carries.
	exposeInternal := func(contextID string) {
		if context, ok := e.contexts[contextID]; ok && !context.External {
			e.internalContextNames[e.contextName(context)] = true
		}
	}
	for _, workflow := range m.Workflows {
		if owner, ok := strings.CutPrefix(workflow.Owner, "bounded_context."); ok {
			exposeInternal(owner)
		}
		for _, reference := range e.sliceEvents(workflow) {
			if parts := strings.Split(reference, "."); len(parts) == 3 {
				exposeInternal(parts[1])
			}
		}
	}
	return e
}

func (e *exporter) warn(format string, args ...any) {
	e.warnings = append(e.warnings, fmt.Sprintf(format, args...))
}

// qualify expands a workflow-local element reference (command.<id>) into its
// fully qualified form (command.<workflow>.<id>). Other references pass through.
func qualify(reference, workflowID string) string {
	parts := strings.Split(reference, ".")
	if len(parts) == 2 && elementTypeOf(model.ElementKind(parts[0])) != "" {
		return parts[0] + "." + workflowID + "." + parts[1]
	}
	return reference
}

func elementTypeOf(kind model.ElementKind) string {
	switch kind {
	case model.Command:
		return "COMMAND"
	case model.ReadModel:
		return "READMODEL"
	case model.Screen:
		return "SCREEN"
	case model.Processor:
		return "AUTOMATION"
	}
	return ""
}

func (e *exporter) slice(index int, workflow model.Workflow) (Slice, error) {
	position := float64(index)
	slice := Slice{
		ID:             "workflow." + workflow.ID,
		Status:         exportStatus(workflow.Status),
		Index:          &position,
		Title:          workflow.Title,
		SliceType:      strings.ToUpper(string(workflow.Kind)),
		Commands:       []Element{},
		Events:         []Element{},
		ReadModels:     []Element{},
		Screens:        []Element{},
		Processors:     []Element{},
		Tables:         []Table{},
		Specifications: []Specification{},
	}
	if workflow.Description != "" {
		e.warn("workflow.%s description has no interchange equivalent and was not exported", workflow.ID)
	}
	if contextID, ok := strings.CutPrefix(workflow.Owner, "bounded_context."); ok {
		slice.Context = e.emitContext(e.contexts[contextID], true)
	} else if workflow.Owner != "" {
		e.warn("workflow.%s owner %s is not a bounded context and has no interchange equivalent; omitted", workflow.ID, workflow.Owner)
	}
	actors := map[string]bool{}
	screenActors := map[string]bool{}
	assignedScreens := 0
	aggregates := map[string]bool{}
	addAggregate := func(title string) {
		if title != "" && !aggregates[title] {
			aggregates[title] = true
			slice.Aggregates = append(slice.Aggregates, title)
		}
	}
	for _, element := range workflow.Elements {
		switch element.Kind {
		case model.ScreenImage:
			slice.ScreenImages = append(slice.ScreenImages, ScreenImage{ID: qualifyAny(element, workflow.ID), Title: element.Title, URL: element.Presentation.URL})
			continue
		case model.Table:
			slice.Tables = append(slice.Tables, Table{ID: qualifyAny(element, workflow.ID), Title: element.Title, Fields: e.fields(element.Fields, "")})
			continue
		}
		id := qualify(string(element.Kind)+"."+element.ID, workflow.ID)
		exported := e.element(id, elementTypeOf(element.Kind), element.Title, element.Semantic, element.Presentation, element.Fields, "")
		addAggregate(exported.Aggregate)
		switch element.Kind {
		case model.Command:
			slice.Commands = append(slice.Commands, exported)
		case model.ReadModel:
			slice.ReadModels = append(slice.ReadModels, exported)
		case model.Screen:
			slice.Screens = append(slice.Screens, exported)
			if element.Semantic.Actor != "" {
				assignedScreens++
				screenActors[element.Semantic.Actor] = true
			}
			if actor, ok := e.actors[element.Semantic.Actor]; ok && !actors[actor.ID] {
				actors[actor.ID] = true
				slice.Actors = append(slice.Actors, Actor{Name: actor.Title, AuthRequired: actor.AuthRequired})
			}
		case model.Processor:
			slice.Processors = append(slice.Processors, exported)
		}
	}
	if len(slice.Actors) == 1 && assignedScreens != len(slice.Screens) {
		e.warn("slice %q actors cannot distinguish assigned from unassigned screens; slice actor associations omitted", workflow.Title)
		slice.Actors = nil
	}
	for _, reference := range e.sliceEvents(workflow) {
		event, context, ok := e.event(reference)
		if !ok {
			continue
		}
		exported := e.element(reference, "EVENT", event.Title, event.Semantic, event.Presentation, event.Fields, context.ID)
		exported.ModelContext = e.emitContext(context, false)
		exported.Context = "INTERNAL"
		if context.External {
			exported.Context = "EXTERNAL"
		}
		exported.ElementCopy = e.isEventCopy(reference, workflow.ID)
		if exported.ElementCopy {
			exported.LinkedID = reference
		}
		addAggregate(exported.Aggregate)
		slice.Events = append(slice.Events, exported)
		e.exportedEvents[reference] = true
	}
	// The format has no event catalog, so the order of a slice's events must
	// not depend on catalog declaration order: re-importing derives a catalog
	// order of its own, and sorting by id keeps the export a fixpoint.
	sort.SliceStable(slice.Events, func(left, right int) bool { return slice.Events[left].ID < slice.Events[right].ID })
	for _, scenario := range workflow.Scenarios {
		slice.Specifications = append(slice.Specifications, e.specification(slice, workflow, scenario))
	}
	if len(screenActors) > 1 {
		names := make([]string, 0, len(screenActors))
		for name := range screenActors {
			names = append(names, name)
		}
		sort.Strings(names)
		return Slice{}, fmt.Errorf("workflow.%s screens name %d distinct actors (%s); the interchange format carries at most one actor per slice and cannot keep their screen associations", workflow.ID, len(names), strings.Join(names, ", "))
	}
	return slice, nil
}

func qualifyAny(element model.Element, workflowID string) string {
	return string(element.Kind) + "." + workflowID + "." + element.ID
}

// sliceEvents lists, in first-seen order, every event that a flow edge
// declared in the workflow or one of its scenario steps references.
func (e *exporter) sliceEvents(workflow model.Workflow) []string {
	seen := map[string]bool{}
	var events []string
	add := func(reference string) {
		if strings.HasPrefix(reference, "event.") && !seen[reference] {
			seen[reference] = true
			events = append(events, reference)
		}
	}
	for _, edge := range e.edges {
		if edge.WorkflowID == workflow.ID {
			add(edge.From)
			add(edge.To)
		}
	}
	for _, scenario := range workflow.Scenarios {
		for _, step := range scenario.Steps {
			add(step.Ref)
		}
	}
	return events
}

func (e *exporter) producedIn(event, workflowID string) bool {
	for _, edge := range e.edges {
		if edge.WorkflowID == workflowID && edge.To == event && strings.HasPrefix(edge.From, "command.") {
			return true
		}
	}
	return false
}

// isEventCopy reports whether event is an occurrence of an event that a
// different workflow produces. Events no workflow produces (external inputs,
// scenario-only givens) are originals, not copies.
func (e *exporter) isEventCopy(event, workflowID string) bool {
	if e.producedIn(event, workflowID) {
		return false
	}
	for _, edge := range e.edges {
		if edge.WorkflowID != workflowID && edge.To == event && strings.HasPrefix(edge.From, "command.") {
			return true
		}
	}
	return false
}

func (e *exporter) event(reference string) (model.Event, model.Context, bool) {
	parts := strings.Split(reference, ".")
	if len(parts) != 3 {
		return model.Event{}, model.Context{}, false
	}
	context, ok := e.contexts[parts[1]]
	if !ok {
		return model.Event{}, model.Context{}, false
	}
	for _, event := range context.Events {
		if event.ID == parts[2] {
			return event, context, true
		}
	}
	return model.Event{}, model.Context{}, false
}

func (e *exporter) element(id, elementType, title string, semantic model.Semantic, presentation model.Presentation, fields []model.Field, contextID string) Element {
	exported := Element{
		GroupID:          presentation.GroupID,
		ID:               id,
		Tags:             presentation.Tags,
		Title:            title,
		Fields:           e.fields(fields, contextID),
		Type:             elementType,
		Description:      semantic.Description,
		Aggregate:        e.aggregate(semantic.Aggregate, contextID),
		Dependencies:     e.dependencies(id),
		APIEndpoint:      semantic.APIEndpoint,
		CreatesAggregate: semantic.CreatesAggregate,
		Triggers:         semantic.Triggers,
		Sketched:         presentation.Sketched,
		ListElement:      presentation.ListElement,
	}
	if semantic.ExternalTrigger {
		e.warn("%s external_trigger has no interchange equivalent and was not exported", id)
	}
	if semantic.Question != "" {
		e.warn("%s question has no interchange equivalent; importing derives a question from description or title", id)
	}
	for _, dependency := range semantic.AggregateDependencies {
		exported.AggregateDependencies = append(exported.AggregateDependencies, e.aggregate(dependency, contextID))
	}
	if semantic.Service != "" {
		service := semantic.Service
		exported.Service = &service
	}
	exported.Prototype = presentation.PrototypeData
	if presentation.Prototype && len(presentation.PrototypeData) == 0 {
		e.warn("%s has a prototype presence flag without object data; no empty prototype was fabricated", id)
	}
	return exported
}

// aggregate resolves an aggregate reference (aggregate.<context>.<id>, or
// aggregate.<id> inside contextID) to its title.
func (e *exporter) aggregate(reference, contextID string) string {
	if reference == "" {
		return ""
	}
	parts := strings.Split(reference, ".")
	address := strings.Join(parts[1:], ".")
	if len(parts) == 2 {
		address = contextID + "." + parts[1]
	}
	if title, ok := e.aggregates[address]; ok {
		return title
	}
	return parts[len(parts)-1]
}

func (e *exporter) dependencies(id string) []Dependency {
	dependencies := []Dependency{}
	seen := map[string]bool{}
	add := func(direction, other string) {
		key := direction + other
		target, ok := e.nodes[other]
		if !ok || seen[key] {
			return
		}
		seen[key] = true
		dependencies = append(dependencies, Dependency{ID: other, Type: direction, Title: target.title, ElementType: target.elementType})
	}
	for _, edge := range e.edges {
		if edge.To == id {
			add("INBOUND", edge.From)
		}
		if edge.From == id {
			add("OUTBOUND", edge.To)
		}
	}
	// Group by direction, then by element kind in the order Import writes
	// workflow children. Each HCL to/from list holds one kind, so its author
	// order survives, while the export no longer depends on how the source
	// interleaves blocks of different kinds.
	sort.SliceStable(dependencies, func(left, right int) bool {
		a, b := dependencies[left], dependencies[right]
		if a.Type != b.Type {
			return a.Type == "INBOUND"
		}
		return dependencyKindRank[a.ElementType] < dependencyKindRank[b.ElementType]
	})
	return dependencies
}

var dependencyKindRank = map[string]int{"SCREEN": 0, "COMMAND": 1, "EVENT": 2, "READMODEL": 3, "AUTOMATION": 4}

func (e *exporter) fields(fields []model.Field, contextID string) []Field {
	exported := make([]Field, 0, len(fields))
	for _, field := range fields {
		exported = append(exported, e.field(field, contextID))
	}
	return exported
}

// field flattens a field and the field_type it references into one
// self-describing interchange field; attributes set on the field win.
func (e *exporter) field(field model.Field, contextID string) Field {
	exported := Field{Name: field.Name, Type: field.Type, Cardinality: field.Cardinality, Mapping: field.Mapping, Optional: field.Optional, TechnicalAttribute: field.TechnicalAttribute, Generated: field.Generated, IDAttribute: field.IDAttribute, PII: field.PII, Schema: field.Schema, Example: exportExample(field.Example)}
	subfields, subfieldContext := field.Fields, contextID
	if parts := strings.Split(field.Type, "."); parts[0] == "field_type" {
		address := strings.Join(parts[1:], ".")
		if len(parts) == 2 {
			address = contextID + "." + parts[1]
		}
		fieldType := e.fieldTypes[address]
		// A field_type that reaches itself through its own subfields would expand
		// forever; the re-entered reference is exported without its subfields.
		cyclic := e.expanding[address]
		if cyclic {
			e.warn("field_type.%s expands into itself through field %q; the cyclic expansion was stopped and its subfields were not exported", address, field.Name)
		} else {
			e.expanding[address] = true
			defer delete(e.expanding, address)
		}
		exported.Type = fieldType.Type
		exported.Cardinality = firstNonEmpty(exported.Cardinality, fieldType.Cardinality)
		if !slices.Contains(field.Overrides, "mapping") {
			exported.Mapping = firstNonEmpty(exported.Mapping, fieldType.Mapping)
		}
		if !slices.Contains(field.Overrides, "schema") {
			exported.Schema = firstNonEmpty(exported.Schema, fieldType.Schema)
		}
		if !slices.Contains(field.Overrides, "optional") {
			exported.Optional = exported.Optional || fieldType.Optional
		}
		if !slices.Contains(field.Overrides, "technical_attribute") {
			exported.TechnicalAttribute = exported.TechnicalAttribute || fieldType.TechnicalAttribute
		}
		if !slices.Contains(field.Overrides, "generated") {
			exported.Generated = exported.Generated || fieldType.Generated
		}
		if !slices.Contains(field.Overrides, "id_attribute") {
			exported.IDAttribute = exported.IDAttribute || fieldType.IDAttribute
		}
		if !slices.Contains(field.Overrides, "pii") {
			exported.PII = exported.PII || fieldType.PII
		}
		if exported.Example == nil {
			exported.Example = exportExample(fieldType.Example)
		}
		if len(subfields) == 0 && !cyclic {
			subfields, subfieldContext = fieldType.Fields, parts[len(parts)-2]
			if len(parts) == 2 {
				subfieldContext = contextID
			}
		}
	}
	if exported.Type == "" {
		exported.Type = "String"
	}
	if len(subfields) > 0 {
		exported.Subfields = e.fields(subfields, subfieldContext)
	}
	return exported
}

// exportExample keeps string and object examples, which the schema allows,
// and carries any other JSON value (number, bool, list, null) as its JSON text.
func exportExample(example json.RawMessage) json.RawMessage {
	trimmed := strings.TrimSpace(string(example))
	if trimmed == "" {
		return nil
	}
	if trimmed[0] == '"' || trimmed[0] == '{' {
		return example
	}
	encoded, _ := json.Marshal(trimmed)
	return encoded
}

func (e *exporter) specification(slice Slice, workflow model.Workflow, scenario model.Scenario) Specification {
	id := "scenario." + workflow.ID + "." + scenario.ID
	specification := Specification{ID: id, SliceName: slice.Title, Title: scenario.Title, Given: []SpecificationStep{}, When: []SpecificationStep{}, Then: []SpecificationStep{}, LinkedID: slice.ID}
	counts := map[model.StepKind]int{}
	for _, step := range scenario.Steps {
		exported := e.step(id, workflow.ID, counts[step.Kind], step)
		counts[step.Kind]++
		switch step.Kind {
		case model.Given:
			specification.Given = append(specification.Given, exported)
		case model.When:
			specification.When = append(specification.When, exported)
		case model.Then:
			specification.Then = append(specification.Then, exported)
		}
	}
	for _, comment := range scenario.Comments {
		specification.Comments = append(specification.Comments, Comment{Description: comment})
	}
	if scenario.Description != "" {
		specification.Comments = append(specification.Comments, Comment{Description: scenario.Description})
	}
	return specification
}

// step converts one given/when/then step. The schema has no processor step
// type, so a processor step is carried as SPEC_COMMAND whose linkedId names
// the processor; Import restores the processor target from that id.
func (e *exporter) step(specificationID, workflowID string, position int, step model.Step) SpecificationStep {
	stepTypes := map[string]string{"event": "SPEC_EVENT", "command": "SPEC_COMMAND", "processor": "SPEC_COMMAND", "readmodel": "SPEC_READMODEL", "error": "SPEC_ERROR"}
	stepType := stepTypes[step.Target]
	index := float64(position)
	exported := SpecificationStep{Title: step.Title, Tags: step.Tags, ID: fmt.Sprintf("%s.%s.%d", specificationID, step.Kind, position), Index: &index, Type: stepType, Fields: e.fields(step.Fields, ""), ExpectEmptyList: step.ExpectEmptyList}
	if step.Target == "error" {
		exported.Title = firstNonEmpty(step.Error, step.Title)
		if step.Error != "" && step.Title != "" && step.Title != step.Error {
			e.warn("%s title %q has no interchange equivalent on an error step; the error text %q was exported as its title", exported.ID, step.Title, step.Error)
		}
	} else {
		exported.LinkedID = qualify(step.Ref, workflowID)
		if exported.Title == "" {
			exported.Title = e.nodes[exported.LinkedID].title
		}
	}
	if len(exported.Fields) == 0 {
		exported.Fields = nil
	}
	if len(step.Examples) > 0 {
		_ = json.Unmarshal(step.Examples, &exported.Examples)
	}
	return exported
}

func exportStatus(status string) string {
	if status == "" {
		return ""
	}
	var builder strings.Builder
	for _, word := range strings.Split(status, "_") {
		if word != "" {
			builder.WriteString(strings.ToUpper(word[:1]) + word[1:])
		}
	}
	return builder.String()
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
