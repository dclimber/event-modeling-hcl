package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/event-modeling-hcl/eventmodeling-hcl/internal/app"
)

func TestParseCommand_RecognizesValidateInvocation(t *testing.T) {
	// Given a validate invocation with an Event Modeling model path.
	args := []string{"validate", "model.em.hcl"}

	// When its command is parsed.
	command, err := parseCommand("emhcl", args)

	// Then it produces a validate command without an error.
	if err != nil {
		t.Fatalf("err = %q, want nil", err)
	}
	if got, want := command, (cliCommand{kind: validateCommand, path: "model.em.hcl", profile: app.Valid}); got != want {
		t.Fatalf("command = %#v, want %#v", got, want)
	}
}

func TestParseCommand_RecognizesValidationProfile(t *testing.T) {
	command, err := parseCommand("emhcl", []string{"validate", "--profile", "strict", "model.em.hcl"})

	if err != nil {
		t.Fatalf("err = %q, want nil", err)
	}
	if got, want := command, (cliCommand{kind: validateCommand, path: "model.em.hcl", profile: app.Strict}); got != want {
		t.Fatalf("command = %#v, want %#v", got, want)
	}
}

func TestParseCommand_RecognizesFormatterInvocations(t *testing.T) {
	for _, test := range []struct {
		args  []string
		write bool
	}{
		{args: []string{"fmt", "model.em.hcl"}},
		{args: []string{"fmt", "-w", "model.em.hcl"}, write: true},
	} {
		command, err := parseCommand("emhcl", test.args)
		if err != nil {
			t.Fatalf("parseCommand(%q): %v", test.args, err)
		}
		if command.kind != formatCommand || command.path != "model.em.hcl" || command.write != test.write {
			t.Fatalf("command = %#v", command)
		}
	}
}

func TestParseCommand_RecognizesDiagramInvocations(t *testing.T) {
	tests := []struct {
		args   []string
		output string
	}{
		{args: []string{"diagram", "model.em.hcl"}},
		{args: []string{"diagram", "model.em.hcl", "-o", "model.html"}, output: "model.html"},
		{args: []string{"diagram", "--output", "model.html", "model.em.hcl"}, output: "model.html"},
	}
	for _, test := range tests {
		command, err := parseCommand("emhcl", test.args)
		if err != nil {
			t.Fatalf("parseCommand(%q): %v", test.args, err)
		}
		if command.kind != diagramCommand || command.path != "model.em.hcl" || command.output != test.output {
			t.Fatalf("command = %#v", command)
		}
	}
}

func TestParseCommand_RecognizesServeInvocation(t *testing.T) {
	// Given a bare serve invocation with just a model path.
	args := []string{"serve", "model.em.hcl"}

	// When its command is parsed.
	command, err := parseCommand("emhcl", args)

	// Then it produces a serve command with the documented defaults.
	if err != nil {
		t.Fatalf("err = %q, want nil", err)
	}
	want := cliCommand{kind: serveCommand, path: "model.em.hcl", profile: app.Valid, addr: defaultServeAddr, port: defaultServePort}
	if command != want {
		t.Fatalf("command = %#v, want %#v", command, want)
	}
}

func TestParseCommand_RejectsServePortsOutsideTheTCPRange(t *testing.T) {
	for _, port := range []string{"-1", "65536"} {
		_, err := parseCommand("emhcl", []string{"serve", "--port", port, "model.em.hcl"})
		if err == nil || !strings.Contains(err.Error(), "between 0 and 65535") {
			t.Errorf("port %s error = %v, want TCP range error", port, err)
		}
	}
}

