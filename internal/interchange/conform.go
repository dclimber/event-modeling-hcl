package interchange

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"slices"
	"sort"
	"strconv"
	"strings"
)

// The constraints below transcribe the published eventmodeling.schema.json
// (additionalProperties false everywhere, required properties, enums and the
// string-or-object field example). Import accepts exactly the documents that
// schema accepts, and Export checks its own output against the same rules, so
// both directions share one definition of the interchange contract.

type valueKind int

const (
	kindString valueKind = iota
	kindNullableString
	kindBoolean
	kindInteger
	kindObject
	kindStringOrObject
	kindArray
	kindDefinition
)

type valueRule struct {
	kind       valueKind
	enum       []string
	items      *valueRule
	definition string
}

type objectRule struct {
	properties map[string]valueRule
	required   []string
}

var (
	stringRule      = valueRule{kind: kindString}
	booleanRule     = valueRule{kind: kindBoolean}
	integerRule     = valueRule{kind: kindInteger}
	stringArrayRule = arrayOf(stringRule)
)

func arrayOf(item valueRule) valueRule { return valueRule{kind: kindArray, items: &item} }

func definition(name string) valueRule { return valueRule{kind: kindDefinition, definition: name} }

func enum(values ...string) valueRule { return valueRule{kind: kindString, enum: values} }

var elementTypes = enum("COMMAND", "EVENT", "READMODEL", "SCREEN", "AUTOMATION")

var schemaDefinitions = map[string]objectRule{
	"Document": {
		properties: map[string]valueRule{"slices": arrayOf(definition("Slice"))},
		required:   []string{"slices"},
	},
	"Slice": {
		properties: map[string]valueRule{
			"id":             stringRule,
			"status":         enum("Created", "Done", "InProgress", "Assigned", "Review", "Blocked", "Planned", "Informational"),
			"index":          integerRule,
			"title":          stringRule,
			"context":        stringRule,
			"sliceType":      enum("STATE_CHANGE", "STATE_VIEW", "AUTOMATION", "TRANSLATION"),
			"assignee":       stringRule,
			"ticketNumber":   stringRule,
			"commands":       arrayOf(definition("Element")),
			"events":         arrayOf(definition("Element")),
			"readmodels":     arrayOf(definition("Element")),
			"screens":        arrayOf(definition("Element")),
			"screenImages":   arrayOf(definition("ScreenImage")),
			"processors":     arrayOf(definition("Element")),
			"tables":         arrayOf(definition("Table")),
			"specifications": arrayOf(definition("Specification")),
			"storylines":     arrayOf(definition("Storyline")),
			"actors":         arrayOf(definition("Actor")),
			"aggregates":     stringArrayRule,
		},
		required: []string{"id", "title", "sliceType", "commands", "events", "readmodels", "screens", "processors", "tables", "specifications"},
	},
	"Element": {
		properties: map[string]valueRule{
			"groupId":               stringRule,
			"id":                    stringRule,
			"tags":                  stringArrayRule,
			"domain":                stringRule,
			"modelContext":          stringRule,
			"context":               enum("INTERNAL", "EXTERNAL"),
			"slice":                 stringRule,
			"title":                 stringRule,
			"fields":                arrayOf(definition("Field")),
			"type":                  elementTypes,
			"description":           stringRule,
			"aggregate":             stringRule,
			"aggregateDependencies": stringArrayRule,
			"dependencies":          arrayOf(definition("Dependency")),
			"apiEndpoint":           stringRule,
			"service":               {kind: kindNullableString},
			"createsAggregate":      booleanRule,
			"triggers":              stringArrayRule,
			"sketched":              booleanRule,
			"prototype":             {kind: kindObject},
			"listElement":           booleanRule,
			"linkedId":              stringRule,
			"elementCopy":           booleanRule,
		},
		required: []string{"id", "title", "fields", "type", "dependencies"},
	},
	"ScreenImage": {
		properties: map[string]valueRule{"id": stringRule, "title": stringRule, "url": stringRule},
		required:   []string{"id", "title"},
	},
	"Table": {
		properties: map[string]valueRule{"id": stringRule, "title": stringRule, "fields": arrayOf(definition("Field"))},
		required:   []string{"id", "title", "fields"},
	},
	"Specification": {
		properties: map[string]valueRule{
			"vertical":  booleanRule,
			"id":        stringRule,
			"sliceName": stringRule,
			"title":     stringRule,
			"given":     arrayOf(definition("SpecificationStep")),
			"when":      arrayOf(definition("SpecificationStep")),
			"then":      arrayOf(definition("SpecificationStep")),
			"comments":  arrayOf(definition("Comment")),
			"linkedId":  stringRule,
		},
		required: []string{"id", "title", "given", "when", "then", "linkedId"},
	},
	"SpecificationStep": {
		properties: map[string]valueRule{
			"title":           stringRule,
			"tags":            stringArrayRule,
			"examples":        arrayOf(valueRule{kind: kindObject}),
			"id":              stringRule,
			"index":           integerRule,
			"specRow":         integerRule,
			"type":            enum("SPEC_EVENT", "SPEC_COMMAND", "SPEC_READMODEL", "SPEC_ERROR"),
			"fields":          arrayOf(definition("Field")),
			"linkedId":        stringRule,
			"expectEmptyList": booleanRule,
		},
		required: []string{"title", "id", "type"},
	},
	"Comment": {
		properties: map[string]valueRule{"description": stringRule},
		required:   []string{"description"},
	},
	"Actor": {
		properties: map[string]valueRule{"name": stringRule, "authRequired": booleanRule, "rolesRequired": stringArrayRule, "tags": stringArrayRule},
		required:   []string{"name", "authRequired"},
	},
	"Storyline": {
		properties: map[string]valueRule{"id": stringRule, "title": stringRule, "elements": arrayOf(definition("Element"))},
		required:   []string{"id", "title", "elements"},
	},
	"Dependency": {
		properties: map[string]valueRule{"id": stringRule, "type": enum("INBOUND", "OUTBOUND"), "title": stringRule, "elementType": elementTypes},
		required:   []string{"id", "type", "title", "elementType"},
	},
	"Field": {
		properties: map[string]valueRule{
			"name":               stringRule,
			"type":               enum("String", "Boolean", "Double", "Decimal", "Long", "Custom", "Date", "DateTime", "UUID", "Int", "Number"),
			"example":            {kind: kindStringOrObject},
			"subfields":          arrayOf(definition("Field")),
			"mapping":            stringRule,
			"optional":           booleanRule,
			"technicalAttribute": booleanRule,
			"generated":          booleanRule,
			"idAttribute":        booleanRule,
			"pii":                booleanRule,
			"schema":             stringRule,
			"cardinality":        enum("List", "Single"),
		},
		required: []string{"name", "type"},
	},
}

