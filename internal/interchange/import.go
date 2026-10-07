package interchange

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"math/big"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/zclconf/go-cty/cty"
	ctyjson "github.com/zclconf/go-cty/cty/json"

	"github.com/event-modeling-hcl/eventmodeling-hcl/internal/model"
)

// Import converts an interchange document into unformatted Event Modeling
// HCL source. Warnings name interchange content that could not be expressed
// (for example a dependency the target workflow kind does not allow).
func Import(document *Document) ([]byte, []string, error) {
	if err := rejectMultipleActors(document); err != nil {
		return nil, nil, err
	}
	im := &importer{
		contexts:      map[string]*importedContext{},
		contextNames:  map[string]string{},
		ids:           map[string]ref{},
		eventRefs:     map[*Element]ref{},
		eventSources:  map[ref]*Element{},
		elementRefs:   map[*Element]ref{},
		elementsByRef: map[ref]*Element{},
		localIDs:      map[string]map[string]ref{},
		workflowOf:    map[*Slice]string{},
		labels:        map[string]map[string]bool{},
		actors:        map[string]string{},
		warningKeys:   map[string]bool{},

		internalContextNames: internalContextNames(document),
	}
	ordered := sortedSlices(document.Slices)
	im.collect(ordered)
	return im.write(ordered), im.warnings, nil
}

// rejectMultipleActors fails when a slice lists more than one distinct actor:
// the native language gives a workflow's screens at most one actor each and
// the interchange format does not say which screen uses which actor.
func rejectMultipleActors(document *Document) error {
	for _, slice := range document.Slices {
		var names []string
		for _, actor := range slice.Actors {
			if !slices.Contains(names, actor.Name) {
				names = append(names, actor.Name)
			}
		}
		if len(names) > 1 {
			return fmt.Errorf("slice %q lists %d actors (%s); Event Modeling HCL cannot express more than one actor per slice", slice.Title, len(names), strings.Join(names, ", "))
		}
	}
	return nil
}

// internalContextNames lists every context name the document uses for
// internal content: slice contexts and the contexts of non-EXTERNAL events.
func internalContextNames(document *Document) map[string]bool {
	names := map[string]bool{}
	for _, slice := range document.Slices {
		if slice.Context != "" {
			names[slice.Context] = true
		}
		for _, event := range slice.Events {
			if name := firstNonEmpty(event.ModelContext, event.Domain); name != "" && event.Context != "EXTERNAL" {
				names[name] = true
			}
		}
	}
	return names
}

// contextLabel is the bounded_context label imported for a JSON context name.
// An external context named like an internal one gets an "_external" suffix,
// so Export can restore the name from the label and vice versa.
func contextLabel(name string, external bool, internalNames map[string]bool) string {
	label := slug(name, "context")
	if external && internalNames[name] {
		label += "_external"
	}
	return label
}

var labelPattern = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

var repeatedUnderscores = regexp.MustCompile(`_+`)

// ref locates an imported element: kind is the HCL block type, scope the
// owning workflow (or bounded_context for events), label its block label.
type ref struct {
	kind  string
	scope string
	label string
}

func (r ref) qualified() string { return r.kind + "." + r.scope + "." + r.label }

// from renders r as seen from workflow: workflow-local references drop the
// workflow segment; events always carry their context.
func (r ref) from(workflow string) string {
	if r.kind != "event" && r.scope == workflow {
		return r.kind + "." + r.label
	}
	return r.qualified()
}

type importedContext struct {
	id         string
	title      string
	internal   bool
	external   bool
	aggregates []string
	aggTitles  map[string]string
	events     []*Element
	eventRefs  map[*Element]ref
}

type importer struct {
	contextOrder []string
	contexts     map[string]*importedContext
	contextNames map[string]string
	ids          map[string]ref
	// eventRefs maps each event occurrence (original or copy) to the event it
	// denotes, so occurrences without ids keep their own identity.
	eventRefs    map[*Element]ref
	eventSources map[ref]*Element
	elementRefs  map[*Element]ref
	// elementsByRef inverts elementRefs for workflow elements.
	elementsByRef map[ref]*Element
	imageRefs     map[*ScreenImage]ref
	tableRefs     map[*Table]ref
	localIDs      map[string]map[string]ref
	workflowOf    map[*Slice]string
	labels        map[string]map[string]bool
	actors        map[string]string
	actorOrder    []Actor
	warnings      []string
	warningKeys   map[string]bool
	// internalContextNames feeds contextLabel; see internalContextNames.
	internalContextNames map[string]bool
}

func (im *importer) warn(format string, args ...any) {
	warning := fmt.Sprintf(format, args...)
	if !im.warningKeys[warning] {
		im.warningKeys[warning] = true
		im.warnings = append(im.warnings, warning)
	}
}

func sortedSlices(input []Slice) []*Slice {
	result := make([]*Slice, len(input))
	for index := range input {
		result[index] = &input[index]
	}
	sort.SliceStable(result, func(left, right int) bool {
		return indexOf(result[left]) < indexOf(result[right])
	})
	return result
}

func indexOf(slice *Slice) float64 {
	if slice.Index == nil {
		return math.Inf(1)
	}
	return *slice.Index
}

// unique reserves a lower_snake_case label within namespace, suffixing a
// counter on collision.
func (im *importer) unique(namespace, label string) string {
	used := im.labels[namespace]
	if used == nil {
		used = map[string]bool{}
		im.labels[namespace] = used
	}
	candidate := label
	for counter := 2; used[candidate]; counter++ {
		candidate = fmt.Sprintf("%s_%d", label, counter)
	}
	used[candidate] = true
	return candidate
}

