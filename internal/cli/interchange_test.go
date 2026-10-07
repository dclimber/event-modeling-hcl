package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/event-modeling-hcl/eventmodeling-hcl/internal/interchange"
)

func TestRun_ImportExportRealNewsPreservesPrototypeData(t *testing.T) {
	input := filepath.Join("..", "..", "testdata", "valid", "axoniq-vsa-sample-news.json")
	modelPath := filepath.Join(t.TempDir(), "news.em.hcl")
	imported := runCLI(t, "import", input, "-o", modelPath)
	if imported.exitCode != 0 || imported.stdout != "" {
		t.Fatalf("import result = %+v", imported)
	}
	exportPath := filepath.Join(t.TempDir(), "news.json")
	exported := runCLI(t, "export", modelPath, "-o", exportPath)
	if exported.exitCode != 0 || exported.stdout != "" {
		t.Fatalf("export result = %+v", exported)
	}
	readDocument := func(path string) interchange.Document {
		t.Helper()
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		var document interchange.Document
		if err := json.Unmarshal(data, &document); err != nil {
			t.Fatal(err)
		}
		return document
	}
	prototypes := func(document interchange.Document) map[string]any {
		t.Helper()
		result := make(map[string]any)
		for _, slice := range document.Slices {
			for _, screen := range slice.Screens {
				if len(screen.Prototype) == 0 {
					continue
				}
				var prototype any
				if err := json.Unmarshal(screen.Prototype, &prototype); err != nil {
					t.Fatal(err)
				}
				result[slice.Title+"/"+screen.Title] = prototype
			}
		}
		return result
	}
	want := prototypes(readDocument(input))
	got := prototypes(readDocument(exportPath))
	if len(want) == 0 {
		t.Fatal("fixture has no prototypes")
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("prototype data changed: got %#v, want %#v", got, want)
	}
}

func TestRun_ImportRejectsInvalidJSONWithoutOverwritingOutput(t *testing.T) {
	for name, inputJSON := range map[string]string{
		"trailing document": `{"slices":[]} {"slices":[]}`,
		"unknown property":  `{"slices":[],"unsupported":true}`,
	} {
		t.Run(name, func(t *testing.T) {
			input := filepath.Join(t.TempDir(), "invalid.json")
			if err := os.WriteFile(input, []byte(inputJSON), 0o600); err != nil {
				t.Fatal(err)
			}
			output := filepath.Join(t.TempDir(), "existing.em.hcl")
			const original = "preserve existing output"
			if err := os.WriteFile(output, []byte(original), 0o600); err != nil {
				t.Fatal(err)
			}
			result := runCLI(t, "import", input, "-o", output)
			if result.exitCode != 1 || result.stdout != "" || !strings.Contains(result.stderr, "Failed to import JSON") {
				t.Fatalf("import result = %+v", result)
			}
			data, err := os.ReadFile(output)
			if err != nil {
				t.Fatal(err)
			}
			if string(data) != original {
				t.Fatalf("existing output overwritten: %q", data)
			}
		})
	}
}

func TestRun_ExportRejectsSourceOverwrite(t *testing.T) {
	for _, name := range []string{"same path", "cleaned same path", "other HCL output", "symlink", "hardlink"} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			input := filepath.Join(dir, "model.em.hcl")
			const original = "bounded_context \"source\" {}\n"
			if err := os.WriteFile(input, []byte(original), 0o600); err != nil {
				t.Fatal(err)
			}
			output := input
			if name == "cleaned same path" {
				output = dir + "/./model.em.hcl"
			}
			if name == "other HCL output" {
				output = filepath.Join(dir, "other.em.hcl")
				if err := os.WriteFile(output, []byte(original), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			if name == "symlink" || name == "hardlink" {
				output = filepath.Join(dir, "alias.json")
				link := os.Symlink
				if name == "hardlink" {
					link = os.Link
				}
				if err := link(input, output); err != nil {
					t.Fatal(err)
				}
			}
			result := runCLI(t, "export", input, "-o", output)
			if result.exitCode != 2 || result.stdout != "" || !strings.Contains(result.stderr, "export") || !strings.Contains(result.stderr, "-o") {
				t.Fatalf("expected actionable usage error, got %+v", result)
			}
			for _, path := range []string{input, output} {
				data, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				if string(data) != original {
					t.Fatalf("%s overwritten: %q", path, data)
				}
			}
		})
	}
}

