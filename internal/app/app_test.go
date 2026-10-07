package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func readExample(t *testing.T, relPath string) (string, []byte) {
	t.Helper()
	path := filepath.Join("..", "..", relPath)
	source, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", relPath, err)
	}
	return path, source
}

func TestRender_ValidModelProducesHTMLWithNoErrorDiagnostics(t *testing.T) {
	path, source := readExample(t, "examples/minimal.em.hcl")

	result := Render(path, source, Valid)

	for _, diagnostic := range result.Diagnostics {
		if diagnostic.Severity == "Error" {
			t.Errorf("unexpected error diagnostic: %+v", diagnostic)
		}
	}
	if result.HTML == "" {
		t.Fatal("expected non-empty HTML for a valid model")
	}
	if !strings.Contains(result.HTML, "<!doctype html>") {
		t.Error("HTML does not look like a complete document")
	}
}

func TestRender_InvalidModelReturnsExpectedDiagnosticAndEmptyHTML(t *testing.T) {
	path, source := readExample(t, "testdata/invalid/dangling-reference.em.hcl")

	result := Render(path, source, Valid)

	if result.HTML != "" {
		t.Errorf("expected empty HTML for an invalid model, got %d bytes", len(result.HTML))
	}
	if len(result.Diagnostics) == 0 {
		t.Fatal("expected at least one diagnostic")
	}
	found := false
	for _, diagnostic := range result.Diagnostics {
		if diagnostic.Code == "EM102" && diagnostic.Severity == "Error" {
			found = true
			if diagnostic.Line == 0 {
				t.Errorf("expected a non-zero line for diagnostic %+v", diagnostic)
			}
			if diagnostic.Filename != path {
				t.Errorf("expected Filename %q, got %q", path, diagnostic.Filename)
			}
		}
	}
	if !found {
		t.Errorf("expected an EM102 error diagnostic, got %+v", result.Diagnostics)
	}
}

func TestFormat_RoundTripsAlreadyFormattedSource(t *testing.T) {
	path, source := readExample(t, "examples/minimal.em.hcl")

	result := Format(path, source)

	for _, diagnostic := range result.Diagnostics {
		if diagnostic.Severity == "Error" {
			t.Errorf("unexpected error diagnostic: %+v", diagnostic)
		}
	}
	if result.Source == "" {
		t.Fatal("expected non-empty formatted source")
	}
	// Formatting an already-canonical document is idempotent.
	second := Format(path, []byte(result.Source))
	if second.Source != result.Source {
		t.Error("expected formatting to be idempotent")
	}
}

func TestValidate_ProfileSwitchingChangesSeverity(t *testing.T) {
	path, source := readExample(t, "examples/complete.em.hcl")

	workshop := Validate(path, source, Workshop)
	valid := Validate(path, source, Valid)
	strict := Validate(path, source, Strict)

	severityFor := func(diagnostics []Diagnostic, code string) (string, bool) {
		for _, diagnostic := range diagnostics {
			if diagnostic.Code == code {
				return diagnostic.Severity, true
			}
		}
		return "", false
	}

	workshopSeverity, ok := severityFor(workshop, "EM406")
	if !ok || workshopSeverity != "Info" {
		t.Errorf("workshop: expected EM406 Info, got %q (found=%v)", workshopSeverity, ok)
	}
	validSeverity, ok := severityFor(valid, "EM406")
	if !ok || validSeverity != "Warning" {
		t.Errorf("valid: expected EM406 Warning, got %q (found=%v)", validSeverity, ok)
	}
	strictSeverity, ok := severityFor(strict, "EM406")
	if !ok || strictSeverity != "Error" {
		t.Errorf("strict: expected EM406 Error, got %q (found=%v)", strictSeverity, ok)
	}
}

func TestValidate_PopulatesFilenameForCLIFormatting(t *testing.T) {
	path, source := readExample(t, "testdata/invalid/dangling-reference.em.hcl")

	diagnostics := Validate(path, source, Valid)

	if len(diagnostics) == 0 {
		t.Fatal("expected at least one diagnostic")
	}
	for _, diagnostic := range diagnostics {
		if diagnostic.Filename != path {
			t.Errorf("expected Filename %q, got %q", path, diagnostic.Filename)
		}
	}
}

func TestValidatedModel_InvalidSourceDoesNotProduceAModel(t *testing.T) {
	path, source := readExample(t, "testdata/invalid/dangling-reference.em.hcl")

	built, diagnostics := ValidatedModel(path, source, Valid)

	if built != nil {
		t.Fatal("model is non-nil for invalid source")
	}
	if !diagnostics.HasErrors() {
		t.Fatal("diagnostics has no errors for invalid source")
	}
}