// canonicalLabel returns the label encoded in a fully qualified identifier
// such as "command.<workflow>.<id>" that Export produces, when id has that shape.
func canonicalLabel(id, kind string, segments int) (string, bool) {
	parts := strings.Split(id, ".")
	if len(parts) != segments || parts[0] != kind {
		return "", false
	}
	for _, part := range parts[1:] {
		if !labelPattern.MatchString(part) {
			return "", false
		}
	}
	return parts[len(parts)-1], true
}

// slug converts free text or camelCase into a lower_snake_case label.
func slug(text, fallback string) string {
	var builder strings.Builder
	previousLower := false
	for _, character := range text {
		if character >= unicode.MaxASCII {
			builder.WriteRune('_')
			previousLower = false
			continue
		}
		switch {
		case unicode.IsUpper(character):
			if previousLower {
				builder.WriteRune('_')
			}
			builder.WriteRune(unicode.ToLower(character))
			previousLower = false
		case unicode.IsLetter(character) || unicode.IsDigit(character):
			builder.WriteRune(character)
			previousLower = unicode.IsLower(character) || unicode.IsDigit(character)
		default:
			builder.WriteRune('_')
			previousLower = false
		}
	}
	label := repeatedUnderscores.ReplaceAllString(builder.String(), "_")
	label = strings.Trim(label, "_")
	if label == "" {
		return fallback
	}
	if !unicode.IsLetter(rune(label[0])) {
		label = fallback + "_" + label
	}
	return label
}

func (im *importer) context(id string) *importedContext {
	if context, ok := im.contexts[id]; ok {
		return context
	}
	context := &importedContext{id: id, aggTitles: map[string]string{}, eventRefs: map[*Element]ref{}}
	im.contexts[id] = context
	im.contextOrder = append(im.contextOrder, id)
	if im.labels["bounded_context"] == nil {
		im.labels["bounded_context"] = map[string]bool{}
	}
	im.labels["bounded_context"][id] = true
	return context
}

// contextKey is the contextNames key of a context name: internal and
// external contexts of the same name are distinct identities.
func contextKey(title string, external bool) string {
	if external {
		return title + "\x00external"
	}
	return title
}

func (im *importer) namedContext(title string, external bool) *importedContext {
	key := contextKey(title, external)
	if id, ok := im.contextNames[key]; ok {
		return im.contexts[id]
	}
	if !external && labelPattern.MatchString(title) {
		if context, ok := im.contexts[title]; ok && (!context.external || context.internal) {
			im.contextNames[key] = context.id
			return context
		}
	}
	label := contextLabel(title, external, im.internalContextNames)
	context := im.context(im.unique("bounded_context", label))
	// A name equal to the label is an identifier, not a display title.
	if title != context.id && title != model.Humanize(context.id) {
		context.title = title
	}
	im.contextNames[key] = context.id
	return context
}

// defaultContext is the context for content that names none. It is never an
// external context: such content falls back to the first internal context,
// else to a uniquely labelled internal "main" context.
func (im *importer) defaultContext(slice *Slice) *importedContext {
	if slice.Context != "" {
		return im.namedContext(slice.Context, false)
	}
	for _, id := range im.contextOrder {
		if context := im.contexts[id]; !context.external || context.internal {
			return context
		}
	}
	return im.context(im.unique("bounded_context", "main"))
}

func (im *importer) collect(ordered []*Slice) {
	for _, slice := range ordered {
		label, ok := canonicalLabel(slice.ID, "workflow", 2)
		if !ok {
			label = slug(slice.Title, "workflow")
		}
		im.workflowOf[slice] = im.unique("workflow", label)
		if slice.Context != "" {
			im.defaultContext(slice)
		}
	}
	type eventOccurrence struct {
		slice *Slice
		event *Element
	}
	var copies []eventOccurrence
	for _, slice := range ordered {
		for index := range slice.Events {
			event := &slice.Events[index]
			if event.ElementCopy {
				copies = append(copies, eventOccurrence{slice, event})
			} else {
				im.collectEvent(slice, event)
			}
		}
	}
	for len(copies) > 0 {
		pending := copies[:0]
		resolved := false
		for _, copy := range copies {
			if target, ok := im.ids[copy.event.LinkedID]; ok && copy.event.LinkedID != "" && target.kind == "event" {
				im.warnEventSnapshot(target, copy.event)
				im.eventRefs[copy.event] = target
				if copy.event.ID != "" {
					if previous, exists := im.ids[copy.event.ID]; exists && previous != target {
						im.warn("event copy %q (%s) has an id conflicting with its linkedId %s; original identity retained", copy.event.Title, copy.event.ID, copy.event.LinkedID)
					} else {
						im.ids[copy.event.ID] = target
					}
				}
				resolved = true
			} else {
				pending = append(pending, copy)
			}
		}
		if !resolved {
			seed := 0
			for index, copy := range pending {
				if copy.event.LinkedID == copy.event.ID {
					seed = index
					break
				}
			}
			copy := pending[seed]
			im.warn("event copy %q (%s) cannot resolve linkedId %q; its supplied snapshot is imported as a separate event", copy.event.Title, copy.event.ID, copy.event.LinkedID)
			im.collectEvent(copy.slice, copy.event)
			pending = append(pending[:seed], pending[seed+1:]...)
		}
		copies = pending
	}
	for _, slice := range ordered {
		workflow := im.workflowOf[slice]
		im.localIDs[workflow] = map[string]ref{}
		for _, group := range []struct {
			kind     string
			elements []Element
		}{
			{"screen", slice.Screens}, {"command", slice.Commands}, {"readmodel", slice.ReadModels}, {"processor", slice.Processors},
		} {
			for index := range group.elements {
				im.collectElement(workflow, group.kind, &group.elements[index])
			}
		}
		for index := range slice.ScreenImages {
			if im.imageRefs == nil {
				im.imageRefs = map[*ScreenImage]ref{}
			}
			image := &slice.ScreenImages[index]
			im.imageRefs[image] = im.reserve(workflow, "screen_image", image.ID, image.Title)
		}
		for index := range slice.Tables {
			if im.tableRefs == nil {
				im.tableRefs = map[*Table]ref{}
			}
			table := &slice.Tables[index]
			im.tableRefs[table] = im.reserve(workflow, "table", table.ID, table.Title)
		}
		for _, actor := range slice.Actors {
			im.collectActor(actor)
		}
		for _, aggregate := range slice.Aggregates {
			im.aggregateRef(aggregate, im.defaultContext(slice), false)
		}
		if slice.Assignee != "" {
			im.warn("slice %q: assignee %q has no HCL equivalent; omitted", slice.Title, slice.Assignee)
		}
		if slice.TicketNumber != "" {
			im.warn("slice %q: ticketNumber %q has no HCL equivalent; omitted", slice.Title, slice.TicketNumber)
		}
		if len(slice.Storylines) > 0 {
			im.warn("slice %q: storylines have no HCL equivalent; omitted", slice.Title)
		}
	}
	im.warn("foreign JSON canvas ids map to canonical HCL labels; original foreign ids, source slice index, specification id/linkedId/index/specRow/vertical layout and copy placement are not retained")
}

