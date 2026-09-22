GO ?= go
BINARY := bin/emhcl
EXAMPLES := $(wildcard examples/*.em.hcl)
WASM_DIR := web/playground

.DEFAULT_GOAL := help

.PHONY: help build test test-race vet fmt-check tidy-check staticcheck exhaustive vulncheck validate-examples build-wasm wasm-check release-tag-check release-annotation-check release-version-check verify clean

help: ## List available commands.
	@awk 'BEGIN {FS = ":.*##"} /^[a-zA-Z0-9_-]+:.*##/ {printf "%-18s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

build: $(BINARY) ## Build the validator into bin/.

$(BINARY): $(shell find cmd internal -type f -name '*.go') go.mod go.sum
	@mkdir -p $(@D)
	$(GO) build -o $@ ./cmd/emhcl

test: ## Run unit and fixture tests.
	$(GO) test ./...

test-race: ## Run tests with the race detector.
	$(GO) test -race ./...

vet: ## Run go vet.
	$(GO) vet ./...

fmt-check: ## Fail if tracked Go files are not gofmt-formatted.
	@unformatted=$$(git ls-files -co --exclude-standard '*.go' | while read -r file; do \
		test ! -f "$$file" || $(GO)fmt -l "$$file"; \
	done); \
	test -z "$$unformatted"

tidy-check: ## Fail if go.mod or go.sum need tidying.
	$(GO) mod tidy
	git diff --exit-code -- go.mod go.sum

staticcheck: ## Run Staticcheck (downloads its CI-pinned version if needed).
	$(GO) run honnef.co/go/tools/cmd/staticcheck@2026.1 ./...

exhaustive: ## Check switch-statement exhaustiveness over local enum types.
	$(GO) tool exhaustive -default-signifies-exhaustive=false ./...

vulncheck: ## Run the Go vulnerability checker.
	$(GO) run golang.org/x/vuln/cmd/govulncheck@v1.1.4 ./...

validate-examples: build ## Validate every shipped Event Modeling example.
	@for model in $(EXAMPLES); do \
		$(BINARY) validate "$$model" || exit $$?; \
	done

build-wasm: ## Build the WebAssembly playground module into web/playground/.
	@mkdir -p $(WASM_DIR)
	$(GO) run ./cmd/playground-seed examples/minimal.em.hcl > $(WASM_DIR)/seed.js
	GOOS=js GOARCH=wasm $(GO) build -o $(WASM_DIR)/emhcl.wasm ./cmd/wasm
	@wasm_exec="$$($(GO) env GOROOT)/lib/wasm/wasm_exec.js"; \
	if [ ! -f "$$wasm_exec" ]; then wasm_exec="$$($(GO) env GOROOT)/misc/wasm/wasm_exec.js"; fi; \
	if [ ! -f "$$wasm_exec" ]; then \
		echo "wasm_exec.js not found under GOROOT (checked lib/wasm and misc/wasm)" >&2; \
		exit 1; \
	fi; \
	cp "$$wasm_exec" $(WASM_DIR)/wasm_exec.js
	@ls -lh $(WASM_DIR)/emhcl.wasm $(WASM_DIR)/wasm_exec.js

wasm-check: ## Verify the WebAssembly module still compiles for js/wasm.
	@temporary=$$(mktemp -d); \
	trap 'rm -rf "$$temporary"' EXIT; \
	GOOS=js GOARCH=wasm $(GO) build -o "$$temporary/emhcl.wasm" ./cmd/wasm

release-tag-check: ## Test the stable semantic-version tag validator.
	sh scripts/test-require-stable-tag.sh

release-annotation-check: ## Test annotated-tag verification from a tagless checkout.
	sh scripts/test-require-annotated-tag.sh

release-version-check: ## Verify the linker-injected release version is reported by emhcl.
	@temporary=$$(mktemp -d); \
	trap 'rm -rf "$$temporary"' EXIT; \
	$(GO) build -ldflags '-X main.buildVersion=v0.7.1' -o "$$temporary/emhcl" ./cmd/emhcl; \
	test "$$($$temporary/emhcl version)" = 'emhcl v0.7.1'

verify: fmt-check tidy-check vet test test-race staticcheck exhaustive vulncheck validate-examples wasm-check release-tag-check release-annotation-check release-version-check ## Run the complete local verification suite.

clean: ## Remove locally built artifacts.
	rm -rf bin dist $(WASM_DIR)/emhcl.wasm $(WASM_DIR)/wasm_exec.js $(WASM_DIR)/seed.js

# -- pre-commit --
.PHONY: pre-commit-install pre-commit-run

pre-commit-install: ## Install the repository pre-commit hook.
	pre-commit install

pre-commit-run: ## Run all repository pre-commit hooks.
	pre-commit run --all-files