func TestParseCommand_RecognizesServeFlags(t *testing.T) {
	// Given a serve invocation overriding every optional flag, in any order.
	args := []string{"serve", "--addr", "127.0.0.1", "--port", "9090", "--profile", "strict", "model.em.hcl"}

	// When its command is parsed.
	command, err := parseCommand("emhcl", args)

	// Then every flag is reflected in the resulting command.
	if err != nil {
		t.Fatalf("err = %q, want nil", err)
	}
	want := cliCommand{kind: serveCommand, path: "model.em.hcl", profile: app.Strict, addr: "127.0.0.1", port: 9090}
	if command != want {
		t.Fatalf("command = %#v, want %#v", command, want)
	}
}

func TestParseCommand_RejectsServeWithoutModelPath(t *testing.T) {
	_, err := parseCommand("emhcl", []string{"serve"})
	if err == nil {
		t.Fatal("expected an error for a serve invocation with no model path")
	}
}

func TestParseCommand_RejectsServeNonNumericPort(t *testing.T) {
	_, err := parseCommand("emhcl", []string{"serve", "--port", "notanumber", "model.em.hcl"})
	if err == nil {
		t.Fatal("expected an error for a non-numeric --port")
	}
}

func TestParseCommand_RejectsServeInvalidProfile(t *testing.T) {
	_, err := parseCommand("emhcl", []string{"serve", "--profile", "bogus", "model.em.hcl"})
	if got, want := err, "profile must be workshop, valid, or strict"; got == nil || got.Error() != want {
		t.Fatalf("err = %v, want %q", got, want)
	}
}

func TestParseCommand_RejectsServeUnsupportedExtension(t *testing.T) {
	_, err := parseCommand("emhcl", []string{"serve", "model.hcl"})
	if got, want := err, "model file must use the .em.hcl extension"; got == nil || got.Error() != want {
		t.Fatalf("err = %v, want %q", got, want)
	}
}

func TestUsageMessage_DocumentsServe(t *testing.T) {
	name := "emhcl"
	want := "usage: " + name + " <validate [--profile workshop|valid|strict] | fmt [-w] | diagram | serve> <model.em.hcl> [diagram: -o <file>] [serve: --addr <host> --port <n> --profile <p>]"
	if got := usageMessage(name); got != want {
		t.Fatalf("usageMessage(%q) = %q, want %q", name, got, want)
	}
}

func TestParseCommand_RejectsUnsupportedExtension(t *testing.T) {
	// Given a validate invocation with a non-Event-Modeling extension.
	args := []string{"validate", "model.hcl"}

	// When its command is parsed.
	_, err := parseCommand("emhcl", args)

	// Then it produces the extension error.
	if got, want := err, "model file must use the .em.hcl extension"; got == nil || got.Error() != want {
		t.Fatalf("err = %v, want %q", got, want)
	}
}

func TestFormatDiagnostics_FormatsSourceLocatedError(t *testing.T) {
	// Given one source-located validation error.
	diagnostics := []app.Diagnostic{{
		Code:     "EM007",
		Severity: "Error",
		Summary:  "Invalid attribute value",
		Detail:   "title must be a string.",
		Filename: "model.em.hcl",
		Line:     3,
		Column:   11,
	}}

	// When its diagnostics are formatted.
	got := formatDiagnostics(diagnostics)

	// Then it returns the CLI rendering without writing to a stream.
	const want = "model.em.hcl:3:11: Error EM007: Invalid attribute value: title must be a string.\n"
	if got != want {
		t.Fatalf("formatted diagnostics = %q, want %q", got, want)
	}
}

func TestRun_ValidatesShippedExample(t *testing.T) {
	paths := modelPaths(t, "examples")
	for _, path := range paths {
		t.Run(filepath.Base(path), func(t *testing.T) {
			// Given one shipped Event Modeling example.

			// When the validate command runs for that file.
			result := runCLI(t, "validate", path)

			// Then the example validates successfully.
			if result.exitCode != 0 {
				t.Fatalf("exit code = %d, stderr = %q", result.exitCode, result.stderr)
			}
		})
	}
}