func (im *importer) collectEvent(slice *Slice, event *Element) {
	im.warnElementType(event, "event")
	if existing, known := im.ids[event.ID]; known && event.ID != "" {
		if existing.kind != "event" {
			im.warn("event %q: id %q is already used by %s; omitted", event.Title, event.ID, existing.qualified())
		} else {
			im.eventRefs[event] = existing
		}
		im.warnEventSnapshot(existing, event)
		return
	}
	var context *importedContext
	label := ""
	parts := strings.Split(event.ID, ".")
	if len(parts) == 3 && parts[0] == "event" && labelPattern.MatchString(parts[1]) && labelPattern.MatchString(parts[2]) {
		context, label = im.context(parts[1]), parts[2]
		// An exported event's context name restores the context title when
		// that name is the one Export derives for this label.
		name := firstNonEmpty(event.ModelContext, event.Domain)
		if context.title == "" && name != context.id && name != model.Humanize(context.id) && contextLabel(name, event.Context == "EXTERNAL", im.internalContextNames) == context.id {
			context.title = name
		}
		// Later events that name this context share it instead of creating a
		// second context for the same name.
		if name != "" {
			key := contextKey(name, event.Context == "EXTERNAL")
			if _, known := im.contextNames[key]; !known {
				im.contextNames[key] = context.id
			}
		}
	} else {
		title := firstNonEmpty(event.ModelContext, event.Domain)
		external := event.Context == "EXTERNAL"
		if title == "" && external {
			title = "External"
		}
		if title == "" {
			context = im.defaultContext(slice)
		} else {
			context = im.namedContext(title, external)
		}
		label = slug(event.Title, "event")
	}
	reference := ref{kind: "event", scope: context.id, label: im.unique("event."+context.id, label)}
	if event.Context == "EXTERNAL" {
		context.external = true
	} else {
		context.internal = true
	}
	context.events = append(context.events, event)
	context.eventRefs[event] = reference
	im.eventSources[reference] = event
	im.eventRefs[event] = reference
	if event.ID != "" {
		im.ids[event.ID] = reference
	} else {
		im.warn("event %q has no id; dependency identity cannot be retained", event.Title)
	}
}

func (im *importer) collectElement(workflow, kind string, element *Element) {
	im.elementRefs[element] = im.reserve(workflow, kind, element.ID, element.Title)
	im.elementsByRef[im.elementRefs[element]] = element
	if element.ElementCopy && element.LinkedID != "" && element.LinkedID != element.ID {
		im.warn("element copy %q (%s): supplied snapshot imported locally; linkedId %q copy lineage has no HCL equivalent", element.Title, element.ID, element.LinkedID)
		if _, exists := im.localIDs[workflow][element.LinkedID]; !exists {
			im.localIDs[workflow][element.LinkedID] = im.elementRefs[element]
		}
	}
}

func (im *importer) reserve(workflow, kind, id, title string) ref {
	label, ok := canonicalLabel(id, kind, 3)
	if !ok {
		label = slug(title, kind)
	}
	reference := ref{kind: kind, scope: workflow, label: im.unique(kind+"."+workflow, label)}
	if id != "" {
		if _, exists := im.ids[id]; !exists {
			im.ids[id] = reference
		}
		if _, exists := im.localIDs[workflow][id]; exists {
			im.warn("workflow %q: duplicate element id %q; separate declarations retained, references use the first", workflow, id)
		} else {
			im.localIDs[workflow][id] = reference
		}
	} else {
		im.warn("element %q has no id; dependency identity cannot be retained", title)
	}
	return reference
}