func TestValidatedModel_WarningOnlySourceProducesAModel(t *testing.T) {
	path, source := readExample(t, "examples/complete.em.hcl")

	built, diagnostics := ValidatedModel(path, source, Valid)

	if built == nil {
		t.Fatal("model is nil for warning-only source")
	}
	if diagnostics.HasErrors() {
		t.Fatalf("diagnostics = %#v, want warnings without errors", diagnostics)
	}
}

func TestValidatePath_UnreadablePathReturnsEM001(t *testing.T) {
	diagnostics := ValidatePath(filepath.Join(t.TempDir(), "missing.em.hcl"), Valid)

	assertReadFailure(t, diagnostics)
}

func TestRenderPath_UnreadablePathReturnsEM001(t *testing.T) {
	result := RenderPath(filepath.Join(t.TempDir(), "missing.em.hcl"), Valid)

	assertReadFailure(t, result.Diagnostics)
	if result.HTML != "" {
		t.Fatal("HTML is non-empty for unreadable input")
	}
}

func TestExportPath_UnreadablePathReturnsEM001(t *testing.T) {
	result := ExportPath(filepath.Join(t.TempDir(), "missing.em.hcl"))

	assertReadFailure(t, result.Diagnostics)
	if result.JSON != "" {
		t.Fatal("JSON is non-empty for unreadable input")
	}
}

func writeModelFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestReadModel_FileIsOneFileModel(t *testing.T) {
	path := filepath.Join(t.TempDir(), "model.em.hcl")
	writeModelFile(t, path, "team \"a\" {}\n")

	files, diagnostics := ReadModel(path)

	if len(diagnostics) != 0 {
		t.Fatalf("diagnostics = %#v, want none", diagnostics)
	}
	if len(files) != 1 || files[0].Name != path || string(files[0].Source) != "team \"a\" {}\n" {
		t.Fatalf("files = %#v, want one file named %q", files, path)
	}
}

func TestReadModel_FolderListsMemberFilesInByteOrder(t *testing.T) {
	dir := t.TempDir()
	writeModelFile(t, filepath.Join(dir, "b.em.hcl"), "team \"b\" {}\n")
	writeModelFile(t, filepath.Join(dir, "a.em.hcl"), "team \"a\" {}\n")
	writeModelFile(t, filepath.Join(dir, ".hidden.em.hcl"), "team \"hidden\" {}\n")
	writeModelFile(t, filepath.Join(dir, "notes.txt"), "notes\n")
	writeModelFile(t, filepath.Join(dir, "sub", "c.em.hcl"), "team \"c\" {}\n")
	if err := os.Mkdir(filepath.Join(dir, "dir.em.hcl"), 0o755); err != nil {
		t.Fatal(err)
	}

	files, diagnostics := ReadModel(dir)

	if len(diagnostics) != 0 {
		t.Fatalf("diagnostics = %#v, want none", diagnostics)
	}
	var names []string
	for _, file := range files {
		names = append(names, file.Name)
	}
	want := []string{filepath.Join(dir, "a.em.hcl"), filepath.Join(dir, "b.em.hcl")}
	if strings.Join(names, "\n") != strings.Join(want, "\n") {
		t.Fatalf("names = %v, want %v", names, want)
	}
	if string(files[0].Source) != "team \"a\" {}\n" || string(files[1].Source) != "team \"b\" {}\n" {
		t.Fatalf("sources = %q, %q", files[0].Source, files[1].Source)
	}
}

