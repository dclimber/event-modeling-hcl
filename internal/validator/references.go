package validator

import (
	"fmt"
	"slices"
	"strings"

	"github.com/hashicorp/hcl/v2"

	"github.com/event-modeling-hcl/eventmodeling-hcl/internal/source"
	"github.com/event-modeling-hcl/eventmodeling-hcl/internal/syntax"
)

func (v *modelValidator) validateOwnerReference(attribute *hcl.Attribute) hcl.Diagnostics {
	traversal, diagnostics := absoluteTraversal(attribute)
	if diagnostics.HasErrors() {
		return diagnostics
	}
	parts, ok := traversalParts(traversal)
	if !ok || len(parts) != 2 || !slices.Contains([]string{"bounded_context", "team", "system"}, parts[0]) {
		return hcl.Diagnostics{invalidReferenceShape(attribute, "owner must reference bounded_context.<id>, team.<id>, or system.<id>.")}
	}
	if !v.referenceExists(parts, "") {
		return hcl.Diagnostics{unresolvedReference(attribute, parts)}
	}
	return nil
}

func (v *modelValidator) validateGlobalReference(attribute *hcl.Attribute) hcl.Diagnostics {
	traversal, diagnostics := absoluteTraversal(attribute)
	if diagnostics.HasErrors() {
		return diagnostics
	}
	parts, ok := traversalParts(traversal)
	if !ok || !v.referenceExists(parts, "") {
		return hcl.Diagnostics{unresolvedReference(attribute, parts)}
	}
	return nil
}

func (v *modelValidator) validateReference(attribute *hcl.Attribute, expectedKind, workflowID string) hcl.Diagnostics {
	traversal, diagnostics := absoluteTraversal(attribute)
	if diagnostics.HasErrors() {
		return diagnostics
	}
	parts, ok := traversalParts(traversal)
	if !ok || len(parts) < 2 || parts[0] != expectedKind {
		return hcl.Diagnostics{invalidReferenceShape(attribute, fmt.Sprintf("reference must target %s.<id>.", expectedKind))}
	}
	if !v.referenceExists(parts, workflowID) {
		return hcl.Diagnostics{unresolvedReference(attribute, parts)}
	}
	return nil
}

func (v *modelValidator) validateReferenceList(attribute *hcl.Attribute, expectedKind, workflowID string) hcl.Diagnostics {
	expressions, diagnostics := hcl.ExprList(attribute.Expr)
	if diagnostics.HasErrors() {
		return diagnostics
	}
	for _, expression := range expressions {
		traversal, traversalDiagnostics := hcl.AbsTraversalForExpr(expression)
		diagnostics = append(diagnostics, traversalDiagnostics...)
		if traversalDiagnostics.HasErrors() {
			continue
		}
		parts, ok := traversalParts(traversal)
		if !ok || len(parts) < 2 || parts[0] != expectedKind {
			diagnostics = append(diagnostics, errorDiagnostic(codeInvalidReference, expression.Range(), "Invalid reference", fmt.Sprintf("reference must target %s.<id>.", expectedKind)))
			continue
		}
		if !v.referenceExists(parts, workflowID) {
			diagnostics = append(diagnostics, errorDiagnostic(codeUnresolvedReference, expression.Range(), "Unresolved reference", fmt.Sprintf("%s is not declared in the model.", strings.Join(parts, "."))))
		}
	}
	return diagnostics
}

