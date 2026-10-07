package serve

import (
	"bytes"
	"context"
	"errors"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/event-modeling-hcl/eventmodeling-hcl/internal/app"
	"github.com/event-modeling-hcl/eventmodeling-hcl/internal/syntax"
)

// syncBuffer is a bytes.Buffer safe for concurrent Write (from the
// production goroutine under test) and String (read by a test goroutine
// polling for output) — start and watch write to stdout/stderr from a
// background goroutine while these tests poll for the resulting text.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

type failingListener struct {
	err error
}

func (l failingListener) Accept() (net.Conn, error) { return nil, l.err }
func (failingListener) Close() error                { return nil }
func (failingListener) Addr() net.Addr              { return testAddr("127.0.0.1:43210") }

type testAddr string

func (a testAddr) Network() string { return "tcp" }
func (a testAddr) String() string  { return string(a) }

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// fakeSignals returns a signals func backed by a channel the test controls
// directly, plus that same channel, so a test can simulate SIGINT/SIGTERM
// without touching the real OS signal machinery.
func fakeSignals() (chan os.Signal, func() (<-chan os.Signal, func())) {
	ch := make(chan os.Signal, 1)
	return ch, func() (<-chan os.Signal, func()) {
		return ch, func() {}
	}
}

func testModelPath() string {
	return filepath.Join("..", "..", "examples", "minimal.em.hcl")
}

// runStart starts the server in the background against env and returns a
// channel that receives start's eventual return value, so tests can
// synchronize on the moment they trigger shutdown.
func runStart(env environment) <-chan error {
	done := make(chan error, 1)
	go func() {
		done <- start(testModelPath(), "127.0.0.1", 0, app.Valid, env)
	}()
	return done
}

func TestStart_OpensBrowserAtTheServedURL(t *testing.T) {
	sigCh, signals := fakeSignals()
	urlCh := make(chan string, 1)

	env := environment{
		readModel:   app.ReadModel,
		listen:      net.Listen,
		openBrowser: func(url string) { urlCh <- url },
		stdout:      &bytes.Buffer{},
		stderr:      &bytes.Buffer{},
		signals:     signals,
	}

	done := runStart(env)

	select {
	case url := <-urlCh:
		if !strings.HasPrefix(url, "http://127.0.0.1:") {
			t.Errorf("openBrowser called with %q, want an http://127.0.0.1:<port> URL", url)
		}
		if strings.HasSuffix(url, ":0") {
			t.Errorf("openBrowser called with the requested port zero instead of the bound port: %q", url)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("openBrowser was never invoked")
	}

	sigCh <- os.Interrupt
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("start returned an error after clean shutdown: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("start did not return after the shutdown signal")
	}
}

func TestStart_ShutsDownCleanlyOnSignalAndPrintsTheShutdownLine(t *testing.T) {
	sigCh, signals := fakeSignals()
	stdout := &syncBuffer{}

	env := environment{
		readModel:   app.ReadModel,
		listen:      net.Listen,
		openBrowser: func(string) {},
		stdout:      stdout,
		stderr:      &bytes.Buffer{},
		signals:     signals,
	}

	done := runStart(env)

	// Give start a moment to reach server.Serve before signaling, so the
	// signal is unambiguously a shutdown trigger rather than a race with
	// startup.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) && !strings.Contains(stdout.String(), "Serving ") {
		time.Sleep(5 * time.Millisecond)
	}

	sigCh <- os.Interrupt
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("start returned an error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("start did not return after the shutdown signal")
	}

	if !strings.Contains(stdout.String(), "Shutting down server...") {
		t.Errorf("stdout = %q, want it to contain the shutdown line", stdout.String())
	}
	if !strings.Contains(stdout.String(), "Serving "+testModelPath()+" at http://127.0.0.1:") {
		t.Errorf("stdout = %q, want it to contain the serving line", stdout.String())
	}
}

func TestStart_ReadModelErrorOnInitialRegeneratePropagates(t *testing.T) {
	_, signals := fakeSignals()

	env := environment{
		readModel: func(string) ([]syntax.File, app.Diagnostics) {
			return nil, app.Diagnostics{{Severity: "Error", Summary: "Failed to read model", Detail: "boom: cannot read model"}}
		},
		listen:      net.Listen,
		openBrowser: func(string) {},
		stdout:      &bytes.Buffer{},
		stderr:      &bytes.Buffer{},
		signals:     signals,
	}

	err := start(testModelPath(), "127.0.0.1", 0, app.Valid, env)
	if err == nil || !strings.Contains(err.Error(), "boom: cannot read model") {
		t.Fatalf("start() error = %v, want it to carry the read failure detail", err)
	}
}

func TestStart_ListenErrorPropagates(t *testing.T) {
	sentinel := errors.New("boom: cannot listen")
	_, signals := fakeSignals()

	env := environment{
		readModel: app.ReadModel,
		listen: func(network, address string) (net.Listener, error) {
			return nil, sentinel
		},
		openBrowser: func(string) {},
		stdout:      &bytes.Buffer{},
		stderr:      &bytes.Buffer{},
		signals:     signals,
	}

	err := start(testModelPath(), "127.0.0.1", 0, app.Valid, env)
	if !errors.Is(err, sentinel) {
		t.Fatalf("start() error = %v, want it to wrap/equal %v", err, sentinel)
	}
}