func (im *importer) collectActor(actor Actor) {
	if len(actor.RolesRequired) > 0 || len(actor.Tags) > 0 {
		im.warn("actor %q: rolesRequired and tags have no HCL actor equivalent; omitted", actor.Name)
	}
	if _, ok := im.actors[actor.Name]; ok {
		for _, previous := range im.actorOrder {
			if previous.Name == actor.Name && previous.AuthRequired != actor.AuthRequired {
				im.warn("actor %q has conflicting authentication requirements; first declaration retained", actor.Name)
			}
		}
		return
	}
	im.actors[actor.Name] = im.unique("actor", slug(actor.Name, "actor"))
	im.actorOrder = append(im.actorOrder, actor)
}

func (im *importer) resolve(id, workflow string) (ref, bool) {
	if id == "" {
		return ref{}, false
	}
	if reference, ok := im.localIDs[workflow][id]; ok {
		return reference, true
	}
	reference, ok := im.ids[id]
	return reference, ok
}

// aggregateRef registers an aggregate by title and returns its reference.
// preferred is the context to create it in when no context declares it yet.
func (im *importer) aggregateRef(title string, preferred *importedContext, local bool) string {
	if title == "" {
		return ""
	}
	owner := preferred
	if _, ok := preferred.aggTitles[title]; !ok {
		for _, id := range im.contextOrder {
			if _, ok := im.contexts[id].aggTitles[title]; ok {
				owner = im.contexts[id]
				break
			}
		}
	}
	label, ok := owner.aggTitles[title]
	if !ok {
		label = im.unique("aggregate."+owner.id, slug(title, "aggregate"))
		owner.aggTitles[title] = label
		owner.aggregates = append(owner.aggregates, title)
	}
	if local && owner == preferred {
		return "aggregate." + label
	}
	return "aggregate." + owner.id + "." + label
}

// edge is one resolved flow between two imported elements.
type edge struct{ from, to ref }

func (im *importer) edges(ordered []*Slice) []edge {
	var result []edge
	seen := map[edge]bool{}
	for _, slice := range ordered {
		workflow := im.workflowOf[slice]
		for _, group := range [][]Element{slice.Screens, slice.Commands, slice.Events, slice.ReadModels, slice.Processors} {
			for index := range group {
				element := &group[index]
				self, ok := im.elementRefs[element]
				if !ok {
					self, ok = im.eventRefs[element]
				}
				if !ok && element.ID != "" {
					self, ok = im.ids[element.ID]
				}
				if !ok {
					continue
				}
				for _, dependency := range element.Dependencies {
					other, ok := im.resolve(dependency.ID, workflow)
					if !ok {
						im.warn("%q depends on unknown element %q (%s); dependency omitted", element.Title, dependency.Title, dependency.ID)
						continue
					}
					if im.warnDependencyKind(element, dependency, other) {
						continue
					}
					candidate := edge{from: other, to: self}
					if dependency.Type == "OUTBOUND" {
						candidate = edge{from: self, to: other}
					}
					if candidate.from == candidate.to {
						im.warn("element %q: self dependency %q has no native flow equivalent; omitted", element.Title, dependency.ID)
					} else if !seen[candidate] {
						seen[candidate] = true
						result = append(result, candidate)
					}
				}
			}
		}
	}
	return result
}

// allowedFlowRoots mirrors the validator's canonical flow grammar.
func allowedFlowRoots(workflowType, ownerKind, direction string) []string {
	switch workflowType {
	case "state_change":
		switch {
		case ownerKind == "screen" && direction == "to":
			return []string{"command"}
		case ownerKind == "command" && direction == "to":
			return []string{"event"}
		}
	case "state_view":
		switch {
		case ownerKind == "readmodel" && direction == "from":
			return []string{"event"}
		case ownerKind == "readmodel" && direction == "to":
			return []string{"screen"}
		}
	case "automation", "translation":
		switch {
		case ownerKind == "readmodel" && direction == "from":
			return []string{"event"}
		case ownerKind == "readmodel" && direction == "to":
			return []string{"processor"}
		case ownerKind == "processor" && direction == "from":
			return []string{"event"}
		case ownerKind == "processor" && direction == "to":
			return []string{"command"}
		case ownerKind == "command" && direction == "to":
			return []string{"event"}
		}
	}
	return nil
}

type flows struct {
	to   map[ref][]ref
	from map[ref][]ref
}

func (im *importer) placeFlows(edges []edge, kinds map[string]string) flows {
	placed := flows{to: map[ref][]ref{}, from: map[ref][]ref{}}
	for _, flow := range edges {
		if flow.from.kind != "event" && !workflowAllows(kinds[flow.from.scope], flow.from.kind) || flow.to.kind != "event" && !workflowAllows(kinds[flow.to.scope], flow.to.kind) {
			im.warn("flow %s -> %s targets an element incompatible with its workflow kind; omitted", flow.from.qualified(), flow.to.qualified())
			continue
		}
		switch {
		case flow.from.kind != "event" && slices.Contains(allowedFlowRoots(kinds[flow.from.scope], flow.from.kind, "to"), flow.to.kind):
			placed.to[flow.from] = append(placed.to[flow.from], flow.to)
		case flow.to.kind != "event" && slices.Contains(allowedFlowRoots(kinds[flow.to.scope], flow.to.kind, "from"), flow.from.kind):
			placed.from[flow.to] = append(placed.from[flow.to], flow.from)
		default:
			im.warn("flow %s -> %s is not allowed by the Event Modeling HCL flow grammar; omitted", flow.from.qualified(), flow.to.qualified())
		}
	}
	for _, lists := range []map[ref][]ref{placed.to, placed.from} {
		for owner, targets := range lists {
			im.orderByOwnerDependencies(owner, targets)
		}
	}
	return placed
}

