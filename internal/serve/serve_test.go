package serve

import (
	"context"
	"io"
	"net"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/event-modeling-hcl/eventmodeling-hcl/internal/app"
	"github.com/event-modeling-hcl/eventmodeling-hcl/internal/syntax"
)

// discardEnv is an environment that reads real files but throws away all
// console output and never touches the network/browser/signals — for tests
// that only care about regenerate/watch's file-driven behavior.
func discardEnv() environment {
	return environment{
		readModel: app.ReadModel,
		stdout:    io.Discard,
		stderr:    io.Discard,
	}
}

func TestServingURLUsesTheActualBoundPort(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer listener.Close()

	got := servingURL(listener, "127.0.0.1")
	if strings.HasSuffix(got, ":0") {
		t.Fatalf("serving URL retained requested port zero: %q", got)
	}
	if !strings.HasPrefix(got, "http://127.0.0.1:") {
		t.Fatalf("serving URL = %q", got)
	}
}

func validResult(t *testing.T) app.RenderResult {
	t.Helper()
	path := filepath.Join("..", "..", "examples", "minimal.em.hcl")
	source, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read example: %v", err)
	}
	return app.Render(path, source, app.Valid)
}

func invalidResult(t *testing.T) app.RenderResult {
	t.Helper()
	path := filepath.Join("..", "..", "testdata", "invalid", "reverse-flow.em.hcl")
	source, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	return app.Render(path, source, app.Valid)
}

// Behavior 1: the served hash changes iff the servable content changes.

func TestState_HashIsStableForTheSameResult(t *testing.T) {
	s := &state{}
	result := validResult(t)

	s.update(result)
	_, hash1 := s.snapshot()
	s.update(result)
	_, hash2 := s.snapshot()

	if hash1 != hash2 {
		t.Errorf("hash changed for an identical result: %q -> %q", hash1, hash2)
	}
}

func TestState_HashChangesWhenHTMLChanges(t *testing.T) {
	s := &state{}

	s.update(validResult(t))
	_, hash1 := s.snapshot()
	s.update(app.RenderResult{HTML: "<html>different</html>"})
	_, hash2 := s.snapshot()

	if hash1 == hash2 {
		t.Error("hash did not change when HTML changed")
	}
}

func TestState_HashChangesWhenDiagnosticsChangeEvenWithEmptyHTML(t *testing.T) {
	// Two different error states both render no HTML at all — hashing only
	// the HTML would make them indistinguishable, so the served page would
	// never reload from one error to a different one.
	s := &state{}

	s.update(invalidResult(t))
	_, hash1 := s.snapshot()
	s.update(app.RenderResult{Diagnostics: []app.Diagnostic{
		{Code: "EM999", Severity: "Error", Summary: "a different problem"},
	}})
	_, hash2 := s.snapshot()

	if hash1 == hash2 {
		t.Error("hash did not change between two different error results")
	}
}

// Behaviors 2, 3, 4, 5: the real production mux, exercised end to end.

func TestMux_RootServesShellPageWithIframeAndPollScript(t *testing.T) {
	s := &state{}
	s.update(validResult(t))

	rec := httptest.NewRecorder()
	newMux(s).ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))

	if rec.Code != 200 {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `<iframe src="/diagram">`) {
		t.Error("shell page missing the diagram iframe")
	}
	if !strings.Contains(body, `fetch("/hash")`) {
		t.Error("shell page missing the poll-reload script")
	}
}

func TestMux_DiagramServesRenderedHTMLVerbatimForAValidModel(t *testing.T) {
	s := &state{}
	result := validResult(t)
	s.update(result)

	rec := httptest.NewRecorder()
	newMux(s).ServeHTTP(rec, httptest.NewRequest("GET", "/diagram", nil))

	if rec.Code != 200 {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if rec.Body.String() != result.HTML {
		t.Error("/diagram body does not match the rendered HTML exactly")
	}
	if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "text/html") {
		t.Errorf("expected text/html content type, got %q", ct)
	}
}