func TestStart_ServeFailureStopsSignalWorker(t *testing.T) {
	sentinel := errors.New("boom: accept failed")
	sigCh, signals := fakeSignals()
	stdout := &syncBuffer{}
	env := environment{
		readModel:   app.ReadModel,
		listen:      func(string, string) (net.Listener, error) { return failingListener{err: sentinel}, nil },
		openBrowser: func(string) {},
		stdout:      stdout,
		stderr:      &bytes.Buffer{},
		signals:     signals,
	}

	err := start(testModelPath(), "127.0.0.1", 0, app.Valid, env)
	if !errors.Is(err, sentinel) {
		t.Fatalf("start() error = %v, want %v", err, sentinel)
	}

	sigCh <- os.Interrupt
	time.Sleep(20 * time.Millisecond)
	if strings.Contains(stdout.String(), "Shutting down server...") {
		t.Fatal("signal worker remained active after start returned")
	}
}

func TestWatch_PrintsDiagramUpdatedToTheInjectedStdout(t *testing.T) {
	originalPollInterval := pollInterval
	pollInterval = 20 * time.Millisecond
	defer func() { pollInterval = originalPollInterval }()

	dir := t.TempDir()
	path := filepath.Join(dir, "model.em.hcl")
	initial, err := os.ReadFile(testModelPath())
	if err != nil {
		t.Fatalf("read seed example: %v", err)
	}
	if err := os.WriteFile(path, initial, 0o644); err != nil {
		t.Fatalf("write seed file: %v", err)
	}

	stdout := &syncBuffer{}
	env := environment{
		readModel: app.ReadModel,
		stdout:    stdout,
		stderr:    &bytes.Buffer{},
	}

	s := &state{}
	if err := regenerate(env, s, path, app.Valid); err != nil {
		t.Fatalf("initial regenerate: %v", err)
	}
	startHash, err := fileSourceHash(env, path)
	if err != nil {
		t.Fatalf("hash seed file: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go watch(ctx, env, s, path, app.Valid, startHash)

	changed, err := os.ReadFile(filepath.Join("..", "..", "testdata", "invalid", "reverse-flow.em.hcl"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	if err := os.WriteFile(path, changed, 0o644); err != nil {
		t.Fatalf("rewrite file: %v", err)
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if strings.Contains(stdout.String(), "Diagram updated.") {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("stdout = %q, want it to contain %q", stdout.String(), "Diagram updated.")
}

func TestWatch_PrintsRegenerationErrorToTheInjectedStderr(t *testing.T) {
	originalPollInterval := pollInterval
	pollInterval = 20 * time.Millisecond
	defer func() { pollInterval = originalPollInterval }()

	dir := t.TempDir()
	path := filepath.Join(dir, "model.em.hcl")
	initial, err := os.ReadFile(testModelPath())
	if err != nil {
		t.Fatalf("read seed example: %v", err)
	}
	if err := os.WriteFile(path, initial, 0o644); err != nil {
		t.Fatalf("write seed file: %v", err)
	}
	changed, err := os.ReadFile(filepath.Join("..", "..", "testdata", "invalid", "reverse-flow.em.hcl"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	// The fake readModel lets every read of the *seed* content through
	// (there may be any number of watch ticks before the rewrite lands),
	// but once it sees the *changed* content for the first time — which is
	// necessarily watch's own fileSourceHash poll, since that always runs
	// before regenerate — it fails every read after that. That forces the
	// very next read (regenerate's) to fail, regardless of tick timing.
	var latchMu sync.Mutex
	seenChangedOnce := false
	stderrBuf := &syncBuffer{}
	env := environment{
		readModel: func(p string) ([]syntax.File, app.Diagnostics) {
			files, diagnostics := app.ReadModel(p)
			if diagnostics.HasErrors() {
				return nil, diagnostics
			}
			latchMu.Lock()
			defer latchMu.Unlock()
			if bytes.Equal(files[0].Source, changed) {
				if seenChangedOnce {
					return nil, app.Diagnostics{{Severity: "Error", Summary: "Failed to read model", Detail: "boom: cannot read on rewatch"}}
				}
				seenChangedOnce = true
			}
			return files, nil
		},
		stdout: &bytes.Buffer{},
		stderr: stderrBuf,
	}

	s := &state{}
	if err := regenerate(env, s, path, app.Valid); err != nil {
		t.Fatalf("initial regenerate: %v", err)
	}
	startHash, err := fileSourceHash(env, path)
	if err != nil {
		t.Fatalf("hash seed file: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go watch(ctx, env, s, path, app.Valid, startHash)

	if err := os.WriteFile(path, changed, 0o644); err != nil {
		t.Fatalf("rewrite file: %v", err)
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if strings.Contains(stderrBuf.String(), "regeneration error") {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("stderr = %q, want it to contain %q", stderrBuf.String(), "regeneration error")
}

func TestDefaultEnvironment_WiresUpTheRealSeams(t *testing.T) {
	env := defaultEnvironment()

	if env.readModel == nil || env.listen == nil || env.openBrowser == nil {
		t.Fatal("defaultEnvironment left a seam nil")
	}
	if env.stdout == nil || env.stderr == nil {
		t.Fatal("defaultEnvironment left stdout/stderr nil")
	}
	if env.signals == nil {
		t.Fatal("defaultEnvironment left signals nil")
	}

	ch, stop := env.signals()
	if ch == nil {
		t.Fatal("default signals() returned a nil channel")
	}
	stop()
}
