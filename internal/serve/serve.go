// Package serve runs a local live-reload HTTP server for one .em.hcl model:
// it watches the file, re-renders it with internal/app on every change,
// and serves the result in a browser tab that reloads itself. All rendering
// is delegated to app's plain RenderResult/Diagnostic types — this
// package owns only HTTP and file-watching, never HCL parsing or the
// renderer's output format.
package serve

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"net"
	"net/http"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/event-modeling-hcl/eventmodeling-hcl/internal/app"
)

// pollInterval is how often the watch loop checks the model file's content.
// It is a var, not a const, so tests can shrink it instead of waiting on
// the real interval.
var pollInterval = 500 * time.Millisecond

// state holds the most recently rendered result and lets HTTP handlers read
// it concurrently with the file-watch goroutine that updates it.
type state struct {
	mu     sync.RWMutex
	result app.RenderResult
	hash   string
}

func (s *state) update(result app.RenderResult) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.result = result
	s.hash = hashResult(result)
}

// snapshot returns the current result and its hash together, so a caller
// can never observe a hash that doesn't match the result it read.
func (s *state) snapshot() (app.RenderResult, string) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.result, s.hash
}

// hashResult hashes everything a viewer could see: the rendered HTML, or —
// when a model has errors — its diagnostics. Hashing only the HTML would
// make any two error states indistinguishable, since both render no HTML
// at all, and the served page would never know to reload from one error to
// a different one.
func hashResult(result app.RenderResult) string {
	encoded, err := json.Marshal(result)
	if err != nil {
		// RenderResult's fields are all plain strings/ints/slices thereof;
		// this cannot fail in practice, but a distinct hash beats a panic.
		return "unhashable"
	}
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:])
}

const pollScript = `<script>
(function () {
  var hash = "";
  setInterval(function () {
    fetch("/hash").then(function (r) { return r.text(); }).then(function (h) {
      if (hash && h !== hash) location.reload();
      hash = h;
    });
  }, 1000);
})();
</script>`

// shellPage is served at "/": a minimal page whose only job is to hold the
// iframe that actually shows the diagram, plus the poll script that reloads
// the whole page (and so the iframe) when the served content changes.
func shellPage() []byte {
	return []byte(`<!doctype html>
<html>
<head>
<meta charset="utf-8">
<title>emhcl serve</title>
<style>html,body,iframe{margin:0;padding:0;border:0;width:100%;height:100%;display:block}</style>
</head>
<body>
<iframe src="/diagram"></iframe>
` + pollScript + `
</body>
</html>
`)
}

// diagnosticsPage is served at "/diagram" in place of HTML whenever the
// current model has errors, so "nothing changed" and "the model is broken"
// never look the same as a blank response.
func diagnosticsPage(diagnostics []app.Diagnostic) []byte {
	var items strings.Builder
	for _, diagnostic := range diagnostics {
		items.WriteString("<li><code>")
		items.WriteString(html.EscapeString(diagnostic.Severity))
		items.WriteString(" ")
		items.WriteString(html.EscapeString(diagnostic.Code))
		items.WriteString("</code>: ")
		items.WriteString(html.EscapeString(diagnostic.Summary))
		if diagnostic.Detail != "" {
			items.WriteString(" — ")
			items.WriteString(html.EscapeString(diagnostic.Detail))
		}
		if diagnostic.Line > 0 {
			fmt.Fprintf(&items, " (line %d, column %d)", diagnostic.Line, diagnostic.Column)
		}
		items.WriteString("</li>\n")
	}

	return []byte(`<!doctype html>
<html>
<head><meta charset="utf-8"><title>emhcl: diagnostics</title></head>
<body>
<h1>This model does not currently render</h1>
<ul>
` + items.String() + `</ul>
</body>
</html>
`)
}

// newMux builds the real, production HTTP routing for a running server —
// the same handler both Start and the test suite exercise, so a test
// against it is a test against exactly what gets served.
func newMux(s *state) *http.ServeMux {
	mux := http.NewServeMux()

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(shellPage())
	})

	mux.HandleFunc("/diagram", func(w http.ResponseWriter, r *http.Request) {
		result, _ := s.snapshot()
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if result.HTML != "" {
			_, _ = io.WriteString(w, result.HTML)
			return
		}
		_, _ = w.Write(diagnosticsPage(result.Diagnostics))
	})

	mux.HandleFunc("/hash", func(w http.ResponseWriter, r *http.Request) {
		_, hash := s.snapshot()
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = io.WriteString(w, hash)
	})

	return mux
}