// orderByOwnerDependencies sorts the references written on owner's to/from
// attribute into the order owner lists them as dependencies, so a reference
// list keeps its author order however the flow was first discovered.
func (im *importer) orderByOwnerDependencies(owner ref, targets []ref) {
	element := im.elementsByRef[owner]
	if element == nil {
		return
	}
	position := map[ref]int{}
	for index, dependency := range element.Dependencies {
		if target, ok := im.resolve(dependency.ID, owner.scope); ok {
			if _, seen := position[target]; !seen {
				position[target] = index
			}
		}
	}
	sort.SliceStable(targets, func(left, right int) bool {
		leftPosition, leftKnown := position[targets[left]]
		rightPosition, rightKnown := position[targets[right]]
		return leftKnown && (!rightKnown || leftPosition < rightPosition)
	})
}

var workflowKinds = map[string]string{"STATE_CHANGE": "state_change", "STATE_VIEW": "state_view", "AUTOMATION": "automation", "TRANSLATION": "translation"}

func (im *importer) write(ordered []*Slice) []byte {
	file := hclwrite.NewEmptyFile()
	root := file.Body()
	edges := im.edges(ordered)
	kinds := map[string]string{}
	for _, slice := range ordered {
		kinds[im.workflowOf[slice]] = workflowKinds[slice.SliceType]
	}
	placed := im.placeFlows(edges, kinds)

	// Workflow bodies are built first so element aggregates register on
	// contexts before the contexts are written.
	workflowBlocks := make([]*hclwrite.Block, 0, len(ordered))
	for _, slice := range ordered {
		workflowBlocks = append(workflowBlocks, im.workflowBlock(slice, kinds[im.workflowOf[slice]], placed, kinds))
	}
	contextBlocks := make([]*hclwrite.Block, 0, len(im.contextOrder))
	for _, id := range im.contextOrder {
		contextBlocks = append(contextBlocks, im.contextBlock(im.contexts[id]))
	}

	for _, actor := range im.actorOrder {
		block := root.AppendNewBlock("actor", []string{im.actors[actor.Name]})
		block.Body().SetAttributeValue("title", cty.StringVal(actor.Name))
		block.Body().SetAttributeValue("auth_required", cty.BoolVal(actor.AuthRequired))
		root.AppendNewline()
	}
	for _, block := range append(contextBlocks, workflowBlocks...) {
		root.AppendBlock(block)
		root.AppendNewline()
	}
	return file.Bytes()
}

func (im *importer) contextBlock(context *importedContext) *hclwrite.Block {
	block := hclwrite.NewBlock("bounded_context", []string{context.id})
	body := block.Body()
	if context.title != "" {
		body.SetAttributeValue("title", cty.StringVal(context.title))
	}
	if context.external && context.internal {
		im.warn("bounded context %q has conflicting INTERNAL/EXTERNAL ownership for the same canonical context id; internal ownership retained", context.id)
	}
	if context.external && !context.internal {
		body.SetAttributeValue("external", cty.True)
	}
	// Event aggregates register while events are written, so build events first.
	eventBlocks := make([]*hclwrite.Block, 0, len(context.events))
	for _, event := range context.events {
		eventBlock := hclwrite.NewBlock("event", []string{context.eventRefs[event].label})
		im.elementAttributes(eventBlock.Body(), event, "event", context, true)
		im.fieldBlocks(eventBlock.Body(), "field", event.Fields)
		eventBlocks = append(eventBlocks, eventBlock)
	}
	for _, title := range context.aggregates {
		aggregate := body.AppendNewBlock("aggregate", []string{context.aggTitles[title]})
		aggregate.Body().SetAttributeValue("title", cty.StringVal(title))
	}
	for _, eventBlock := range eventBlocks {
		body.AppendBlock(eventBlock)
	}
	return block
}

