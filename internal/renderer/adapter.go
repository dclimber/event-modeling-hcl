package renderer

import (
	"path/filepath"
	"sort"
	"strings"

	"github.com/event-modeling-hcl/eventmodeling-hcl/internal/model"
)

type fieldTypeEntry struct {
	typeName    string
	cardinality string
	id          bool
	pii         bool
}

type eventEntry struct {
	contextID string
	event     model.Event
	external  bool
}

type eventPlacement struct {
	sliceIndex int
	given      bool
}

type adapter struct {
	model      *model.Model
	fieldTypes map[string]fieldTypeEntry
	events     map[string]eventEntry
	titles     map[string]string
	owners     map[string]string
}

func BuildViewModel(filename string, source *model.Model) *ViewModel {
	a := newAdapter(source)
	view := &ViewModel{
		Title:    humanizeFilename(filename),
		Version:  specVersion,
		Actors:   map[string]Actor{},
		Contexts: map[string]Context{},
	}
	for _, actor := range source.Actors {
		view.Actors[actor.ID] = Actor{Title: actor.Title, AuthRequired: actor.AuthRequired}
	}
	for _, context := range source.Contexts {
		view.Contexts[context.ID] = Context{Title: context.Title, External: context.External}
	}
	for _, workflow := range source.Workflows {
		view.Slices = append(view.Slices, a.adaptWorkflow(workflow))
	}
	view.Edges = a.adaptEdges()
	a.placeEvents(view)
	a.assignLayout(view)
	view.Chapters = adaptChapters(source.Chapters)
	view.ContextMap = a.buildContextMap()
	view.Hotspots = a.adaptHotspots(source.Hotspots)
	return view
}

func newAdapter(source *model.Model) *adapter {
	a := &adapter{
		model:      source,
		fieldTypes: map[string]fieldTypeEntry{},
		events:     map[string]eventEntry{},
		titles:     map[string]string{},
		owners:     map[string]string{},
	}
	for _, actor := range source.Actors {
		a.owners["actor."+actor.ID] = actor.Title
	}
	for _, owner := range source.Owners {
		a.owners[owner.Kind+"."+owner.ID] = owner.Title
	}
	for _, context := range source.Contexts {
		a.owners["bounded_context."+context.ID] = context.Title
		for _, fieldType := range context.FieldTypes {
			a.fieldTypes[context.ID+"."+fieldType.ID] = fieldTypeEntry{
				typeName: fieldType.Type, cardinality: fieldType.Cardinality,
				id: fieldType.IDAttribute, pii: fieldType.PII,
			}
		}
		for _, event := range context.Events {
			address := context.ID + "." + event.ID
			a.events[address] = eventEntry{contextID: context.ID, event: event, external: context.External}
			a.titles["event."+address] = event.Title
		}
	}
	for _, workflow := range source.Workflows {
		a.titles["workflow."+workflow.ID] = workflow.Title
		for _, element := range workflow.Elements {
			a.titles[string(element.Kind)+"."+workflow.ID+"."+element.ID] = element.Title
			a.titles[workflow.ID+"::"+string(element.Kind)+"."+element.ID] = element.Title
		}
	}
	return a
}

func (a *adapter) adaptWorkflow(workflow model.Workflow) Slice {
	slice := Slice{
		ID: workflow.ID, Type: string(workflow.Kind), Title: workflow.Title,
		Status: workflow.Status, Owner: a.ownerTitle(workflow.Owner), Description: workflow.Description,
		Elements: []Element{}, Scenarios: []Scenario{},
	}
	for _, source := range workflow.Elements {
		slice.Elements = append(slice.Elements, a.adaptElement(workflow.ID, source))
	}
	for _, source := range workflow.Scenarios {
		slice.Scenarios = append(slice.Scenarios, a.adaptScenario(workflow.ID, source))
	}
	return slice
}

func (a *adapter) assignLayout(view *ViewModel) {
	for sliceIndex := range view.Slices {
		slice := &view.Slices[sliceIndex]
		stages := make(map[string]int, len(slice.Elements))
		local := make(map[string]bool, len(slice.Elements))
		for _, element := range slice.Elements {
			local[element.ID] = true
			stages[element.ID] = naturalStage(slice.Type, element)
		}
		for range slice.Elements {
			changed := false
			for _, edge := range view.Edges {
				if !local[edge.From] || !local[edge.To] || stages[edge.To] > stages[edge.From] {
					continue
				}
				stages[edge.To] = stages[edge.From] + 1
				changed = true
			}
			if !changed {
				break
			}
		}
		alignPresentationStages(slice, stages)
		stages = compactStages(stages)
		maxStage := 0
		for elementIndex := range slice.Elements {
			element := &slice.Elements[elementIndex]
			element.Stage = stages[element.ID]
			maxStage = max(maxStage, element.Stage)
		}
		slice.StageCount = maxStage + 1
	}
}