func TestRun_ValidatesValidFixture(t *testing.T) {
	paths := modelPaths(t, "testdata", "valid")
	for _, path := range paths {
		t.Run(filepath.Base(path), func(t *testing.T) {
			// Given one valid structural fixture.

			// When the validate command runs for that file.
			result := runCLI(t, "validate", path)

			// Then the fixture validates successfully.
			if result.exitCode != 0 {
				t.Fatalf("exit code = %d, stderr = %q", result.exitCode, result.stderr)
			}
		})
	}
}

func TestRun_InvalidFixtureFailsValidation(t *testing.T) {
	paths := modelPaths(t, "testdata", "invalid")
	for _, path := range paths {
		t.Run(filepath.Base(path), func(t *testing.T) {
			// Given one intentionally invalid fixture.

			// When the validate command runs for that file.
			result := runCLI(t, "validate", path)

			// Then validation fails.
			if result.exitCode == 0 {
				t.Fatal("exit code = 0, want validation failure")
			}
		})
	}
}

func TestRun_CanonicalInvalidFixturesReportExpectedCodes(t *testing.T) {
	tests := []struct {
		path string
		code string
	}{
		{path: filepath.Join("..", "..", "testdata", "invalid", "reverse-flow.em.hcl"), code: "EM201"},
		{path: filepath.Join("..", "..", "testdata", "invalid", "state-view-when.em.hcl"), code: "EM301"},
	}

	for _, test := range tests {
		t.Run(filepath.Base(test.path), func(t *testing.T) {
			result := runCLI(t, "validate", test.path)
			if result.exitCode != 1 {
				t.Fatalf("exit code = %d, want 1; stderr = %q", result.exitCode, result.stderr)
			}
			if !strings.Contains(result.stderr, test.code) {
				t.Fatalf("stderr = %q, want %s", result.stderr, test.code)
			}
		})
	}
}

func TestRun_ValidateValidModelPrintsSuccess(t *testing.T) {
	// Given a valid Event Modeling model.
	modelPath := writeModel(t, "valid.em.hcl", `bounded_context "example" {
  title = "Example"
}

state_change "example" {
  title = "Example"
}
`)

	// When the validate command runs.
	result := runCLI(t, "validate", modelPath)

	// Then it returns success and reports the validated path.
	if result.exitCode != 0 {
		t.Fatalf("exit code = %d, stderr = %q", result.exitCode, result.stderr)
	}
	if got, want := result.stdout, modelPath+" valid\n"; got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}
	if result.stderr != "" {
		t.Fatalf("stderr = %q, want empty", result.stderr)
	}
}

func TestRun_ValidatePrintsWarningsWithoutFailing(t *testing.T) {
	modelPath := writeModel(t, "warning.em.hcl", `state_change "example" {
  title = "Example"
  command "submit" { title = "Submit" }
}`)

	result := runCLI(t, "validate", modelPath)

	if result.exitCode != 0 {
		t.Fatalf("exit code = %d, stderr = %q", result.exitCode, result.stderr)
	}
	if !strings.Contains(result.stderr, "Warning EM404: Every command has a reason") {
		t.Fatalf("stderr = %q, want command-reason warning", result.stderr)
	}
}

func TestRun_ValidateStrictProfileEscalatesCommandReason(t *testing.T) {
	modelPath := writeModel(t, "warning.em.hcl", `state_change "example" {
  command "submit" {}
}`)

	result := runCLI(t, "validate", "--profile", "strict", modelPath)

	if result.exitCode != 1 {
		t.Fatalf("exit code = %d, want 1; stderr = %q", result.exitCode, result.stderr)
	}
	if !strings.Contains(result.stderr, "Error EM404: Every command has a reason") {
		t.Fatalf("stderr = %q, want strict command-reason error", result.stderr)
	}
}