// validateChapterRange enforces the contiguous source-order chapter rule of a
// one-file model. A multi-file model orders workflows by its chapters instead,
// so the rule does not apply there.
func (v *modelValidator) validateChapterRange(attribute *hcl.Attribute) hcl.Diagnostics {
	expressions, diagnostics := hcl.ExprList(attribute.Expr)
	if diagnostics.HasErrors() || len(expressions) < 2 || v.document.Parsed().FileCount() > 1 {
		return diagnostics
	}
	positions := make(map[string]int, len(v.index.workflowOrder))
	for index, workflow := range v.index.workflowOrder {
		positions[workflow] = index
	}
	previous := -1
	for _, expression := range expressions {
		traversal, traversalDiagnostics := hcl.AbsTraversalForExpr(expression)
		if traversalDiagnostics.HasErrors() {
			continue
		}
		parts, ok := traversalParts(traversal)
		if !ok || len(parts) != 2 || parts[0] != "workflow" {
			continue
		}
		position, exists := positions[parts[1]]
		if !exists {
			continue
		}
		if previous >= 0 && position != previous+1 {
			return hcl.Diagnostics{errorDiagnostic(codeInvalidChapterRange, attribute.Expr.Range(), "Invalid chapter range", "chapter workflows must be contiguous and listed in source order.")}
		}
		previous = position
	}
	return nil
}

// validateComposition checks the rules that only apply to a model made of
// several files: chapters live in one file, no workflow is in two chapters,
// and every workflow is in a chapter. References that do not resolve are
// already reported as EM102 and take no part in these checks.
func (v *modelValidator) validateComposition(blocks hcl.Blocks) hcl.Diagnostics {
	var diagnostics hcl.Diagnostics
	chapterFile := ""
	chaptered := map[string]string{}
	for _, block := range blocks {
		if block.Type != "chapter" || len(block.Labels) == 0 {
			continue
		}
		if chapterFile == "" {
			chapterFile = block.DefRange.Filename
		}
		if block.DefRange.Filename != chapterFile {
			diagnostics = append(diagnostics, errorDiagnostic(codeChaptersInSeveralFiles, block.DefRange, "Chapters in several files", fmt.Sprintf("all chapter blocks of a multi-file model must be in one file; first chapter file is %s.", chapterFile)))
		}
		diagnostics = append(diagnostics, v.collectChapterWorkflows(block, chaptered)...)
	}
	for _, workflow := range v.index.workflowOrder {
		if _, ok := chaptered[workflow]; !ok {
			diagnostics = append(diagnostics, warningDiagnostic(codeUnchapteredWorkflow, v.index.definitionRanges[workflow], "Workflow outside every chapter", fmt.Sprintf("workflow.%s is in no chapter; it is placed after chaptered workflows in file name order. Add it to a chapter.", workflow)))
		}
	}
	return diagnostics
}

// collectChapterWorkflows records each declared workflow listed by a chapter in
// chaptered, mapping it to that chapter's id. A workflow already recorded for
// an earlier chapter block is reported as EM014 on the later list item.
func (v *modelValidator) collectChapterWorkflows(chapter *hcl.Block, chaptered map[string]string) hcl.Diagnostics {
	content, _, _ := syntax.PartialContent(chapter.Body, syntax.ChapterSchema())
	attribute := content.Attributes["workflows"]
	if attribute == nil {
		return nil
	}
	expressions, listDiagnostics := hcl.ExprList(attribute.Expr)
	if listDiagnostics.HasErrors() {
		return nil
	}
	var diagnostics hcl.Diagnostics
	listed := map[string]bool{}
	for _, expression := range expressions {
		traversal, traversalDiagnostics := hcl.AbsTraversalForExpr(expression)
		if traversalDiagnostics.HasErrors() {
			continue
		}
		parts, ok := traversalParts(traversal)
		if !ok || len(parts) != 2 || parts[0] != "workflow" {
			continue
		}
		if _, declared := v.index.workflows[parts[1]]; !declared {
			continue
		}
		if owner, exists := chaptered[parts[1]]; exists && !listed[parts[1]] {
			diagnostics = append(diagnostics, errorDiagnostic(codeWorkflowInSeveralChapters, expression.Range(), "Workflow in several chapters", fmt.Sprintf("workflow.%s is already in chapter.%s.", parts[1], owner)))
			continue
		}
		chaptered[parts[1]] = chapter.Labels[0]
		listed[parts[1]] = true
	}
	return diagnostics
}

