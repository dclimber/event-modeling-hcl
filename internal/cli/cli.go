// Package cli implements the emhcl command-line adapter.
package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/event-modeling-hcl/eventmodeling-hcl/internal/app"
	"github.com/event-modeling-hcl/eventmodeling-hcl/internal/serve"
)

func usageMessage(name string) string {
	return "usage: " + name + " <validate [--profile workshop|valid|strict] | fmt [-w] | diagram | serve> <model.em.hcl> [diagram: -o <file>] [serve: --addr <host> --port <n> --profile <p>]"
}

// defaultServeAddr and defaultServePort are serve's defaults when a
// command line omits --addr / --port: loopback only, so a model isn't
// reachable from the rest of the network unless --addr is passed
// explicitly, on a port unlikely to already be in use by another local dev
// server. Pass --addr 0.0.0.0 (or a specific interface) to share it.
const (
	defaultServeAddr = "127.0.0.1"
	defaultServePort = 8080
)

// commandKind names which subcommand a parsed command line requested.
type commandKind uint8

const (
	validateCommand commandKind = iota
	formatCommand
	diagramCommand
	serveCommand
	versionCommand
)

// cliCommand is one fully parsed command-line invocation, ready to be
// executed by Run.
type cliCommand struct {
	kind    commandKind
	path    string
	profile app.Profile
	write   bool
	output  string
	addr    string
	port    int
}

// Run executes one command-line invocation and returns the process exit code:
// 0 for success, 1 when validation finds problems in the model, and 2 when
// the command line itself could not be understood.
func Run(name, toolVersion string, args []string, stdout, stderr io.Writer) int {
	command, err := parseCommand(name, args)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}

	if command.kind == versionCommand {
		fmt.Fprintf(stdout, "%s %s\n", name, toolVersion)
		return 0
	}
	if command.kind == formatCommand {
		return formatFile(command, stdout, stderr)
	}
	if command.kind == diagramCommand {
		return diagramFile(command, stdout, stderr)
	}
	if command.kind == serveCommand {
		return serveFile(command, stderr)
	}
	diagnostics := app.ValidateFile(command.path, command.profile)
	if len(diagnostics) > 0 {
		writeDiagnostics(stderr, diagnostics)
	}
	if diagnostics.HasErrors() {
		return 1
	}

	fmt.Fprintf(stdout, "%s valid\n", command.path)
	return 0
}

// parseCommand turns raw command-line arguments into a cliCommand, or reports
// an error describing why the arguments are not a valid invocation.
func parseCommand(name string, args []string) (cliCommand, error) {
	usage := usageMessage(name)
	if len(args) == 1 && args[0] == "version" {
		return cliCommand{kind: versionCommand}, nil
	}
	if len(args) == 2 && args[0] == "validate" {
		return validateCLICommand(args[1], app.Valid)
	}
	if len(args) == 4 && args[0] == "validate" && args[1] == "--profile" {
		profile, ok := app.ParseProfile(args[2])
		if !ok {
			return cliCommand{}, errors.New("profile must be workshop, valid, or strict")
		}
		return validateCLICommand(args[3], profile)
	}
	if len(args) == 2 && args[0] == "fmt" {
		return formatCLICommand(args[1], false)
	}
	if len(args) == 3 && args[0] == "fmt" && args[1] == "-w" {
		return formatCLICommand(args[2], true)
	}
	if len(args) >= 1 && args[0] == "diagram" {
		return diagramCLICommand(args[1:], usage)
	}
	if len(args) >= 1 && args[0] == "serve" {
		return serveCLICommand(args[1:], usage)
	}
	return cliCommand{}, errors.New(usage)
}

func validateCLICommand(path string, profile app.Profile) (cliCommand, error) {
	if !strings.HasSuffix(path, ".em.hcl") {
		return cliCommand{}, errors.New("model file must use the .em.hcl extension")
	}
	return cliCommand{kind: validateCommand, path: path, profile: profile}, nil
}

func formatCLICommand(path string, write bool) (cliCommand, error) {
	if !strings.HasSuffix(path, ".em.hcl") {
		return cliCommand{}, errors.New("model file must use the .em.hcl extension")
	}
	return cliCommand{kind: formatCommand, path: path, write: write}, nil
}

func diagramCLICommand(args []string, usage string) (cliCommand, error) {
	var path string
	var output string
	for index := 0; index < len(args); index++ {
		switch args[index] {
		case "-o", "--output":
			if output != "" || index+1 >= len(args) {
				return cliCommand{}, errors.New(usage)
			}
			output = args[index+1]
			index++
		default:
			if path != "" {
				return cliCommand{}, errors.New(usage)
			}
			path = args[index]
		}
	}
	if path == "" {
		return cliCommand{}, errors.New(usage)
	}
	if !strings.HasSuffix(path, ".em.hcl") {
		return cliCommand{}, errors.New("model file must use the .em.hcl extension")
	}
	return cliCommand{kind: diagramCommand, path: path, output: output}, nil
}