func TestRun_FormatWritesToStdoutOrInPlace(t *testing.T) {
	source := `state_change "example" {
  command "submit" {
    to = [event.example.submitted]
    title = "Submit"
  }
}`

	stdoutPath := writeModel(t, "stdout.em.hcl", source)
	stdoutResult := runCLI(t, "fmt", stdoutPath)
	if stdoutResult.exitCode != 0 || stdoutResult.stderr != "" {
		t.Fatalf("stdout format result = %#v", stdoutResult)
	}
	if !strings.Contains(stdoutResult.stdout, "title = \"Submit\"") {
		t.Fatalf("stdout = %q, want formatted source", stdoutResult.stdout)
	}

	writePath := writeModel(t, "write.em.hcl", source)
	writeResult := runCLI(t, "fmt", "-w", writePath)
	if writeResult.exitCode != 0 || writeResult.stdout != "" || writeResult.stderr != "" {
		t.Fatalf("write format result = %#v", writeResult)
	}
	written, err := os.ReadFile(writePath)
	if err != nil {
		t.Fatalf("read formatted model: %v", err)
	}
	if !strings.Contains(string(written), "title = \"Submit\"") {
		t.Fatalf("written = %q, want formatted source", written)
	}
}

func TestRun_DiagramWritesHTMLToStdoutOrFile(t *testing.T) {
	modelPath := writeModel(t, "valid.em.hcl", `state_change "example" {}`)

	stdoutResult := runCLI(t, "diagram", modelPath)
	if stdoutResult.exitCode != 0 || stdoutResult.stderr != "" {
		t.Fatalf("stdout diagram result = %#v", stdoutResult)
	}
	if !strings.Contains(stdoutResult.stdout, "<!doctype html>") || !strings.Contains(stdoutResult.stdout, "const MODEL =") {
		t.Fatalf("stdout does not contain rendered HTML")
	}

	outputPath := filepath.Join(t.TempDir(), "model.html")
	fileResult := runCLI(t, "diagram", "--output", outputPath, modelPath)
	if fileResult.exitCode != 0 || fileResult.stdout != "" {
		t.Fatalf("file diagram result = %#v", fileResult)
	}
	written, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("read diagram: %v", err)
	}
	if !strings.Contains(string(written), "<!doctype html>") {
		t.Fatal("output file does not contain rendered HTML")
	}
}

func TestRun_DiagramRejectsInvalidModelWithoutOverwritingOutput(t *testing.T) {
	modelPath := writeModel(t, "invalid.em.hcl", `state_change "example" {
  command "submit" { to = [event.example.missing] }
}`)
	outputPath := filepath.Join(t.TempDir(), "existing.html")
	const existing = "keep this"
	if err := os.WriteFile(outputPath, []byte(existing), 0o600); err != nil {
		t.Fatalf("write existing output: %v", err)
	}

	result := runCLI(t, "diagram", modelPath, "-o", outputPath)
	if result.exitCode != 1 || result.stdout != "" || !strings.Contains(result.stderr, "EM102") {
		t.Fatalf("diagram result = %#v", result)
	}
	written, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("read existing output: %v", err)
	}
	if got := string(written); got != existing {
		t.Fatalf("output = %q, want existing content", got)
	}
}

func TestRun_DiagramReportsWarningsAndStillRenders(t *testing.T) {
	modelPath := writeModel(t, "warning.em.hcl", `state_change "example" {
  command "submit" {}
}`)

	result := runCLI(t, "diagram", modelPath)
	if result.exitCode != 0 || !strings.Contains(result.stdout, "<!doctype html>") {
		t.Fatalf("diagram result = %#v", result)
	}
	if !strings.Contains(result.stderr, "Warning EM404") {
		t.Fatalf("stderr = %q, want warning", result.stderr)
	}
}