func (v *modelValidator) validateFlowReferences(attribute *hcl.Attribute, workflowType, workflowID, ownerKind, direction string) hcl.Diagnostics {
	expressions, diagnostics := hcl.ExprList(attribute.Expr)
	if diagnostics.HasErrors() {
		return diagnostics
	}
	allowedRoots := allowedFlowRoots(workflowType, ownerKind, direction)
	for _, expression := range expressions {
		traversal, traversalDiagnostics := hcl.AbsTraversalForExpr(expression)
		diagnostics = append(diagnostics, traversalDiagnostics...)
		if traversalDiagnostics.HasErrors() {
			continue
		}
		parts, ok := traversalParts(traversal)
		if !ok || len(parts) < 2 || !slices.Contains(allowedRoots, parts[0]) {
			diagnostics = append(diagnostics, errorDiagnostic(codeInvalidFlowReference, expression.Range(), "Invalid flow reference", flowReferenceDetail(ownerKind, direction, workflowType, allowedRoots, parts)))
			continue
		}
		if !v.referenceExists(parts, workflowID) {
			diagnostics = append(diagnostics, errorDiagnostic(codeUnresolvedReference, expression.Range(), "Unresolved reference", fmt.Sprintf("%s is not declared in the model.", strings.Join(parts, "."))))
		}
	}
	return diagnostics
}

func flowReferenceDetail(ownerKind, direction, workflowType string, allowedRoots, referenced []string) string {
	actual := strings.Join(referenced, ".")
	if len(allowedRoots) == 0 {
		return fmt.Sprintf("%s.%s has no canonical flow targets in a %s workflow. You referenced: %s.", ownerKind, direction, workflowType, actual)
	}
	expected := make([]string, 0, len(allowedRoots))
	for _, root := range allowedRoots {
		expected = append(expected, flowReferenceTargetShape(root))
	}
	return fmt.Sprintf("%s.%s may reference: %s. You referenced: %s.", ownerKind, direction, strings.Join(expected, " or "), actual)
}

func flowReferenceTargetShape(root string) string {
	if root == "event" {
		return "event.<context>.<id>"
	}
	return root + ".<id>"
}

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

func (v *modelValidator) referenceExists(parts []string, workflowID string) bool {
	if len(parts) == 2 {
		switch parts[0] {
		case "actor":
			return v.index.actors[parts[1]]
		case "bounded_context":
			return v.index.boundedContexts[parts[1]]
		case "system":
			return v.index.systems[parts[1]]
		case "team":
			return v.index.teams[parts[1]]
		case "workflow":
			_, ok := v.index.workflows[parts[1]]
			return ok
		case "command", "readmodel", "screen", "processor":
			workflow, ok := v.index.workflows[workflowID]
			return ok && workflow.elements[parts[0]][parts[1]]
		}
	}
	if len(parts) == 3 {
		address := parts[1] + "." + parts[2]
		switch parts[0] {
		case "event":
			return v.index.catalogEvents[address]
		case "aggregate":
			return v.index.aggregates[address]
		case "field_type":
			_, ok := v.index.fieldTypes[address]
			return ok
		case "command", "readmodel", "screen", "processor":
			workflow, ok := v.index.workflows[parts[1]]
			return ok && workflow.elements[parts[0]][parts[2]]
		}
	}
	return false
}

func (v *contextValidator) validateAggregate(attribute *hcl.Attribute, allowLocal bool) hcl.Diagnostics {
	traversal, diagnostics := absoluteTraversal(attribute)
	if diagnostics.HasErrors() {
		return diagnostics
	}
	parts, ok := traversalParts(traversal)
	if !ok || parts[0] != "aggregate" || !validLocalOrQualifiedLength(parts, allowLocal) {
		return hcl.Diagnostics{invalidReferenceShape(attribute, "aggregate must be an aggregate.<context>.<name> reference; context-owned events may use aggregate.<name>.")}
	}
	address := ""
	if len(parts) == 2 {
		address = v.contextID + "." + parts[1]
	} else {
		address = parts[1] + "." + parts[2]
	}
	if !v.model.index.aggregates[address] {
		return hcl.Diagnostics{unresolvedReference(attribute, parts)}
	}
	return nil
}

