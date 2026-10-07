package interchange

import (
	"math/big"
	"reflect"
	"sort"
	"strings"

	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/zclconf/go-cty/cty"
)

func workflowAllows(kind, child string) bool {
	if child == "table" || child == "scenario" {
		return true
	}
	switch kind {
	case "state_change":
		return child == "screen" || child == "command" || child == "screen_image"
	case "state_view":
		return child == "screen" || child == "readmodel" || child == "screen_image"
	case "automation", "translation":
		return child == "processor" || child == "readmodel" || child == "command"
	}
	return false
}

func scenarioAllows(kind, phase, target string) bool {
	switch kind {
	case "state_change":
		return phase == "given" && target == "event" || phase == "when" && target == "command" || phase == "then" && (target == "event" || target == "error")
	case "state_view":
		return phase == "given" && target == "event" || phase == "then" && (target == "readmodel" || target == "error")
	case "automation", "translation":
		return phase == "given" && (target == "event" || target == "readmodel") || phase == "when" && (target == "command" || target == "processor") || phase == "then" && (target == "event" || target == "error")
	}
	return false
}

func (im *importer) stepTarget(workflow string, step SpecificationStep) (ref, bool) {
	if step.Type == "SPEC_ERROR" {
		return ref{kind: "error"}, true
	}
	target, ok := im.resolve(step.LinkedID, workflow)
	if !ok {
		return ref{}, false
	}
	valid := step.Type == "SPEC_EVENT" && target.kind == "event" || step.Type == "SPEC_READMODEL" && target.kind == "readmodel" || step.Type == "SPEC_COMMAND" && (target.kind == "command" || target.kind == "processor")
	return target, valid
}

func (im *importer) prepareSpecification(workflow, kind string, specification Specification, kinds map[string]string) (Specification, bool) {
	result := specification
	result.Given, result.When, result.Then = nil, nil, nil
	invalidWhen := false
	for _, phase := range []struct {
		name   string
		input  []SpecificationStep
		output *[]SpecificationStep
	}{
		{"given", specification.Given, &result.Given}, {"when", specification.When, &result.When}, {"then", specification.Then, &result.Then},
	} {
		steps := append([]SpecificationStep(nil), phase.input...)
		sort.SliceStable(steps, func(i, j int) bool {
			if steps[i].Index == nil {
				return false
			}
			if steps[j].Index == nil {
				return true
			}
			return *steps[i].Index < *steps[j].Index
		})
		for _, step := range steps {
			target, valid := im.stepTarget(workflow, step)
			if !valid {
				im.warn("specification %q: %s step %q (%s) cannot resolve linkedId %q to the declared target kind; step omitted", specification.Title, phase.name, step.Title, step.Type, step.LinkedID)
			} else if !scenarioAllows(kind, phase.name, target.kind) {
				im.warn("specification %q: %s target %s is not allowed in %s; step omitted", specification.Title, phase.name, target.kind, kind)
				valid = false
			} else if target.kind != "event" && target.kind != "error" && !workflowAllows(kinds[target.scope], target.kind) {
				im.warn("specification %q: %s step %q targets an element omitted from its incompatible workflow; step omitted", specification.Title, phase.name, step.Title)
				valid = false
			}
			if valid {
				*phase.output = append(*phase.output, step)
			} else if phase.name == "when" {
				invalidWhen = true
			}
		}
	}
	if invalidWhen || len(result.Then) == 0 || kind == "state_view" && (len(result.Given) == 0 || len(result.When) != 0) || kind != "state_view" && len(result.When) != 1 {
		im.warn("specification %q has no valid %s scenario shape after unsupported steps; entire specification omitted", specification.Title, kind)
		return result, false
	}
	return result, true
}