func TestMux_DiagramServesReadableDiagnosticsPageForAnInvalidModel(t *testing.T) {
	s := &state{}
	result := invalidResult(t)
	if result.HTML != "" {
		t.Fatal("test fixture unexpectedly produced HTML")
	}
	s.update(result)

	rec := httptest.NewRecorder()
	newMux(s).ServeHTTP(rec, httptest.NewRequest("GET", "/diagram", nil))

	if rec.Code != 200 {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	body := rec.Body.String()
	if body == "" {
		t.Fatal("expected a non-blank diagnostics page")
	}
	for _, diagnostic := range result.Diagnostics {
		if !strings.Contains(body, diagnostic.Code) {
			t.Errorf("diagnostics page missing code %q", diagnostic.Code)
		}
		if !strings.Contains(body, diagnostic.Summary) {
			t.Errorf("diagnostics page missing summary %q", diagnostic.Summary)
		}
	}
}

func TestMux_HashReturnsTheCurrentHash(t *testing.T) {
	s := &state{}
	s.update(validResult(t))
	_, want := s.snapshot()

	rec := httptest.NewRecorder()
	newMux(s).ServeHTTP(rec, httptest.NewRequest("GET", "/hash", nil))

	if rec.Code != 200 {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if got := rec.Body.String(); got != want {
		t.Errorf("/hash returned %q, want %q", got, want)
	}
}

// Behavior 6: after the watched file changes on disk, the served state
// updates to match.

func TestWatch_RegeneratesWhenTheWatchedFileChanges(t *testing.T) {
	originalPollInterval := pollInterval
	pollInterval = 20 * time.Millisecond
	defer func() { pollInterval = originalPollInterval }()

	dir := t.TempDir()
	path := filepath.Join(dir, "model.em.hcl")
	initial, err := os.ReadFile(filepath.Join("..", "..", "examples", "minimal.em.hcl"))
	if err != nil {
		t.Fatalf("read seed example: %v", err)
	}
	if err := os.WriteFile(path, initial, 0o644); err != nil {
		t.Fatalf("write seed file: %v", err)
	}

	s := &state{}
	if err := regenerate(discardEnv(), s, path, app.Valid); err != nil {
		t.Fatalf("initial regenerate: %v", err)
	}
	_, hashBefore := s.snapshot()

	startHash, err := fileSourceHash(discardEnv(), path)
	if err != nil {
		t.Fatalf("hash seed file: %v", err)
	}

	invalid, err := os.ReadFile(filepath.Join("..", "..", "testdata", "invalid", "reverse-flow.em.hcl"))
	if err != nil {
		t.Fatalf("read invalid fixture: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go watch(ctx, discardEnv(), s, path, app.Valid, startHash)
	if err := os.WriteFile(path, invalid, 0o644); err != nil {
		t.Fatalf("rewrite file: %v", err)
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		result, hashAfter := s.snapshot()
		if hashAfter != hashBefore {
			if result.HTML != "" {
				t.Fatal("expected the rewritten (invalid) file to clear the HTML")
			}
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("watch did not regenerate after the watched file changed")
}

func TestWatch_RegeneratesWhenContentChangesWithoutMTimeAdvancing(t *testing.T) {
	originalPollInterval := pollInterval
	pollInterval = 20 * time.Millisecond
	defer func() { pollInterval = originalPollInterval }()

	dir := t.TempDir()
	path := filepath.Join(dir, "model.em.hcl")
	initial, err := os.ReadFile(filepath.Join("..", "..", "examples", "minimal.em.hcl"))
	if err != nil {
		t.Fatalf("read seed example: %v", err)
	}
	if err := os.WriteFile(path, initial, 0o644); err != nil {
		t.Fatalf("write seed: %v", err)
	}

	s := &state{}
	if err := regenerate(discardEnv(), s, path, app.Valid); err != nil {
		t.Fatalf("initial regenerate: %v", err)
	}
	_, before := s.snapshot()
	startHash, err := fileSourceHash(discardEnv(), path)
	if err != nil {
		t.Fatalf("hash seed: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat seed: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go watch(ctx, discardEnv(), s, path, app.Valid, startHash)

	invalid, err := os.ReadFile(filepath.Join("..", "..", "testdata", "invalid", "reverse-flow.em.hcl"))
	if err != nil {
		t.Fatalf("read invalid fixture: %v", err)
	}
	if err := os.WriteFile(path, invalid, 0o644); err != nil {
		t.Fatalf("rewrite file: %v", err)
	}
	if err := os.Chtimes(path, info.ModTime(), info.ModTime()); err != nil {
		t.Fatalf("restore mtime: %v", err)
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		result, after := s.snapshot()
		if after != before {
			if result.HTML != "" {
				t.Fatal("expected invalid replacement to clear HTML")
			}
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("watch did not regenerate after a same-mtime content change")
}

// Folder models: the watch contract covers the set of member files, not
// just one file's content.

func TestSourceHash_SeparatesFileNameFromContentAndFileFromFile(t *testing.T) {
	pairs := [][]syntax.File{
		{{Name: "ab", Source: []byte("c")}},
		{{Name: "a", Source: []byte("bc")}},
		{{Name: "a", Source: []byte("b")}, {Name: "c", Source: []byte("d")}},
		{{Name: "a", Source: []byte("bc")}, {Name: "d"}},
	}
	seen := map[string]int{}
	for index, files := range pairs {
		hash := sourceHash(files)
		if previous, ok := seen[hash]; ok {
			t.Errorf("file lists %d and %d hash identically", previous, index)
		}
		seen[hash] = index
	}
}

func TestFileSourceHash_ChangesWhenAFolderFileIsAddedRemovedRenamedOrEdited(t *testing.T) {
	dir := t.TempDir()
	write := func(name, content string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	hash := func() string {
		t.Helper()
		got, err := fileSourceHash(discardEnv(), dir)
		if err != nil {
			t.Fatalf("hash folder: %v", err)
		}
		return got
	}

	write("a.em.hcl", "# a\n")
	base := hash()
	if again := hash(); again != base {
		t.Fatalf("hash of an unchanged folder changed: %q -> %q", base, again)
	}

	write("b.em.hcl", "# b\n")
	added := hash()
	if added == base {
		t.Error("hash did not change when a file was added")
	}

	write("b.em.hcl", "# b edited\n")
	edited := hash()
	if edited == added {
		t.Error("hash did not change when a file was edited")
	}

	if err := os.Rename(filepath.Join(dir, "b.em.hcl"), filepath.Join(dir, "c.em.hcl")); err != nil {
		t.Fatalf("rename: %v", err)
	}
	renamed := hash()
	if renamed == edited {
		t.Error("hash did not change when a file was renamed")
	}

	if err := os.Remove(filepath.Join(dir, "c.em.hcl")); err != nil {
		t.Fatalf("remove: %v", err)
	}
	if removed := hash(); removed != base {
		t.Errorf("hash after removing the added file = %q, want the original %q", removed, base)
	}
}

func TestFileSourceHash_ReportsAnUnreadableModel(t *testing.T) {
	if _, err := fileSourceHash(discardEnv(), filepath.Join(t.TempDir(), "missing")); err == nil {
		t.Fatal("expected an error for a path that does not exist")
	}
}

func TestRegenerate_RendersAFolderModel(t *testing.T) {
	s := &state{}
	folder := filepath.Join("..", "..", "examples", "pet-clinic")
	if err := regenerate(discardEnv(), s, folder, app.Valid); err != nil {
		t.Fatalf("regenerate folder: %v", err)
	}
	result, _ := s.snapshot()
	if result.HTML == "" {
		t.Fatalf("expected the folder to render, got diagnostics %+v", result.Diagnostics)
	}
}

func TestWatch_RegeneratesWhenAFileIsAddedToTheFolder(t *testing.T) {
	originalPollInterval := pollInterval
	pollInterval = 20 * time.Millisecond
	defer func() { pollInterval = originalPollInterval }()

	dir := t.TempDir()
	seed, err := os.ReadFile(filepath.Join("..", "..", "examples", "minimal.em.hcl"))
	if err != nil {
		t.Fatalf("read seed example: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "model.em.hcl"), seed, 0o644); err != nil {
		t.Fatalf("write seed file: %v", err)
	}

	s := &state{}
	if err := regenerate(discardEnv(), s, dir, app.Valid); err != nil {
		t.Fatalf("initial regenerate: %v", err)
	}
	_, hashBefore := s.snapshot()
	startHash, err := fileSourceHash(discardEnv(), dir)
	if err != nil {
		t.Fatalf("hash seed folder: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go watch(ctx, discardEnv(), s, dir, app.Valid, startHash)
	// A duplicate of the seed's blocks makes the merged model invalid, so
	// the regenerated state clears the HTML. The file is written under a
	// non-member name and renamed into place, so a poll never reads it
	// half-written.
	temp := filepath.Join(dir, "other.tmp")
	if err := os.WriteFile(temp, seed, 0o644); err != nil {
		t.Fatalf("write temp file: %v", err)
	}
	if err := os.Rename(temp, filepath.Join(dir, "other.em.hcl")); err != nil {
		t.Fatalf("add file: %v", err)
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		result, hashAfter := s.snapshot()
		if hashAfter != hashBefore {
			if result.HTML != "" {
				t.Fatal("expected the duplicate declarations to clear the HTML")
			}
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("watch did not regenerate after a file was added to the folder")
}

func TestWatch_PublishesDiagnosticsWhenTheLastMemberIsRemovedAndRecovers(t *testing.T) {
	originalPollInterval := pollInterval
	pollInterval = 20 * time.Millisecond
	defer func() { pollInterval = originalPollInterval }()

	dir := t.TempDir()
	member := filepath.Join(dir, "model.em.hcl")
	seed, err := os.ReadFile(filepath.Join("..", "..", "examples", "minimal.em.hcl"))
	if err != nil {
		t.Fatalf("read seed example: %v", err)
	}
	if err := os.WriteFile(member, seed, 0o644); err != nil {
		t.Fatalf("write seed file: %v", err)
	}

	s := &state{}
	if err := regenerate(discardEnv(), s, dir, app.Valid); err != nil {
		t.Fatalf("initial regenerate: %v", err)
	}
	loadedResult, loadedHash := s.snapshot()
	if loadedResult.HTML == "" {
		t.Fatalf("expected the seed folder to render, got diagnostics %+v", loadedResult.Diagnostics)
	}
	startHash, err := fileSourceHash(discardEnv(), dir)
	if err != nil {
		t.Fatalf("hash seed folder: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go watch(ctx, discardEnv(), s, dir, app.Valid, startHash)

	if err := os.Remove(member); err != nil {
		t.Fatalf("remove member: %v", err)
	}

	var emptyHash string
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		result, hash := s.snapshot()
		if hash != loadedHash {
			if result.HTML != "" {
				t.Fatal("expected a folder with no member to clear the HTML")
			}
			found := false
			for _, diagnostic := range result.Diagnostics {
				if diagnostic.Code == "EM001" {
					found = true
				}
			}
			if !found {
				t.Fatalf("diagnostics = %+v, want an EM001 diagnostic", result.Diagnostics)
			}
			emptyHash = hash
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if emptyHash == "" {
		t.Fatal("watch did not publish the unloadable folder")
	}

	// Restore the member through a rename so a poll never reads it half-written.
	temp := filepath.Join(dir, "model.tmp")
	if err := os.WriteFile(temp, seed, 0o644); err != nil {
		t.Fatalf("write temp file: %v", err)
	}
	if err := os.Rename(temp, member); err != nil {
		t.Fatalf("restore member: %v", err)
	}

	deadline = time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		result, hash := s.snapshot()
		if hash != emptyHash {
			if result.HTML == "" {
				t.Fatalf("expected the restored member to render again, got diagnostics %+v", result.Diagnostics)
			}
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("watch did not render again after the member was restored")
}

func TestState_HashChangesWhenOnlyADiagnosticFilenameChanges(t *testing.T) {
	diagnostic := app.Diagnostic{Code: "EM002", Severity: "Error", Summary: "Duplicate ID", Line: 3, Column: 1}
	inA, inB := diagnostic, diagnostic
	inA.Filename = "model/a.em.hcl"
	inB.Filename = "model/b.em.hcl"

	hashA := hashResult(app.RenderResult{Diagnostics: []app.Diagnostic{inA}})
	hashB := hashResult(app.RenderResult{Diagnostics: []app.Diagnostic{inB}})
	if hashA == hashB {
		t.Error("hash did not change when only the diagnostic's file changed")
	}
}

func TestDiagnosticsPage_ShowsTheFileNameOfADiagnostic(t *testing.T) {
	page := string(diagnosticsPage([]app.Diagnostic{
		{Code: "EM002", Severity: "Error", Summary: "Duplicate ID", Line: 3, Column: 7, Filename: "model/b.em.hcl"},
		{Code: "EM001", Severity: "Error", Summary: "Failed to read model"},
	}))

	if !strings.Contains(page, "model/b.em.hcl (line 3, column 7)") {
		t.Errorf("diagnostics page = %q, want it to show the file with the location", page)
	}
	if strings.Contains(page, "(line 0") {
		t.Errorf("diagnostics page = %q, want no location for a diagnostic without one", page)
	}
}
