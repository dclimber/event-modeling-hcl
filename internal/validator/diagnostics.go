package validator

import "github.com/hashicorp/hcl/v2"

type diagnosticCode string

const (
	codeUnclassified diagnosticCode = "EM000"

	codeReadFile                  diagnosticCode = "EM001"
	codeDuplicateID               diagnosticCode = "EM002"
	codeInvalidBlockLabel         diagnosticCode = "EM003"
	codeInvalidWorkflowChild      diagnosticCode = "EM004"
	codeInvalidChapter            diagnosticCode = "EM005"
	codeInvalidChapterRange       diagnosticCode = "EM006"
	codeInvalidAttribute          diagnosticCode = "EM007"
	codeInvalidEnum               diagnosticCode = "EM008"
	codeInvalidFieldType          diagnosticCode = "EM009"
	codeInvalidExample            diagnosticCode = "EM010"
	codeInvalidTranslation        diagnosticCode = "EM011"
	codeInvalidAutomation         diagnosticCode = "EM012"
	codeChaptersInSeveralFiles    diagnosticCode = "EM013"
	codeWorkflowInSeveralChapters diagnosticCode = "EM014"

	codeInvalidReference    diagnosticCode = "EM101"
	codeUnresolvedReference diagnosticCode = "EM102"

	codeInvalidFlowReference diagnosticCode = "EM201"

	codeInvalidScenario       diagnosticCode = "EM301"
	codeInvalidScenarioStep   diagnosticCode = "EM302"
	codeInvalidScenarioTarget diagnosticCode = "EM303"

	codeBedAntiPattern        diagnosticCode = "EM401"
	codeLeftChairAntiPattern  diagnosticCode = "EM402"
	codeRightChairAntiPattern diagnosticCode = "EM403"
	codeCommandWithoutReason  diagnosticCode = "EM404"
	codeShelfAntiPattern      diagnosticCode = "EM405"
	codeOpenHotspot           diagnosticCode = "EM406"
	codeUnchapteredWorkflow   diagnosticCode = "EM407"
)

var profileSeverities = map[Profile]map[diagnosticCode]hcl.DiagnosticSeverity{
	Workshop: {
		codeBedAntiPattern:        hcl.DiagInvalid,
		codeLeftChairAntiPattern:  hcl.DiagInvalid,
		codeRightChairAntiPattern: hcl.DiagInvalid,
		codeCommandWithoutReason:  hcl.DiagInvalid,
		codeShelfAntiPattern:      hcl.DiagInvalid,
		codeOpenHotspot:           hcl.DiagInvalid,
		codeUnchapteredWorkflow:   hcl.DiagInvalid,
	},
	Strict: {
		codeCommandWithoutReason: hcl.DiagError,
		codeOpenHotspot:          hcl.DiagError,
		codeUnchapteredWorkflow:  hcl.DiagError,
	},
}

// Profile controls the severity assigned to Event Modeling judgment diagnostics.
type Profile uint8

const (
	Workshop Profile = iota
	Valid
	Strict
)

func (p Profile) String() string {
	switch p {
	case Workshop:
		return "workshop"
	case Valid:
		return "valid"
	case Strict:
		return "strict"
	default:
		return "valid"
	}
}

// ParseProfile converts a CLI profile name into its typed representation.
func ParseProfile(value string) (Profile, bool) {
	switch value {
	case "workshop":
		return Workshop, true
	case "valid":
		return Valid, true
	case "strict":
		return Strict, true
	default:
		return Valid, false
	}
}

func diag(code diagnosticCode, severity hcl.DiagnosticSeverity, subject hcl.Range, summary, detail string) *hcl.Diagnostic {
	return &hcl.Diagnostic{Severity: severity, Summary: summary, Detail: detail, Subject: &subject, Extra: code}
}

func errorDiagnostic(code diagnosticCode, subject hcl.Range, summary, detail string) *hcl.Diagnostic {
	return diag(code, hcl.DiagError, subject, summary, detail)
}

func warningDiagnostic(code diagnosticCode, subject hcl.Range, summary, detail string) *hcl.Diagnostic {
	return diag(code, hcl.DiagWarning, subject, summary, detail)
}

// DiagnosticCode returns a stable Event Modeling code, or EM000 for diagnostics
// produced by HCL before Event Modeling validation begins.
func DiagnosticCode(diagnostic *hcl.Diagnostic) string {
	if code, ok := diagnostic.Extra.(diagnosticCode); ok {
		return string(code)
	}
	return string(codeUnclassified)
}

// DiagnosticCodeExtra provides an HCL diagnostic Extra value with an Event
// Modeling code for adapters that construct diagnostics outside this package.
func DiagnosticCodeExtra(code string) interface{} {
	return diagnosticCode(code)
}

func applyProfile(diagnostics hcl.Diagnostics, profile Profile) hcl.Diagnostics {
	severityOverrides, ok := profileSeverities[profile]
	if !ok {
		return diagnostics
	}
	for _, diagnostic := range diagnostics {
		if severity, exists := severityOverrides[diagnosticCode(DiagnosticCode(diagnostic))]; exists {
			diagnostic.Severity = severity
		}
	}
	return diagnostics
}