// fieldExample validates a field example against its native type with cty and
// returns it as tokens built from the original JSON, so object keys survive
// exactly.
func (im *importer) fieldExample(field Field, fieldType, cardinality string) (hclwrite.Tokens, bool) {
	if len(field.Example) == 0 {
		return nil, false
	}
	raw := exampleValue(field.Example, fieldType, cardinality)
	value, ok := jsonToCty(keyNeutral(raw))
	tokens, tokensOK := jsonTokens(raw)
	if !ok || !tokensOK {
		im.warn("field %q: example %s cannot be represented as a native value; omitted", field.Name, field.Example)
		return nil, false
	}
	if value.IsNull() {
		return tokens, true
	}
	if cardinality == "List" {
		if fieldType == "Custom" && value.Type().IsObjectType() {
			im.warn("field %q: singleton object example imported as a one-item List example", field.Name)
			return hclwrite.TokensForTuple([]hclwrite.Tokens{tokens}), true
		}
		if value.Type().IsTupleType() || value.Type().IsListType() {
			iterator := value.ElementIterator()
			valid := true
			for iterator.Next() {
				_, item := iterator.Element()
				if !exampleMatchesType(item, fieldType) {
					valid = false
					break
				}
			}
			if valid {
				return tokens, true
			}
		}
	} else if exampleMatchesType(value, fieldType) {
		return tokens, true
	}
	im.warn("field %q: example %s does not match native %s/%s and is omitted", field.Name, field.Example, fieldType, firstNonEmpty(cardinality, "Single"))
	return nil, false
}

func exampleMatchesType(value cty.Value, fieldType string) bool {
	if value.IsNull() {
		return true
	}
	switch fieldType {
	case "String", "UUID", "Date", "DateTime":
		return value.Type().Equals(cty.String)
	case "Boolean":
		return value.Type().Equals(cty.Bool)
	case "Double", "Decimal":
		return value.Type().Equals(cty.Number)
	case "Int", "Long":
		if !value.Type().Equals(cty.Number) {
			return false
		}
		_, accuracy := value.AsBigFloat().Int(nil)
		return accuracy == big.Exact
	case "Custom":
		return value.Type().IsObjectType()
	}
	return false
}

func elementTypeMatches(kind, elementType string) bool {
	if kind == "processor" {
		return elementType == "AUTOMATION"
	}
	return elementType == strings.ToUpper(kind)
}

func (im *importer) warnElementType(element *Element, kind string) {
	if !elementTypeMatches(kind, element.Type) {
		im.warn("element %q: type %q disagrees with its %s collection; collection kind retained", element.Title, element.Type, kind)
	}
}

func (im *importer) warnFieldRename(name, label string) {
	if name != label {
		im.warn("field name %q mapped to native HCL label %q; original field spelling is not retained", name, label)
	}
}

func (im *importer) warnIllegalChild(slice *Slice, kind, child, title string) {
	im.warn("slice %q: %s %q is not allowed in %s workflows; element and its incompatible flows omitted", slice.Title, child, title, kind)
}

func (im *importer) warnDependencyKind(element *Element, dependency Dependency, other ref) bool {
	if elementTypeMatches(other.kind, dependency.ElementType) {
		return false
	}
	im.warn("element %q: dependency %q declares elementType %q but resolves to %s; dependency omitted", element.Title, dependency.ID, dependency.ElementType, other.qualified())
	return true
}

func (im *importer) warnEmptyAggregate(element *Element) {
	for _, dependency := range element.AggregateDependencies {
		if dependency == "" {
			im.warn("element %q: empty aggregate dependency has no resolvable native target; omitted", element.Title)
		}
	}
}

func (im *importer) warnEventSnapshot(reference ref, incoming *Element) {
	original := im.eventSources[reference]
	if original == nil {
		return
	}
	// Dependencies are merged separately. Placement and copy bookkeeping
	// cannot be attributes of a single canonical catalog event.
	first, second := *original, *incoming
	first.ID, second.ID = "", ""
	first.Dependencies, second.Dependencies = nil, nil
	first.Slice, second.Slice = "", ""
	first.ElementCopy, second.ElementCopy = false, false
	first.LinkedID, second.LinkedID = "", ""
	if !reflect.DeepEqual(first, second) {
		im.warn("event %q (%s) repeats %s with different supplied data; canonical event data retained, copy-specific overrides have no HCL equivalent", incoming.Title, incoming.ID, reference.qualified())
	}
}