func naturalStage(workflowType string, element Element) int {
	switch workflowType {
	case string(model.StateChange):
		switch element.Kind {
		case "screen", "screen_image":
			return 0
		case "command", "table":
			return 1
		case "event":
			return 2
		}
	case string(model.StateView):
		switch element.Kind {
		case "event":
			return 0
		case "readmodel", "table":
			return 1
		case "screen", "screen_image":
			return 2
		}
	case string(model.Automation), string(model.Translation):
		switch element.Kind {
		case "event":
			if element.Given {
				return 0
			}
			return 4
		case "readmodel", "table":
			return 1
		case "processor":
			return 2
		case "command":
			return 3
		}
	}
	return 0
}

func alignPresentationStages(slice *Slice, stages map[string]int) {
	screenStage := -1
	semanticStage := -1
	for _, element := range slice.Elements {
		switch element.Kind {
		case "screen":
			if screenStage == -1 || stages[element.ID] < screenStage {
				screenStage = stages[element.ID]
			}
		case "command", "readmodel":
			if semanticStage == -1 || stages[element.ID] < semanticStage {
				semanticStage = stages[element.ID]
			}
		}
	}
	for _, element := range slice.Elements {
		switch {
		case element.Kind == "screen_image" && screenStage >= 0:
			stages[element.ID] = screenStage
		case element.Kind == "table" && semanticStage >= 0:
			stages[element.ID] = semanticStage
		}
	}
}

func compactStages(stages map[string]int) map[string]int {
	values := make([]int, 0, len(stages))
	seen := map[int]bool{}
	for _, stage := range stages {
		if !seen[stage] {
			seen[stage] = true
			values = append(values, stage)
		}
	}
	sort.Ints(values)
	compact := make(map[int]int, len(values))
	for index, stage := range values {
		compact[stage] = index
	}
	result := make(map[string]int, len(stages))
	for id, stage := range stages {
		result[id] = compact[stage]
	}
	return result
}

func (a *adapter) adaptElement(workflowID string, source model.Element) Element {
	contextID := contextFromAggregate(source.Semantic.Aggregate)
	return Element{
		ID:       elementNodeID(workflowID, string(source.Kind), source.ID),
		Kind:     string(source.Kind),
		Title:    source.Title,
		Actor:    lastPart(source.Semantic.Actor),
		ImageURL: source.Presentation.URL,
		Agg:      lastPart(source.Semantic.Aggregate),
		API:      source.Semantic.APIEndpoint,
		Question: source.Semantic.Question,
		Tags:     source.Presentation.Tags,
		Fields:   a.adaptFields(source.Fields, contextID),
	}
}

func (a *adapter) adaptFields(source []model.Field, contextID string) []Field {
	fields := make([]Field, 0, len(source))
	for _, sourceField := range source {
		field := Field{Name: sourceField.Name, Type: sourceField.Type, ID: sourceField.IDAttribute, PII: sourceField.PII}
		cardinality := sourceField.Cardinality
		if address, ok := fieldTypeAddress(sourceField.Type, contextID); ok {
			if fieldType, exists := a.fieldTypes[address]; exists {
				field.Type = fieldType.typeName
				field.ID = field.ID || fieldType.id
				field.PII = field.PII || fieldType.pii
				if cardinality == "" {
					cardinality = fieldType.cardinality
				}
			}
		}
		if cardinality == "List" {
			field.Type += "[]"
		}
		fields = append(fields, field)
	}
	return fields
}

func (a *adapter) adaptScenario(workflowID string, source model.Scenario) Scenario {
	scenario := Scenario{Title: source.Title, Comments: append([]string(nil), source.Comments...)}
	for _, sourceStep := range source.Steps {
		step := a.adaptStep(workflowID, sourceStep)
		switch sourceStep.Kind {
		case model.Given:
			scenario.Given = append(scenario.Given, step)
		case model.When:
			scenario.When = &step
		case model.Then:
			scenario.Then = append(scenario.Then, step)
		}
	}
	return scenario
}

func (a *adapter) adaptStep(workflowID string, source model.Step) Step {
	title := source.Title
	if title == "" {
		if source.Error != "" {
			title = source.Error
		} else {
			title = a.referenceTitle(workflowID, source.Ref)
		}
	}
	fieldNames := make([]string, 0, len(source.Fields))
	for _, field := range source.Fields {
		fieldNames = append(fieldNames, field.Name)
	}
	return Step{
		Title: title, RefKind: source.Target, Ref: a.referenceTitle(workflowID, source.Ref),
		Examples: source.Examples, Fields: fieldNames, Error: source.Error,
		EmptyList: source.ExpectEmptyList,
	}
}