func TestRun_ValidateRejectsNonEventModelExtension(t *testing.T) {
	// Given a valid model whose file name does not end in .em.hcl.
	modelPath := writeModel(t, "valid.hcl", `bounded_context "example" {
  title = "Example"
}

state_change "example" {
  title = "Example"
}
`)

	// When the validate command runs.
	result := runCLI(t, "validate", modelPath)

	// Then it returns the usage error for an unsupported extension.
	if result.exitCode != 2 {
		t.Fatalf("exit code = %d, want 2; stderr = %q", result.exitCode, result.stderr)
	}
	if !strings.Contains(result.stderr, "must use the .em.hcl extension") {
		t.Fatalf("stderr = %q, want extension diagnostic", result.stderr)
	}
}

func TestRun_RejectsInvalidArguments(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{"missing subcommand", nil},
		{"missing model path", []string{"validate"}},
		{"multiple model paths", []string{"validate", "first.em.hcl", "second.em.hcl"}},
		{"unknown subcommand", []string{"format", "model.em.hcl"}},
		{"diagram missing model path", []string{"diagram"}},
		{"diagram missing output value", []string{"diagram", "model.em.hcl", "-o"}},
		{"diagram multiple models", []string{"diagram", "first.em.hcl", "second.em.hcl"}},
		{"serve missing port value", []string{"serve", "model.em.hcl", "--port"}},
		{"serve multiple models", []string{"serve", "first.em.hcl", "second.em.hcl"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Given an invalid command-line invocation.

			// When the command runs.
			result := runCLI(t, test.args...)

			// Then it returns the documented usage error.
			if result.exitCode != 2 {
				t.Fatalf("exit code = %d, want 2; stderr = %q", result.exitCode, result.stderr)
			}
			if got, want := result.stderr, usageMessage("emhcl")+"\n"; got != want {
				t.Fatalf("stderr = %q, want %q", got, want)
			}
		})
	}
}

func TestRun_ValidateSyntaxErrorIncludesSourceLocation(t *testing.T) {
	// Given a model with an unclosed block.
	modelPath := writeModel(t, "invalid.em.hcl", "bounded_context \"example\" {\n")

	// When the validate command runs.
	result := runCLI(t, "validate", modelPath)

	// Then it returns a source-located error without standard output.
	if result.exitCode != 1 {
		t.Fatalf("exit code = %d, want 1; stderr = %q", result.exitCode, result.stderr)
	}
	location := regexp.MustCompile(regexp.QuoteMeta(modelPath) + `:\d+:\d+: Error EM000:`)
	if !location.MatchString(result.stderr) {
		t.Fatalf("stderr = %q, want source-located error", result.stderr)
	}
	if result.stdout != "" {
		t.Fatalf("stdout = %q, want empty", result.stdout)
	}
}

func TestRun_VersionPrintsBuildVersion(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := Run("emhcl", "v0.6.0", []string{"version"}, &stdout, &stderr)

	if exitCode != 0 {
		t.Fatalf("exit code = %d, stderr = %q", exitCode, stderr.String())
	}
	if got, want := stdout.String(), "emhcl v0.6.0\n"; got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

type cliResult struct {
	exitCode int
	stdout   string
	stderr   string
}

func runCLI(t *testing.T, args ...string) cliResult {
	t.Helper()
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	return cliResult{
		exitCode: Run("emhcl", "dev", args, &stdout, &stderr),
		stdout:   stdout.String(),
		stderr:   stderr.String(),
	}
}

func writeModel(t *testing.T, name, model string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(model), 0o600); err != nil {
		t.Fatalf("write model: %v", err)
	}
	return path
}

func modelPaths(t *testing.T, directories ...string) []string {
	t.Helper()
	pattern := filepath.Join(append([]string{"..", ".."}, directories...)...)
	paths, err := filepath.Glob(filepath.Join(pattern, "*.em.hcl"))
	if err != nil {
		t.Fatalf("find models: %v", err)
	}
	if len(paths) == 0 {
		t.Fatalf("no Event Modeling files found at %s", pattern)
	}
	return paths
}