// maxConformanceErrors bounds the report; one violation already rejects.
const maxConformanceErrors = 20

// CheckConformance reports every violation of the interchange schema in
// data (one JSON value), in ajv-like "instancePath: message" form. A nil
// result means the document conforms.
func CheckConformance(data []byte) []string {
	_, violations := conform(data)
	return violations
}

// conform checks data and returns it decoded generically, with integral
// numbers such as 2.0 (integers to JSON Schema) rewritten as 2 so the typed
// decoder accepts them too.
func conform(data []byte) (any, []string) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return nil, []string{err.Error()}
	}
	if _, err := decoder.Token(); err != io.EOF {
		return nil, []string{"trailing content after the JSON document"}
	}
	checker := &conformance{}
	checker.object("", value, "Document")
	return value, checker.violations
}

type conformance struct{ violations []string }

func (c *conformance) fail(path, format string, args ...any) {
	if len(c.violations) < maxConformanceErrors {
		c.violations = append(c.violations, fmt.Sprintf("%s: %s", displayPath(path), fmt.Sprintf(format, args...)))
	}
}

func displayPath(path string) string {
	if path == "" {
		return "/"
	}
	return path
}

func (c *conformance) object(path string, value any, name string) {
	rule := schemaDefinitions[name]
	members, ok := value.(map[string]any)
	if !ok {
		c.fail(path, "must be object")
		return
	}
	for _, property := range rule.required {
		if _, present := members[property]; !present {
			c.fail(path, "must have required property %q", property)
		}
	}
	keys := make([]string, 0, len(members))
	for key := range members {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		property, known := rule.properties[key]
		if !known {
			c.fail(path, "must NOT have additional property %q", key)
			continue
		}
		members[key] = c.value(path+"/"+key, members[key], property)
	}
}

func (c *conformance) value(path string, value any, rule valueRule) any {
	switch rule.kind {
	case kindString:
		text, ok := value.(string)
		if !ok {
			c.fail(path, "must be string")
		} else if len(rule.enum) > 0 && !slices.Contains(rule.enum, text) {
			c.fail(path, "must be equal to one of the allowed values: %s", strings.Join(rule.enum, ", "))
		}
	case kindNullableString:
		if _, ok := value.(string); !ok && value != nil {
			c.fail(path, "must be string,null")
		}
	case kindBoolean:
		if _, ok := value.(bool); !ok {
			c.fail(path, "must be boolean")
		}
	case kindInteger:
		if integer, ok := integral(value); ok {
			return integer
		}
		c.fail(path, "must be integer")
	case kindObject:
		if _, ok := value.(map[string]any); !ok {
			c.fail(path, "must be object")
		}
	case kindStringOrObject:
		_, text := value.(string)
		_, object := value.(map[string]any)
		if !text && !object {
			c.fail(path, "must be string or object")
		}
	case kindArray:
		items, ok := value.([]any)
		if !ok {
			c.fail(path, "must be array")
			return value
		}
		for index, item := range items {
			items[index] = c.value(fmt.Sprintf("%s/%d", path, index), item, *rule.items)
		}
	case kindDefinition:
		c.object(path, value, rule.definition)
	}
	return value
}

// integral judges a number the way ajv does, as an IEEE-754 double: it is an
// integer when finite and without a fractional part (so 2.0 and 1e-400, which
// underflows to 0, qualify while 1e309 does not). The canonical decimal form
// is returned for the typed decoder.
func integral(value any) (json.Number, bool) {
	number, ok := value.(json.Number)
	if !ok {
		return "", false
	}
	parsed, err := strconv.ParseFloat(number.String(), 64)
	if err != nil && !errors.Is(err, strconv.ErrRange) || math.IsInf(parsed, 0) || math.Trunc(parsed) != parsed {
		return "", false
	}
	return json.Number(strconv.FormatFloat(parsed, 'f', -1, 64)), true
}