func (a *adapter) referenceTitle(workflowID, reference string) string {
	if reference == "" {
		return ""
	}
	if title, ok := a.titles[reference]; ok {
		return title
	}
	return a.titles[workflowID+"::"+reference]
}

func (a *adapter) buildContextMap() ContextMap {
	contextMap := ContextMap{
		Nodes: make([]ContextNode, 0, len(a.model.Contexts)),
		Edges: make([]ContextEdge, 0),
	}
	for _, context := range a.model.Contexts {
		contextMap.Nodes = append(contextMap.Nodes, ContextNode{
			ID: context.ID, Title: context.Title, External: context.External,
			Team: a.ownerTitle(context.Owner), Events: len(context.Events), Aggregates: len(context.Aggregates),
		})
	}

	edges := map[string]*ContextEdge{}
	workflows := map[string]map[string]bool{}
	for _, workflow := range a.model.Workflows {
		produced := map[string][]int{}
		consumed := map[string][]int{}
		for _, edge := range a.model.Edges {
			if edge.WorkflowID != workflow.ID {
				continue
			}
			recordEventReference(edge.To, 0, produced)
			recordEventReference(edge.From, 0, consumed)
		}
		for _, scenario := range workflow.Scenarios {
			for _, step := range scenario.Steps {
				if step.Target != "event" {
					continue
				}
				switch step.Kind {
				case model.Given:
					recordEventReference(step.Ref, 0, consumed)
				case model.Then:
					recordEventReference(step.Ref, 0, produced)
				case model.When:
				}
			}
		}

		homeContexts := map[string]bool{}
		for address := range produced {
			if entry, ok := a.events[address]; ok && !entry.external {
				homeContexts[entry.contextID] = true
			}
		}
		home := ""
		for _, context := range a.model.Contexts {
			if homeContexts[context.ID] {
				home = context.ID
				break
			}
		}
		if home == "" {
			for _, element := range workflow.Elements {
				if ctx := contextFromAggregate(element.Semantic.Aggregate); ctx != "" {
					home = ctx
					break
				}
			}
		}
		if home == "" {
			continue
		}

		for address := range consumed {
			entry, ok := a.events[address]
			if !ok || entry.contextID == home {
				continue
			}
			key := entry.contextID + ">" + home
			edge := edges[key]
			if edge == nil {
				edge = &ContextEdge{
					Upstream: entry.contextID, Downstream: home, Pattern: "customer_supplier",
				}
				edges[key] = edge
				workflows[key] = map[string]bool{}
			}
			if workflow.Kind == model.Translation {
				edge.Pattern = "anticorruption"
			}
			workflows[key][workflow.ID] = true
		}
	}

	contextMap.Edges = make([]ContextEdge, 0, len(edges))
	for key, edge := range edges {
		edge.Via = make([]string, 0, len(workflows[key]))
		for workflowID := range workflows[key] {
			edge.Via = append(edge.Via, workflowID)
		}
		sort.Strings(edge.Via)
		contextMap.Edges = append(contextMap.Edges, *edge)
	}
	sort.Slice(contextMap.Edges, func(i, j int) bool {
		if contextMap.Edges[i].Upstream == contextMap.Edges[j].Upstream {
			return contextMap.Edges[i].Downstream < contextMap.Edges[j].Downstream
		}
		return contextMap.Edges[i].Upstream < contextMap.Edges[j].Upstream
	})
	return contextMap
}

func (a *adapter) adaptEdges() []Edge {
	edges := make([]Edge, 0, len(a.model.Edges))
	seen := map[string]bool{}
	for _, source := range a.model.Edges {
		from := nodeIDForReference(source.WorkflowID, source.From)
		to := nodeIDForReference(source.WorkflowID, source.To)
		key := from + ">" + to
		if from == "" || to == "" || seen[key] {
			continue
		}
		seen[key] = true
		edges = append(edges, Edge{From: from, To: to})
	}
	return edges
}