func (im *importer) workflowBlock(slice *Slice, kind string, placed flows, kinds map[string]string) *hclwrite.Block {
	workflow := im.workflowOf[slice]
	block := hclwrite.NewBlock(kind, []string{workflow})
	body := block.Body()
	body.SetAttributeValue("title", cty.StringVal(slice.Title))
	if slice.Status != "" {
		body.SetAttributeValue("status", cty.StringVal(slug(slice.Status, "status")))
	}
	var context *importedContext
	if slice.Context != "" {
		context = im.defaultContext(slice)
		body.SetAttributeTraversal("owner", traversal("bounded_context."+context.id))
	}
	// Import has already rejected slices listing more than one actor.
	soleActor := ""
	if len(slice.Actors) > 0 {
		soleActor = im.actors[slice.Actors[0].Name]
	}
	for _, group := range []struct {
		kind     string
		elements []Element
	}{{"screen", slice.Screens}, {"command", slice.Commands}, {"readmodel", slice.ReadModels}, {"processor", slice.Processors}} {
		for index := range group.elements {
			element := &group.elements[index]
			self := im.elementRefs[element]
			if !workflowAllows(kind, group.kind) {
				im.warnIllegalChild(slice, kind, group.kind, element.Title)
				continue
			}
			child := body.AppendNewBlock(group.kind, []string{self.label}).Body()
			if element.Aggregate != "" {
				context = im.defaultContext(slice)
			}
			for _, dependency := range element.AggregateDependencies {
				if dependency != "" {
					context = im.defaultContext(slice)
					break
				}
			}
			im.elementAttributes(child, element, group.kind, context, false)
			if group.kind == "readmodel" {
				child.SetAttributeValue("question", cty.StringVal(firstNonEmpty(element.Description, element.Title)))
			}
			if group.kind == "screen" && soleActor != "" {
				child.SetAttributeTraversal("actor", traversal("actor."+soleActor))
			}
			setReferences(child, "from", placed.from[self], workflow)
			setReferences(child, "to", placed.to[self], workflow)
			im.fieldBlocks(child, "field", element.Fields)
		}
	}
	for index := range slice.ScreenImages {
		image := &slice.ScreenImages[index]
		if !workflowAllows(kind, "screen_image") {
			im.warnIllegalChild(slice, kind, "screen_image", image.Title)
			continue
		}
		child := body.AppendNewBlock("screen_image", []string{im.imageRefs[image].label}).Body()
		child.SetAttributeValue("title", cty.StringVal(image.Title))
		if image.URL != "" {
			child.SetAttributeValue("url", cty.StringVal(image.URL))
		}
	}
	for index := range slice.Tables {
		table := &slice.Tables[index]
		child := body.AppendNewBlock("table", []string{im.tableRefs[table].label}).Body()
		child.SetAttributeValue("title", cty.StringVal(table.Title))
		im.fieldBlocks(child, "field", table.Fields)
	}
	for _, specification := range slice.Specifications {
		if specification.Vertical {
			im.warn("specification %q: vertical layout has no native equivalent; omitted", specification.Title)
		}
		if specification.LinkedID != "" && specification.LinkedID != slice.ID {
			im.warn("specification %q: linkedId %q attachment metadata has no native equivalent; enclosing workflow and explicit step targets retained", specification.Title, specification.LinkedID)
		}
		for _, step := range specification.Then {
			if step.Type == "SPEC_ERROR" && step.LinkedID != "" {
				im.warn("specification %q: error step %q linkedId %q has no native equivalent; error text and fields retained", specification.Title, step.Title, step.LinkedID)
			}
		}
		if prepared, ok := im.prepareSpecification(workflow, kind, specification, kinds); ok {
			im.scenarioBlock(body, workflow, prepared)
		}
	}
	return block
}

func (im *importer) elementAttributes(body *hclwrite.Body, element *Element, kind string, context *importedContext, isEvent bool) {
	im.warnElementType(element, kind)
	if isEvent && (element.APIEndpoint != "" || element.CreatesAggregate || len(element.Triggers) > 0) {
		im.warn("event %q: apiEndpoint, createsAggregate and triggers have no native event equivalent; omitted", element.Title)
	}
	body.SetAttributeValue("title", cty.StringVal(element.Title))
	if element.Description != "" {
		body.SetAttributeValue("description", cty.StringVal(element.Description))
	}
	if element.GroupID != "" {
		body.SetAttributeValue("group_id", cty.StringVal(element.GroupID))
	}
	if len(element.Tags) > 0 {
		body.SetAttributeValue("tags", stringList(element.Tags))
	}
	if element.Aggregate != "" {
		body.SetAttributeTraversal("aggregate", traversal(im.aggregateRef(element.Aggregate, context, isEvent)))
	}
	if len(element.AggregateDependencies) > 0 {
		references := make([]hclwrite.Tokens, 0, len(element.AggregateDependencies))
		im.warnEmptyAggregate(element)
		for _, dependency := range element.AggregateDependencies {
			if dependency == "" {
				continue
			}
			references = append(references, hclwrite.TokensForTraversal(traversal(im.aggregateRef(dependency, context, isEvent))))
		}
		body.SetAttributeRaw("aggregate_dependencies", hclwrite.TokensForTuple(references))
	}
	if element.APIEndpoint != "" && !isEvent {
		body.SetAttributeValue("api_endpoint", cty.StringVal(element.APIEndpoint))
	}
	if element.Service != nil {
		body.SetAttributeValue("service", cty.StringVal(*element.Service))
	}
	if element.CreatesAggregate && !isEvent {
		body.SetAttributeValue("creates_aggregate", cty.True)
	}
	if len(element.Triggers) > 0 && !isEvent {
		body.SetAttributeValue("triggers", stringList(element.Triggers))
	}
	if element.Sketched {
		body.SetAttributeValue("sketched", cty.True)
	}
	if element.ListElement {
		body.SetAttributeValue("list_element", cty.True)
	}
	if tokens, ok := jsonTokens(element.Prototype); ok {
		body.SetAttributeRaw("prototype", tokens)
	}
}

func (im *importer) fieldBlocks(body *hclwrite.Body, blockType string, fields []Field) {
	used := map[string]bool{}
	for _, field := range fields {
		label := slug(field.Name, "field")
		candidate := label
		for counter := 2; used[candidate]; counter++ {
			candidate = fmt.Sprintf("%s_%d", label, counter)
		}
		used[candidate] = true
		im.warnFieldRename(field.Name, candidate)
		child := body.AppendNewBlock(blockType, []string{candidate}).Body()
		fieldType := field.Type
		if fieldType == "Number" {
			fieldType = "Double"
		}
		child.SetAttributeValue("type", cty.StringVal(fieldType))
		if field.Cardinality != "" {
			child.SetAttributeValue("cardinality", cty.StringVal(field.Cardinality))
		}
		if field.Mapping != "" {
			child.SetAttributeValue("mapping", cty.StringVal(field.Mapping))
		}
		if field.Schema != "" {
			child.SetAttributeValue("schema", cty.StringVal(field.Schema))
		}
		for _, flag := range []struct {
			name string
			set  bool
		}{{"optional", field.Optional}, {"technical_attribute", field.TechnicalAttribute}, {"generated", field.Generated}, {"id_attribute", field.IDAttribute}, {"pii", field.PII}} {
			if flag.set {
				child.SetAttributeValue(flag.name, cty.True)
			}
		}
		if tokens, ok := im.fieldExample(field, fieldType, field.Cardinality); ok {
			child.SetAttributeRaw("example", tokens)
		}
		im.fieldBlocks(child, "subfield", field.Subfields)
	}
}