func validLocalOrQualifiedLength(parts []string, allowLocal bool) bool {
	return len(parts) == 3 || allowLocal && len(parts) == 2
}

func (v *contextValidator) validateAggregateList(attribute *hcl.Attribute, allowLocal bool) hcl.Diagnostics {
	expressions, diagnostics := hcl.ExprList(attribute.Expr)
	if diagnostics.HasErrors() {
		return diagnostics
	}
	for _, expression := range expressions {
		diagnostics = append(diagnostics, v.validateAggregate(&hcl.Attribute{Expr: expression}, allowLocal)...)
	}
	return diagnostics
}

func (v *contextValidator) fieldTypeAddress(attribute *hcl.Attribute) (string, hcl.Diagnostics) {
	traversal, diagnostics := absoluteTraversal(attribute)
	if diagnostics.HasErrors() {
		return "", diagnostics
	}
	parts, ok := traversalParts(traversal)
	if !ok || parts[0] != "field_type" || len(parts) != 2 && len(parts) != 3 {
		return "", hcl.Diagnostics{invalidReferenceShape(attribute, "field type must be a field_type.<context>.<name> reference; context-owned fields may use field_type.<name>.")}
	}
	address := ""
	if len(parts) == 2 {
		if v.contextID == "" {
			return "", hcl.Diagnostics{invalidReferenceShape(attribute, "workflow fields must use field_type.<context>.<name>.")}
		}
		address = v.contextID + "." + parts[1]
	} else {
		address = parts[1] + "." + parts[2]
	}
	if _, exists := v.model.index.fieldTypes[address]; !exists {
		return address, hcl.Diagnostics{unresolvedReference(attribute, parts)}
	}
	return address, nil
}

// inferredFieldTypeAddress resolves the field_type a typeless field block infers
// from its own name. A field owned by a bounded_context (event or subfield)
// resolves against that context; a workflow-element field resolves the unique
// document-wide field_type of the same name and errors on absence or ambiguity.
func (v *contextValidator) inferredFieldTypeAddress(name string, subject hcl.Range) (string, hcl.Diagnostics) {
	if address := v.model.document.InferFieldType(v.contextID, name); address != "" {
		address = strings.TrimPrefix(address, "field_type.")
		if _, exists := v.model.index.fieldTypes[address]; !exists {
			return "", hcl.Diagnostics{errorDiagnostic(codeInvalidFieldType, subject, "Unresolved field type", fmt.Sprintf("field %q has no type and bounded_context %q declares no field_type %q.", name, v.contextID, name))}
		}
		return address, nil
	}
	contexts := v.model.document.FieldTypeContexts(name)
	switch len(contexts) {
	case 0:
		return "", hcl.Diagnostics{errorDiagnostic(codeInvalidFieldType, subject, "Unresolved field type", fmt.Sprintf("field %q has no type and no field_type %q is declared in any bounded_context.", name, name))}
	default:
		return "", hcl.Diagnostics{errorDiagnostic(codeInvalidFieldType, subject, "Ambiguous field type", fmt.Sprintf("field %q has no type and field_type %q is declared in multiple bounded_contexts (%s); add an explicit type.", name, name, strings.Join(contexts, ", ")))}
	}
}

func absoluteTraversal(attribute *hcl.Attribute) (hcl.Traversal, hcl.Diagnostics) {
	return source.AbsoluteTraversal(attribute.Expr)
}

func traversalParts(traversal hcl.Traversal) ([]string, bool) {
	return source.TraversalParts(traversal)
}

func invalidReferenceShape(attribute *hcl.Attribute, detail string) *hcl.Diagnostic {
	return errorDiagnostic(codeInvalidReference, attribute.Expr.Range(), "Invalid reference", detail)
}

func unresolvedReference(attribute *hcl.Attribute, parts []string) *hcl.Diagnostic {
	return errorDiagnostic(codeUnresolvedReference, attribute.Expr.Range(), "Unresolved reference", fmt.Sprintf("%s is not declared in the model.", strings.Join(parts, ".")))
}