// regenerate reads filePath (via env.readFile) and renders it into s, under
// profile.
func regenerate(env environment, s *state, filePath string, profile app.Profile) error {
	source, err := env.readFile(filePath)
	if err != nil {
		return err
	}
	s.update(app.Render(filePath, source, profile))
	return nil
}

func sourceHash(source []byte) string {
	sum := sha256.Sum256(source)
	return hex.EncodeToString(sum[:])
}

func fileSourceHash(env environment, filePath string) (string, error) {
	source, err := env.readFile(filePath)
	if err != nil {
		return "", err
	}
	return sourceHash(source), nil
}

// watch polls filePath every pollInterval and re-renders it whenever its
// content changes. Content, rather than mtime, is the reliable contract:
// editors and source-control tools may preserve timestamps when replacing a
// file.
func watch(ctx context.Context, env environment, s *state, filePath string, profile app.Profile, lastSourceHash string) {
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			currentSourceHash, err := fileSourceHash(env, filePath)
			if err != nil {
				continue
			}
			if currentSourceHash == lastSourceHash {
				continue
			}
			if err := regenerate(env, s, filePath, profile); err != nil {
				fmt.Fprintf(env.stderr, "emhcl serve: regeneration error: %v\n", err)
				continue
			}
			lastSourceHash = currentSourceHash
			fmt.Fprintln(env.stdout, "Diagram updated.")
		}
	}
}

// openBrowser tries to open url in the user's default browser. Failures are
// silently ignored — the server prints the URL either way.
func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "linux":
		cmd = exec.Command("xdg-open", url)
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		return
	}
	_ = cmd.Start()
}

func servingURL(listener net.Listener, requestedHost string) string {
	_, port, err := net.SplitHostPort(listener.Addr().String())
	if err != nil {
		return "http://" + listener.Addr().String()
	}
	host := requestedHost
	if host == "" || host == "0.0.0.0" || host == "::" {
		host = "localhost"
	}
	return "http://" + net.JoinHostPort(host, port)
}

// Start renders filePath once, then serves it at http://addr:port, live-
// reloading in the browser whenever filePath changes on disk, until it
// receives SIGINT. It runs against the real OS environment; see start for
// the version that takes an injected environment.
func Start(filePath string, addr string, port int, profile app.Profile) error {
	return start(filePath, addr, port, profile, defaultEnvironment())
}

// start is Start's implementation, with every external interaction routed
// through env instead of directly through the os/net/fmt packages. This is
// what tests drive, so that file reads, the listener, browser-opening,
// console output, and shutdown signaling can all be faked deterministically.
func start(filePath string, addr string, port int, profile app.Profile, env environment) error {
	s := &state{}
	if err := regenerate(env, s, filePath, profile); err != nil {
		return err
	}

	lastSourceHash, err := fileSourceHash(env, filePath)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	listener, err := env.listen("tcp", net.JoinHostPort(addr, strconv.Itoa(port)))
	if err != nil {
		return err
	}

	// Only start watching once the listener is bound, so a failed listen
	// never leaks a watch goroutine, and join that goroutine before start
	// returns: otherwise a leaked watch could still be reading pollInterval
	// or updating state after the caller believes serving has stopped.
	watchDone := make(chan struct{})
	go func() {
		defer close(watchDone)
		watch(ctx, env, s, filePath, profile, lastSourceHash)
	}()

	server := &http.Server{Handler: newMux(s)}

	sigCh, stopSignals := env.signals()
	defer stopSignals()
	signalDone := make(chan struct{})
	go func() {
		defer close(signalDone)
		select {
		case <-sigCh:
			fmt.Fprintln(env.stdout, "\nShutting down server...")
			cancel()
			_ = server.Shutdown(context.Background())
		case <-ctx.Done():
		}
	}()
	defer func() {
		cancel()
		<-watchDone
		<-signalDone
	}()

	url := servingURL(listener, addr)
	fmt.Fprintf(env.stdout, "Serving %s at %s\n", filePath, url)
	env.openBrowser(url)

	if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}