// serveCLICommand parses the arguments following "serve": exactly one
// model path, plus any of --addr, --port, and --profile in any order.
func serveCLICommand(args []string, usage string) (cliCommand, error) {
	path := ""
	addr := defaultServeAddr
	port := defaultServePort
	profile := app.Valid

	for index := 0; index < len(args); index++ {
		switch args[index] {
		case "--addr":
			if index+1 >= len(args) {
				return cliCommand{}, errors.New(usage)
			}
			addr = args[index+1]
			index++
		case "--port":
			if index+1 >= len(args) {
				return cliCommand{}, errors.New(usage)
			}
			parsedPort, err := strconv.Atoi(args[index+1])
			if err != nil {
				return cliCommand{}, errors.New("port must be a number")
			}
			if parsedPort < 0 || parsedPort > 65535 {
				return cliCommand{}, errors.New("port must be between 0 and 65535")
			}
			port = parsedPort
			index++
		case "--profile":
			if index+1 >= len(args) {
				return cliCommand{}, errors.New(usage)
			}
			parsedProfile, ok := app.ParseProfile(args[index+1])
			if !ok {
				return cliCommand{}, errors.New("profile must be workshop, valid, or strict")
			}
			profile = parsedProfile
			index++
		default:
			if path != "" {
				return cliCommand{}, errors.New(usage)
			}
			path = args[index]
		}
	}
	if path == "" {
		return cliCommand{}, errors.New(usage)
	}
	if !strings.HasSuffix(path, ".em.hcl") {
		return cliCommand{}, errors.New("model file must use the .em.hcl extension")
	}
	return cliCommand{kind: serveCommand, path: path, profile: profile, addr: addr, port: port}, nil
}

func formatFile(command cliCommand, stdout, stderr io.Writer) int {
	result := app.FormatFile(command.path)
	if len(result.Diagnostics) > 0 {
		writeDiagnostics(stderr, result.Diagnostics)
	}
	if result.Diagnostics.HasErrors() {
		return 1
	}
	formatted := []byte(result.Source)
	if !command.write {
		_, _ = stdout.Write(formatted)
		return 0
	}
	info, err := os.Stat(command.path)
	if err != nil {
		fmt.Fprintf(stderr, "failed to stat %s: %v\n", command.path, err)
		return 1
	}
	if err := os.WriteFile(command.path, formatted, info.Mode().Perm()); err != nil {
		fmt.Fprintf(stderr, "failed to write %s: %v\n", command.path, err)
		return 1
	}
	return 0
}

// serveFile runs a local live-reload server for command.path until it is
// interrupted (Ctrl-C) or fails to start. All behavior — rendering,
// file-watching, HTTP, browser-opening — is internal/serve's; this is
// deliberately thin, blocking glue with nothing of its own worth unit
// testing (internal/serve's own test suite already covers every part of it
// that is testable without actually binding a live socket).
func serveFile(command cliCommand, stderr io.Writer) int {
	if err := serve.Start(command.path, command.addr, command.port, command.profile); err != nil {
		fmt.Fprintf(stderr, "failed to serve %s: %v\n", command.path, err)
		return 1
	}
	return 0
}

func diagramFile(command cliCommand, stdout, stderr io.Writer) int {
	result := app.RenderFile(command.path, app.Valid)
	if len(result.Diagnostics) > 0 {
		writeDiagnostics(stderr, result.Diagnostics)
	}
	if result.Diagnostics.HasErrors() {
		return 1
	}
	if command.output == "" {
		_, _ = io.WriteString(stdout, result.HTML)
		return 0
	}
	if err := os.WriteFile(command.output, []byte(result.HTML), 0o644); err != nil {
		fmt.Fprintf(stderr, "failed to write %s: %v\n", command.output, err)
		return 1
	}
	fmt.Fprintf(stderr, "wrote %s\n", command.output)
	return 0
}

// writeDiagnostics renders diagnostics in the CLI's text format and writes
// the result to w.
func writeDiagnostics(w io.Writer, diagnostics []app.Diagnostic) {
	_, _ = io.WriteString(w, formatDiagnostics(diagnostics))
}

// formatDiagnostics renders every diagnostic with formatDiagnostic and
// concatenates the results in order.
func formatDiagnostics(diagnostics []app.Diagnostic) string {
	return strings.Join(mapSlice(diagnostics, formatDiagnostic), "")
}

// mapSlice applies transform to each element of items, in order, and
// returns the resulting slice.
func mapSlice[T, U any](items []T, transform func(T) U) []U {
	result := make([]U, len(items))
	for index, item := range items {
		result[index] = transform(item)
	}
	return result
}

// formatDiagnostic renders one diagnostic as a single line of text, in the
// form "file:line:column: Severity EMxxx: Summary: Detail\n". Diagnostics
// that have no source location (for example, a failure to even read the
// model file, which happens before any source text exists to point at) omit
// the "file:line:column:" prefix — recognized here by an empty Filename,
// which app.Diagnostic only ever leaves unset when the underlying
// hcl.Diagnostic had no Subject.
func formatDiagnostic(diagnostic app.Diagnostic) string {
	message := fmt.Sprintf("%s %s: %s", diagnostic.Severity, diagnostic.Code, diagnostic.Summary)
	if diagnostic.Detail != "" {
		message += ": " + diagnostic.Detail
	}
	if diagnostic.Filename == "" {
		return fmt.Sprintf("%s\n", message)
	}
	return fmt.Sprintf("%s:%d:%d: %s\n", diagnostic.Filename, diagnostic.Line, diagnostic.Column, message)
}