// exampleValue undoes the schema's string-or-object restriction: a string
// example for a non-textual or List field that holds JSON text (as Export
// writes numbers, booleans and lists) is decoded back to that JSON value.
func exampleValue(raw json.RawMessage, fieldType, cardinality string) json.RawMessage {
	var text string
	if json.Unmarshal(raw, &text) != nil {
		return raw
	}
	textual := slices.Contains([]string{"String", "UUID", "Date", "DateTime"}, fieldType)
	if textual && cardinality != "List" || !json.Valid([]byte(text)) {
		return raw
	}
	return json.RawMessage(text)
}

func (im *importer) scenarioBlock(body *hclwrite.Body, workflow string, specification Specification) {
	label, ok := canonicalLabel(specification.ID, "scenario", 3)
	if !ok {
		label = slug(specification.Title, "scenario")
	}
	scenario := body.AppendNewBlock("scenario", []string{im.unique("scenario."+workflow, label)}).Body()
	scenario.SetAttributeValue("title", cty.StringVal(specification.Title))
	for _, phase := range []struct {
		kind  string
		steps []SpecificationStep
	}{{"given", specification.Given}, {"when", specification.When}, {"then", specification.Then}} {
		for _, step := range phase.steps {
			im.stepBlock(scenario, workflow, phase.kind, step)
		}
	}
	for _, comment := range specification.Comments {
		scenario.AppendNewBlock("comment", nil).Body().SetAttributeValue("description", cty.StringVal(comment.Description))
	}
}

func (im *importer) stepBlock(scenario *hclwrite.Body, workflow, kind string, step SpecificationStep) {
	target, resolved := im.stepTarget(workflow, step)
	if !resolved {
		return
	} // prepareSpecification already reports this loss.
	body := scenario.AppendNewBlock(kind, nil).Body()
	if step.Type == "SPEC_ERROR" {
		body.SetAttributeValue("error", cty.StringVal(step.Title))
	} else {
		body.SetAttributeValue("title", cty.StringVal(step.Title))
		body.SetAttributeTraversal(target.kind, traversal(target.from(workflow)))
	}
	if len(step.Tags) > 0 {
		body.SetAttributeValue("tags", stringList(step.Tags))
	}
	if examples := objectList(step.Examples); len(examples) > 0 {
		body.SetAttributeRaw("examples", hclwrite.TokensForTuple(examples))
	}
	if step.ExpectEmptyList {
		body.SetAttributeValue("expect_empty_list", cty.True)
	}
	im.fieldBlocks(body, "field", step.Fields)
}

// objectList converts step examples, which the schema guarantees are objects.
func objectList(raw []json.RawMessage) []hclwrite.Tokens {
	values := make([]hclwrite.Tokens, 0, len(raw))
	for _, item := range raw {
		if tokens, ok := jsonTokens(item); ok {
			values = append(values, tokens)
		}
	}
	return values
}

// plainKey matches object keys written bare; every other key is quoted.
var plainKey = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// reservedKeys are identifier-shaped keys that HCL reads as keywords.
var reservedKeys = map[string]bool{"for": true, "in": true, "if": true, "true": true, "false": true, "null": true}