func TestReadModel_FolderFollowsSymlinkToFile(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(t.TempDir(), "target.em.hcl")
	writeModelFile(t, target, "team \"a\" {}\n")
	if err := os.Symlink(target, filepath.Join(dir, "link.em.hcl")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	files, diagnostics := ReadModel(dir)

	if len(diagnostics) != 0 {
		t.Fatalf("diagnostics = %#v, want none", diagnostics)
	}
	if len(files) != 1 || files[0].Name != filepath.Join(dir, "link.em.hcl") {
		t.Fatalf("files = %#v, want the symlinked member", files)
	}
}

func TestReadModel_BrokenMemberSymlinkReturnsEM001(t *testing.T) {
	dir := t.TempDir()
	writeModelFile(t, filepath.Join(dir, "a.em.hcl"), "team \"a\" {}\n")
	broken := filepath.Join(dir, "x.em.hcl")
	if err := os.Symlink(filepath.Join(dir, "missing-target"), broken); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	files, diagnostics := ReadModel(dir)

	if files != nil {
		t.Fatalf("files = %#v, want nil", files)
	}
	if len(diagnostics) != 1 {
		t.Fatalf("diagnostics = %#v, want exactly one", diagnostics)
	}
	diagnostic := diagnostics[0]
	if diagnostic.Code != "EM001" || diagnostic.Severity != "Error" || !strings.Contains(diagnostic.Detail, broken) {
		t.Fatalf("diagnostic = %#v, want EM001 naming %s", diagnostic, broken)
	}
}

func TestReadModel_MemberDirectoryIsSkipped(t *testing.T) {
	dir := t.TempDir()
	writeModelFile(t, filepath.Join(dir, "a.em.hcl"), "team \"a\" {}\n")
	if err := os.Mkdir(filepath.Join(dir, "d.em.hcl"), 0o755); err != nil {
		t.Fatal(err)
	}

	files, diagnostics := ReadModel(dir)

	if len(diagnostics) != 0 {
		t.Fatalf("diagnostics = %#v, want none", diagnostics)
	}
	if len(files) != 1 || files[0].Name != filepath.Join(dir, "a.em.hcl") {
		t.Fatalf("files = %#v, want only a.em.hcl", files)
	}
}

func TestReadModel_EmptyFolderReturnsEM001(t *testing.T) {
	dir := t.TempDir()
	writeModelFile(t, filepath.Join(dir, "notes.txt"), "notes\n")

	files, diagnostics := ReadModel(dir)

	if files != nil {
		t.Fatalf("files = %#v, want nil", files)
	}
	if len(diagnostics) != 1 {
		t.Fatalf("diagnostics = %#v, want exactly one", diagnostics)
	}
	diagnostic := diagnostics[0]
	wantDetail := "folder " + dir + " has no .em.hcl files"
	if diagnostic.Code != "EM001" || diagnostic.Severity != "Error" || diagnostic.Summary != "Failed to read model" || diagnostic.Detail != wantDetail {
		t.Fatalf("diagnostic = %#v, want EM001 %q", diagnostic, wantDetail)
	}
}

func TestValidatePath_FolderModelHasNoErrors(t *testing.T) {
	diagnostics := ValidatePath(filepath.Join("..", "..", "examples", "pet-clinic"), Valid)

	if diagnostics.HasErrors() {
		t.Fatalf("diagnostics = %#v, want no errors", diagnostics)
	}
}

func TestRenderPath_FolderModelProducesHTML(t *testing.T) {
	result := RenderPath(filepath.Join("..", "..", "examples", "pet-clinic"), Valid)

	if result.Diagnostics.HasErrors() {
		t.Fatalf("diagnostics = %#v, want no errors", result.Diagnostics)
	}
	if result.HTML == "" {
		t.Fatal("HTML is empty for a valid folder model")
	}
}

func TestValidatePath_CrossFileDuplicateIDNamesSecondFile(t *testing.T) {
	dir := t.TempDir()
	writeModelFile(t, filepath.Join(dir, "a.em.hcl"), "team \"clinic_team\" {\n  title = \"A\"\n}\n")
	writeModelFile(t, filepath.Join(dir, "b.em.hcl"), "team \"clinic_team\" {\n  title = \"B\"\n}\n")

	diagnostics := ValidatePath(dir, Valid)

	second := filepath.Join(dir, "b.em.hcl")
	for _, diagnostic := range diagnostics {
		if diagnostic.Code == "EM002" && diagnostic.Severity == "Error" && diagnostic.Filename == second {
			return
		}
	}
	t.Fatalf("diagnostics = %#v, want an EM002 error in %s", diagnostics, second)
}

func TestFormatFile_UnreadablePathReturnsEM001(t *testing.T) {
	result := FormatFile(filepath.Join(t.TempDir(), "missing.em.hcl"))

	assertReadFailure(t, result.Diagnostics)
	if result.Source != "" {
		t.Fatal("formatted source is non-empty for unreadable input")
	}
}

func assertReadFailure(t *testing.T, diagnostics []Diagnostic) {
	t.Helper()
	if len(diagnostics) != 1 {
		t.Fatalf("diagnostics = %#v, want exactly one", diagnostics)
	}
	diagnostic := diagnostics[0]
	if diagnostic.Code != "EM001" || diagnostic.Severity != "Error" || diagnostic.Summary != "Failed to read file" {
		t.Fatalf("diagnostic = %#v, want EM001 read error", diagnostic)
	}
}

func TestParseProfile_RecognizesKnownNames(t *testing.T) {
	cases := map[string]Profile{"workshop": Workshop, "valid": Valid, "strict": Strict}
	for name, expected := range cases {
		profile, ok := ParseProfile(name)
		if !ok || profile != expected {
			t.Errorf("ParseProfile(%q) = %v, %v; want %v, true", name, profile, ok, expected)
		}
	}
	if _, ok := ParseProfile("bogus"); ok {
		t.Error("expected ParseProfile(\"bogus\") to report false")
	}
}
