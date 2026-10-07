package app

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRender_RendersEveryShippedExample(t *testing.T) {
	paths, err := filepath.Glob(filepath.Join("..", "..", "examples", "*.em.hcl"))
	if err != nil {
		t.Fatalf("find examples: %v", err)
	}
	if len(paths) == 0 {
		t.Fatal("no example fixtures found")
	}
	for _, path := range paths {
		t.Run(filepath.Base(path), func(t *testing.T) {
			source, readErr := os.ReadFile(path)
			if readErr != nil {
				t.Fatalf("read example: %v", readErr)
			}
			result := Render(path, source, Valid)
			if result.Diagnostics.HasErrors() {
				t.Fatalf("diagnostics = %#v, want no errors", result.Diagnostics)
			}
			if result.HTML == "" {
				t.Fatal("HTML is empty for valid example")
			}
		})
	}
}
func TestRender_RendersEveryShippedExampleFolder(t *testing.T) {
	entries, err := os.ReadDir(filepath.Join("..", "..", "examples"))
	if err != nil {
		t.Fatalf("read examples dir: %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("no examples found")
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		path := filepath.Join("..", "..", "examples", entry.Name())
		t.Run(entry.Name(), func(t *testing.T) {
			result := RenderPath(path, Valid)
			if result.Diagnostics.HasErrors() {
				t.Fatalf("diagnostics = %#v, want no errors", result.Diagnostics)
			}
			if result.HTML == "" {
				t.Fatal("HTML is empty for valid folder")
			}
		})
	}
}

func TestRender_RefusesEveryInvalidFixture(t *testing.T) {
	paths, err := filepath.Glob(filepath.Join("..", "..", "testdata", "invalid", "*.em.hcl"))
	if err != nil {
		t.Fatalf("find invalid fixtures: %v", err)
	}
	if len(paths) == 0 {
		t.Fatal("no invalid fixtures found")
	}
	for _, path := range paths {
		t.Run(filepath.Base(path), func(t *testing.T) {
			source, readErr := os.ReadFile(path)
			if readErr != nil {
				t.Fatalf("read fixture: %v", readErr)
			}
			result := Render(path, source, Valid)
			if !result.Diagnostics.HasErrors() {
				t.Fatalf("diagnostics = %#v, want an error", result.Diagnostics)
			}
			if result.HTML != "" {
				t.Fatal("HTML is non-empty for invalid fixture")
			}
		})
	}
}

func TestValidate_InvalidFoldersProduceExpectedErrors(t *testing.T) {
	testCases := []struct {
		name         string
		expectedCode string
	}{
		{"duplicate-workflow", "EM002"},
		{"chapters-in-two-files", "EM013"},
		{"workflow-in-two-chapters", "EM014"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join("..", "..", "testdata", "invalid-folders", tc.name)
			result := ValidatePath(path, Valid)
			if !result.HasErrors() {
				t.Fatalf("ValidatePath should error for %s, got no errors", tc.name)
			}
			foundCode := false
			for _, diag := range result {
				if diag.Code == tc.expectedCode {
					foundCode = true
					break
				}
			}
			if !foundCode {
				t.Fatalf("expected code %s not found in diagnostics: %#v", tc.expectedCode, result)
			}
		})
	}
}

func TestValidate_UnchapteredFolderWarnsUnderValid(t *testing.T) {
	path := filepath.Join("..", "..", "testdata", "valid-folders", "unchaptered")
	result := ValidatePath(path, Valid)
	if result.HasErrors() {
		t.Fatalf("ValidatePath should not error under Valid profile, got: %#v", result)
	}
	foundEM407 := false
	for _, diag := range result {
		if diag.Code == "EM407" {
			foundEM407 = true
			break
		}
	}
	if !foundEM407 {
		t.Fatalf("expected EM407 warning not found in diagnostics: %#v", result)
	}
}

func TestValidate_UnchapteredFolderErrorsUnderStrict(t *testing.T) {
	path := filepath.Join("..", "..", "testdata", "valid-folders", "unchaptered")
	result := ValidatePath(path, Strict)
	if !result.HasErrors() {
		t.Fatalf("ValidatePath should error under Strict profile, got no errors")
	}
	foundEM407 := false
	for _, diag := range result {
		if diag.Code == "EM407" {
			foundEM407 = true
			break
		}
	}
	if !foundEM407 {
		t.Fatalf("expected EM407 error not found in diagnostics: %#v", result)
	}
}