// jsonTokens renders a JSON value as HCL expression tokens without routing
// it through cty types, which would normalize Unicode object keys and so
// merge keys that are distinct in the source. It streams the JSON so object
// keys keep their source order, which export reproduces.
func jsonTokens(raw json.RawMessage) (hclwrite.Tokens, bool) {
	if len(bytes.TrimSpace(raw)) == 0 || !json.Valid(raw) {
		return nil, false
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	return valueTokens(decoder)
}

// valueTokens renders the next JSON value read from decoder.
func valueTokens(decoder *json.Decoder) (hclwrite.Tokens, bool) {
	token, err := decoder.Token()
	if err != nil {
		return nil, false
	}
	switch typed := token.(type) {
	case nil:
		return hclwrite.TokensForValue(cty.NullVal(cty.DynamicPseudoType)), true
	case bool:
		return hclwrite.TokensForValue(cty.BoolVal(typed)), true
	case string:
		return hclwrite.TokensForValue(cty.StringVal(typed)), true
	case json.Number:
		number, _, err := big.ParseFloat(typed.String(), 10, 512, big.ToNearestEven)
		if err != nil {
			return nil, false
		}
		return hclwrite.TokensForValue(cty.NumberVal(number)), true
	case json.Delim:
		if typed == '[' {
			var items []hclwrite.Tokens
			for decoder.More() {
				item, ok := valueTokens(decoder)
				if !ok {
					return nil, false
				}
				items = append(items, item)
			}
			if _, err := decoder.Token(); err != nil {
				return nil, false
			}
			return hclwrite.TokensForTuple(items), true
		}
		tokens := hclwrite.Tokens{{Type: hclsyntax.TokenOBrace, Bytes: []byte("{")}}
		if decoder.More() {
			tokens = append(tokens, &hclwrite.Token{Type: hclsyntax.TokenNewline, Bytes: []byte("\n")})
		}
		for decoder.More() {
			key, err := decoder.Token()
			if err != nil {
				return nil, false
			}
			item, ok := valueTokens(decoder)
			if !ok {
				return nil, false
			}
			tokens = append(tokens, keyTokens(key.(string))...)
			tokens = append(tokens, &hclwrite.Token{Type: hclsyntax.TokenEqual, Bytes: []byte("=")})
			tokens = append(tokens, item...)
			tokens = append(tokens, &hclwrite.Token{Type: hclsyntax.TokenNewline, Bytes: []byte("\n")})
		}
		if _, err := decoder.Token(); err != nil {
			return nil, false
		}
		return append(tokens, &hclwrite.Token{Type: hclsyntax.TokenCBrace, Bytes: []byte("}")}), true
	}
	return nil, false
}

// keyTokens writes an object key bare when it is a plain identifier and as a
// quoted string otherwise. The quoted form is built here, not by cty, so the
// key text stays exactly as supplied.
func keyTokens(key string) hclwrite.Tokens {
	if plainKey.MatchString(key) && !reservedKeys[key] {
		return hclwrite.Tokens{{Type: hclsyntax.TokenIdent, Bytes: []byte(key)}}
	}
	var escaped strings.Builder
	for index, character := range key {
		switch character {
		case '\n':
			escaped.WriteString(`\n`)
		case '\r':
			escaped.WriteString(`\r`)
		case '\t':
			escaped.WriteString(`\t`)
		case '"':
			escaped.WriteString(`\"`)
		case '\\':
			escaped.WriteString(`\\`)
		case '$', '%':
			escaped.WriteRune(character)
			if strings.HasPrefix(key[index+1:], "{") {
				escaped.WriteRune(character)
			}
		default:
			switch {
			case unicode.IsPrint(character):
				escaped.WriteRune(character)
			case character < 65536:
				fmt.Fprintf(&escaped, `\u%04x`, character)
			default:
				fmt.Fprintf(&escaped, `\U%08x`, character)
			}
		}
	}
	tokens := hclwrite.Tokens{{Type: hclsyntax.TokenOQuote, Bytes: []byte(`"`)}}
	if escaped.Len() > 0 {
		tokens = append(tokens, &hclwrite.Token{Type: hclsyntax.TokenQuotedLit, Bytes: []byte(escaped.String())})
	}
	return append(tokens, &hclwrite.Token{Type: hclsyntax.TokenCQuote, Bytes: []byte(`"`)})
}

func jsonToCty(raw json.RawMessage) (cty.Value, bool) {
	if len(bytes.TrimSpace(raw)) == 0 {
		return cty.NilVal, false
	}
	impliedType, err := ctyjson.ImpliedType(raw)
	if err != nil {
		return cty.NilVal, false
	}
	value, err := ctyjson.Unmarshal(raw, impliedType)
	if err != nil {
		return cty.NilVal, false
	}
	return value, true
}

// keyNeutral rewrites every object key in raw as its position ("0", "1", …).
// Example type checks only inspect value kinds, and cty normalizes Unicode
// keys, so checking the original keys could merge distinct ones and reject
// a valid example. It returns nil when raw is not valid JSON.
func keyNeutral(raw json.RawMessage) json.RawMessage {
	if len(bytes.TrimSpace(raw)) == 0 || !json.Valid(raw) {
		return nil
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var out bytes.Buffer
	if !writeKeyNeutral(decoder, &out) {
		return nil
	}
	return out.Bytes()
}

func writeKeyNeutral(decoder *json.Decoder, out *bytes.Buffer) bool {
	token, err := decoder.Token()
	if err != nil {
		return false
	}
	delim, isDelim := token.(json.Delim)
	if !isDelim {
		encoded, err := json.Marshal(token)
		if err != nil {
			return false
		}
		out.Write(encoded)
		return true
	}
	object := delim == '{'
	if object {
		out.WriteByte('{')
	} else {
		out.WriteByte('[')
	}
	for index := 0; decoder.More(); index++ {
		if index > 0 {
			out.WriteByte(',')
		}
		if object {
			if _, err := decoder.Token(); err != nil {
				return false
			}
			fmt.Fprintf(out, "%q:", strconv.Itoa(index))
		}
		if !writeKeyNeutral(decoder, out) {
			return false
		}
	}
	if _, err := decoder.Token(); err != nil {
		return false
	}
	if object {
		out.WriteByte('}')
	} else {
		out.WriteByte(']')
	}
	return true
}

func stringList(values []string) cty.Value {
	items := make([]cty.Value, len(values))
	for index, value := range values {
		items[index] = cty.StringVal(value)
	}
	return cty.TupleVal(items)
}

func traversal(reference string) hcl.Traversal {
	parts := strings.Split(reference, ".")
	result := hcl.Traversal{hcl.TraverseRoot{Name: parts[0]}}
	for _, part := range parts[1:] {
		result = append(result, hcl.TraverseAttr{Name: part})
	}
	return result
}

func setReferences(body *hclwrite.Body, name string, references []ref, workflow string) {
	if len(references) == 0 {
		return
	}
	tokens := make([]hclwrite.Tokens, 0, len(references))
	for _, reference := range references {
		tokens = append(tokens, hclwrite.TokensForTraversal(traversal(reference.from(workflow))))
	}
	body.SetAttributeRaw(name, hclwrite.TokensForTuple(tokens))
}