func (a *adapter) placeEvents(view *ViewModel) {
	producers := map[string][]int{}
	consumers := map[string][]int{}
	workflowIndex := map[string]int{}
	for index, workflow := range a.model.Workflows {
		workflowIndex[workflow.ID] = index
	}
	for _, edge := range a.model.Edges {
		index := workflowIndex[edge.WorkflowID]
		recordEventReference(edge.To, index, producers)
		recordEventReference(edge.From, index, consumers)
	}
	for index, workflow := range a.model.Workflows {
		for _, scenario := range workflow.Scenarios {
			for _, step := range scenario.Steps {
				if step.Target != "event" {
					continue
				}
				switch step.Kind {
				case model.Given:
					recordEventReference(step.Ref, index, consumers)
				case model.Then:
					recordEventReference(step.Ref, index, producers)
				case model.When:
				}
			}
		}
	}

	placements := map[string]eventPlacement{}
	for address := range a.events {
		if indices := producers[address]; len(indices) > 0 {
			placements[address] = eventPlacement{sliceIndex: maxIndex(indices)}
		} else if indices := consumers[address]; len(indices) > 0 {
			placements[address] = eventPlacement{sliceIndex: minIndex(indices), given: true}
		}
	}
	addresses := make([]string, 0, len(placements))
	for address := range placements {
		addresses = append(addresses, address)
	}
	sort.Strings(addresses)
	for _, address := range addresses {
		placement := placements[address]
		entry := a.events[address]
		view.Slices[placement.sliceIndex].Elements = append(view.Slices[placement.sliceIndex].Elements, Element{
			ID:       eventNodeID(address),
			Kind:     "event",
			Title:    entry.event.Title,
			Agg:      lastPart(entry.event.Semantic.Aggregate),
			Ctx:      entry.contextID,
			External: entry.external,
			Given:    placement.given,
			Tags:     entry.event.Presentation.Tags,
			Fields:   a.adaptFields(entry.event.Fields, entry.contextID),
		})
	}
}

func (a *adapter) adaptHotspots(source []model.Hotspot) []Hotspot {
	hotspots := make([]Hotspot, 0, len(source))
	for _, hotspot := range source {
		hotspots = append(hotspots, Hotspot{
			ID: hotspot.ID, OnID: nodeIDForGlobalReference(hotspot.On), Target: hotspot.On,
			Status: hotspot.Status, Question: hotspot.Question,
		})
	}
	return hotspots
}

func adaptChapters(source []model.Chapter) []Chapter {
	chapters := make([]Chapter, 0, len(source))
	for _, chapter := range source {
		workflows := make([]string, 0, len(chapter.Workflows))
		for _, workflow := range chapter.Workflows {
			workflows = append(workflows, lastPart(workflow))
		}
		chapters = append(chapters, Chapter{ID: chapter.ID, Title: chapter.Title, Slices: workflows})
	}
	return chapters
}

func (a *adapter) ownerTitle(reference string) string {
	if title, ok := a.owners[reference]; ok {
		return title
	}
	return lastPart(reference)
}

func nodeIDForReference(workflowID, reference string) string {
	parts := strings.Split(reference, ".")
	switch len(parts) {
	case 2:
		return elementNodeID(workflowID, parts[0], parts[1])
	case 3:
		if parts[0] == "event" {
			return eventNodeID(parts[1] + "." + parts[2])
		}
		return elementNodeID(parts[1], parts[0], parts[2])
	default:
		return ""
	}
}

func nodeIDForGlobalReference(reference string) string {
	parts := strings.Split(reference, ".")
	if len(parts) == 2 && parts[0] == "workflow" {
		return "slice__" + parts[1]
	}
	return nodeIDForReference("", reference)
}

func elementNodeID(workflowID, kind, id string) string {
	return workflowID + "__" + kind + "__" + id
}

func eventNodeID(address string) string {
	return "event__" + strings.ReplaceAll(address, ".", "__")
}

func recordEventReference(reference string, index int, target map[string][]int) {
	parts := strings.Split(reference, ".")
	if len(parts) == 3 && parts[0] == "event" {
		address := parts[1] + "." + parts[2]
		target[address] = append(target[address], index)
	}
}

func fieldTypeAddress(reference, contextID string) (string, bool) {
	parts := strings.Split(reference, ".")
	if len(parts) == 3 && parts[0] == "field_type" {
		return parts[1] + "." + parts[2], true
	}
	if len(parts) == 2 && parts[0] == "field_type" && contextID != "" {
		return contextID + "." + parts[1], true
	}
	return "", false
}

func contextFromAggregate(reference string) string {
	parts := strings.Split(reference, ".")
	if len(parts) == 3 && parts[0] == "aggregate" {
		return parts[1]
	}
	return ""
}

func lastPart(reference string) string {
	parts := strings.Split(reference, ".")
	return parts[len(parts)-1]
}

func minIndex(indices []int) int {
	minimum := indices[0]
	for _, index := range indices[1:] {
		minimum = min(minimum, index)
	}
	return minimum
}

func maxIndex(indices []int) int {
	maximum := indices[0]
	for _, index := range indices[1:] {
		maximum = max(maximum, index)
	}
	return maximum
}

func humanizeFilename(filename string) string {
	base := strings.TrimSuffix(filepath.Base(filename), ".em.hcl")
	words := strings.FieldsFunc(base, func(character rune) bool {
		return character == '_' || character == '-'
	})
	for index, word := range words {
		if word != "" {
			words[index] = strings.ToUpper(word[:1]) + word[1:]
		}
	}
	return strings.Join(words, " ")
}