func TestRun_ExportRejectsFolderMemberOverwrite(t *testing.T) {
	for _, name := range []string{"output symlink to member", "output hardlink to member", "member symlink to output"} {
		t.Run(name, func(t *testing.T) {
			folder := filepath.Join(t.TempDir(), "model")
			if err := os.Mkdir(folder, 0o700); err != nil {
				t.Fatal(err)
			}
			const original = "bounded_context \"source\" {}\n"
			member := filepath.Join(folder, "x.em.hcl")
			if err := os.WriteFile(member, []byte(original), 0o600); err != nil {
				t.Fatal(err)
			}
			output := filepath.Join(t.TempDir(), "alias.json")
			switch name {
			case "output symlink to member":
				if err := os.Symlink(member, output); err != nil {
					t.Fatal(err)
				}
			case "output hardlink to member":
				if err := os.Link(member, output); err != nil {
					t.Fatal(err)
				}
			case "member symlink to output":
				if err := os.WriteFile(output, []byte(original), 0o600); err != nil {
					t.Fatal(err)
				}
				member = filepath.Join(folder, "link.em.hcl")
				if err := os.Symlink(output, member); err != nil {
					t.Fatal(err)
				}
				if err := os.Remove(filepath.Join(folder, "x.em.hcl")); err != nil {
					t.Fatal(err)
				}
			}
			result := runCLI(t, "export", folder, "-o", output)
			if result.exitCode != 2 || result.stdout != "" || !strings.Contains(result.stderr, "export output must not overwrite the source") {
				t.Fatalf("expected refusal, got %+v", result)
			}
			for _, path := range []string{member, output} {
				data, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				if string(data) != original {
					t.Fatalf("%s overwritten: %q", path, data)
				}
			}
		})
	}
}

func TestRun_ImportEmptySlicesSucceeds(t *testing.T) {
	input := filepath.Join(t.TempDir(), "empty.json")
	if err := os.WriteFile(input, []byte(`{"slices":[]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, output := range []string{"", filepath.Join(t.TempDir(), "empty.em.hcl")} {
		args := []string{"import", input}
		if output != "" {
			args = append(args, "-o", output)
		}
		result := runCLI(t, args...)
		if result.exitCode != 0 || result.stdout != "" {
			t.Fatalf("empty import result = %+v", result)
		}
		if output != "" {
			data, err := os.ReadFile(output)
			if err != nil || len(data) != 0 {
				t.Fatalf("empty output = %q, error = %v", data, err)
			}
		}
	}
}

func TestRun_ImportStdoutDiagnosticNamesStdout(t *testing.T) {
	input := filepath.Join(t.TempDir(), "invalid-model.json")
	const document = `{"slices":[{"id":"slice","title":"Slice","context":"Context","sliceType":"TRANSLATION","commands":[{"id":"command","title":"Command","type":"COMMAND","fields":[],"dependencies":[]}],"events":[],"readmodels":[],"screens":[],"processors":[],"tables":[],"specifications":[]}]}`
	if err := os.WriteFile(input, []byte(document), 0o600); err != nil {
		t.Fatal(err)
	}
	result := runCLI(t, "import", input)
	if !strings.Contains(result.stderr, "<stdout>:") || strings.Contains(result.stderr, "invalid-model.em.hcl") {
		t.Fatalf("stdout diagnostics = %+v", result)
	}
}

func TestRun_UsageDescribesImportExport(t *testing.T) {
	result := runCLI(t)
	if result.exitCode != 2 || result.stdout != "" {
		t.Fatalf("usage result = %+v", result)
	}
	for _, term := range []string{"usage:", "import", "export", ".json", ".em.hcl", "-o"} {
		if !strings.Contains(result.stderr, term) {
			t.Errorf("usage omits %q: %s", term, result.stderr)
		}
	}
}
