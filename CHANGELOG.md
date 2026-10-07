# Changelog

This repository records implementation releases of the validator, formatter,
and typed IR. The authoritative Event Modeling HCL language history is in the
[specification changelog](https://github.com/event-modeling-hcl/spec/blob/main/CHANGELOG.md).

## [Unreleased]

### Added

- Folder models, as Specification v0.4.0 and RFC 0002 define them. A folder is one model. Every `.em.hcl` file directly in the folder joins the model, in file name order.
- The new diagnostics EM013 (chapters in several files), EM014 (workflow in several chapters), and EM407 (workflow outside every chapter).
- The example folder `examples/pet-clinic/`.
- `emhcl serve` reloads the page when a member file of the model folder is added, removed, renamed, or edited. A member file is a `.em.hcl` file directly in the folder with a name that does not start with a dot. An empty folder shows error EM001 in `serve`.
- The browser playground has file tabs when the host page has a `#files` element. You can add, rename, and remove files. The playground renders all tabs as one folder model, and a click on a diagnostic opens its file.
- `eventModelingRender` in the WebAssembly module accepts an array of `{name, source}` files as well as one string. Each diagnostic has a `file` field.

### Changed

- `emhcl validate`, `emhcl diagram`, `emhcl serve`, and `emhcl export` accept a `.em.hcl` file or a folder.
- The detail of EM002 names the first declaration as `file:line:column`. This applies only to duplicate top-level declarations: catalog blocks, chapters, hotspots, workflows, and the events, aggregates, and field types that a bounded context owns. It does not apply to workflow children or fields.
- Each diagnostic names the member file that it points at.
- `emhcl fmt` refuses a folder and formats one `.em.hcl` file at a time.

## [v0.8.0] - 2026-10-07

### Added

- The command `emhcl export <model.em.hcl> [-o file.json]` converts a valid model to slice-based Event Modeling JSON. Without `-o`, it writes the JSON to stdout.
- The command `emhcl import <file.json> [-o model.em.hcl]` converts Event Modeling JSON to formatted `.em.hcl` and validates the result. Without `-o`, it writes the source to stdout.
- Import accepts a document only if the published Event Modeling JSON schema accepts it, the same result as `ajv`. Export applies the same rules to its output. An error names the JSON path of the fault.
- Import keeps the `sliceType` that the document declares. It omits a child that the workflow kind does not allow, and it writes a warning.
- Import and export keep these items: the order of each `to` and `from` list, bounded-context identity, prototype objects, field examples, specification examples, and specification tags.
- Object keys in prototypes and examples keep their text and their order. Import writes a key such as `for` or `a-b` in quotes. Keys that differ only by Unicode normalization stay separate.
- Both commands write a warning at each place where data has no equivalent on the other side. Examples are canvas IDs, layout, catalog metadata, hotspots, and error-step titles that differ from the error text.
- The typed model has the new fields `example` on fields and field types. Field types also have `mapping`, `optional`, `technical_attribute`, `generated`, and `schema`.
- Import and export fail when a slice has more than one actor. JSON slices list actors without a link to screens, and Specification v0.3.0 has no actor attribute on a workflow, so no correct native form exists. Import fails for a slice that lists two or more actors, even when it has no screens. Export fails for a workflow whose screens name two or more actors. Import and export do not choose an actor for you.
- `emhcl export` refuses an `-o` target that ends in `.em.hcl` or that is the source file, and it exits with code 2. Thus it cannot overwrite the model.

### Changed

- `examples/course_subscriptions.em.hcl` has separate workflows for the student catalog view and the course-manager catalog view.

### Limits

- Conversion is a projection, not a lossless copy of a canvas. Warnings list the losses.
- Export, then import, then a second export gives the same JSON for each file in `examples/` and `testdata/valid/`. This is not true for all JSON from other tools, because import changes IDs, names, and types to native labels.
- Unicode normalization can change the text of a string value, but not the text of an object key.
- The two Cart fixtures give different `aggregate` metadata to two read models. No evidence shows which value is correct.

## [v0.7.3]

### Added

- `Compact` diagram view: one events row, each bounded context drawn as a dashed rounded box with an auto-assigned colour (pink reserved for external systems). Within a slice a box sits right of the command that emits its events and left of the read model they feed, so automation and translation slices show the upstream (given) event's box first, then the produced event's. Arrows run command → event (solid) → read model. The aggregate (cube icon) is a sticky on top of its event, without arrows; external events carry a pink external-system sticky (globe icon) instead.

## [v0.7.2] - 2026-10-02

### Added

- Zoom for the `Model` and `Storming` diagram views: toolbar controls
  (−, reset to 100%, +, Fit width), Ctrl/⌘ + scroll or trackpad pinch anchored
  at the cursor, and `+`/`-`/`0`/`F` keys. Range 25%–200%; each view keeps its
  own zoom level.

### Changed

- Hotspots in the `Model` view render as large red sticky notes (~60% of the
  pinned card) showing the question, instead of a small `?` badge. Slice
  hotspots sit in the slice's Screens row, under the slice title and above
  its actors and screens.

## [v0.7.1] - 2026-09-22

This release updates the styling of Event Modeling diagrams - it's now consistent
with the presentation of the `Storming` view.

### Changed

- Event Modeling and Context map diagrams now use a style similar to that of the Storming
view.

## [v0.7.0] - 2026-09-21

This release adds two new rendered views to the diagram canvas and completes
the `eventmodeling-hcl` removal scheduled in v0.6.0. The Event Modeling HCL
language is unchanged and still implements Specification v0.3.0.

### Added

- `Storming` view: renders the model as tactical Event Storming, one row per
  workflow with sticky notes for screens, commands, aggregates, events,
  automations/translations, and read models. An actor card renders beside
  every screen that has one (to the screen's left when the screen precedes a
  command, to its right when the screen follows a read model). A workflow
  connected to another by an edge is placed to the right of the workflow it
  depends on; independent workflows run in parallel rows. Hotspots render as
  a bigger sticky that also shows its question once field details are
  enabled. A reading-note banner clarifies how tactical Event Storming and
  Event Modeling differ as methodologies.
- `Context Map` view **(experimental)**: derives a DDD bounded-context map
  from cross-context event consumption — one node per bounded context, with
  upstream → downstream edges labeled customer/supplier, or anti-corruption
  when the consumer is a `translation` workflow. The heuristic derivation and
  the layout may still change in a future release.
- An in-page `Model` / `Storming` / `Context Map` view switcher in the
  masthead. `diagram`, `serve`, and the WebAssembly playground all render
  every view from the same generated document; no CLI flags changed.

### Removed

- The deprecated `eventmodeling-hcl` executable and its
  `github.com/event-modeling-hcl/eventmodeling-hcl/cmd/eventmodeling-hcl`
  install path, as scheduled in v0.6.0. Use `emhcl`.

## [v0.6.0] - 2026-09-14

This release renames the command-line tool to `emhcl` and adds reproducible,
versioned installation through `go install`. The Event Modeling HCL language is
unchanged and still implements Specification v0.3.0.

### Added

- Install the canonical command for the current user with
  `go install github.com/event-modeling-hcl/eventmodeling-hcl/cmd/emhcl@v0.6.0`.
  Versioned Go installs report `emhcl v0.6.0`.
- Release archives include both the canonical `emhcl` executable and the
  v0.6-only compatibility executable.

### Changed

- Renamed the command, release archives, rendered-diagram attribution, and
  playground branding from `eventmodeling-hcl` to `emhcl`.

### Deprecated

- The `eventmodeling-hcl` executable and
  `github.com/event-modeling-hcl/eventmodeling-hcl/cmd/eventmodeling-hcl`
  install path remain functional for v0.6.0 and will be removed in v0.7.0.

## [v0.5.0] - 2026-09-10

This release adds interactive ways to author and preview models, improves the
diagram's domain layout, and reorganizes the implementation around explicit
application and validation boundaries. The Event Modeling HCL language is
unchanged — it still implements Specification v0.3.0.

### Added

- `serve <model.em.hcl> [--addr H] [--port N] [--profile P]` command: a local
  edit-and-render loop that watches one model and live-reloads the rendered
  diagram in the browser. It binds `127.0.0.1:8080` by default; `--port 0`
  requests an available port and the actual URL is printed after binding.
- In-browser playground: the real parse, validate, format, and render core
  compiled to WebAssembly (`cmd/wasm`), driven by a dependency-free editor under
  `web/playground/`. Everything runs client-side — nothing typed leaves the
  browser.
- Playground integration hooks for a host-supplied editor and initial source,
  plus `eventmodeling:status`, `eventmodeling:diagnostics`, and
  `eventmodeling:rendered` events.
- `cmd/playground-seed` (with `internal/playground`) generates the playground's
  seed model from `examples/minimal.em.hcl`.
- `build-wasm` and `wasm-check` Makefile targets; `wasm-check` runs as part of
  `make verify`.
- `architecture-guide.md` documents the repository's Axiomatic Design
  requirements, influence matrix, module contracts, and change rules.

### Changed

- Rendered diagrams now group domain events into one lane per aggregate under a
  bounded-context header, replacing the single flat Events swimlane. The
  Screens, Processors, and Model rows are unchanged, and all wiring, filtering,
  drawer, and theme behavior is preserved.
- CLI, WebAssembly, and live-server use cases now share `internal/app`; source
  interpretation is centralized in `internal/source`, grammar lives in
  `internal/syntax`, and canonical model construction requires a
  validator-issued typed document.
- The Go toolchain is pinned to patched Go 1.26.6 for release and verification
  builds.


## [v0.4.0] - 2026-09-07

This release implements Event Modeling HCL Specification v0.3.0: a `field` block
may omit a redundant `type`, and a `fields` list declares several typed fields
at once.

### Added

- Field shorthand: a `field` block may omit `type` and infer the same-named
  `field_type` — resolved in the owning `bounded_context` for event and subfield
  fields, and by unique document-wide name for workflow-element fields.
- `fields = [field_type.<...>]` list attribute on events, workflow elements,
  tables, and scenario steps, synthesizing one field per entry.
- Screen `field` blocks and `fields` lists are now exercised by the shipped
  examples and covered by tests.

### Changed

- A `field_type` declaration must still set an explicit built-in `type`; only
  plain `field` blocks may use the shorthand.
- The formatter ranks the `fields` attribute with the semantic group, before
  `from` and `to`.
- The rendered HTML diagram reports Event Modeling HCL Specification v0.3.0.

## [v0.3.1] - 2026-09-07

This release polishes the diagram canvas introduced in v0.3.0.

### Changed

- Actor cards render directly beside their associated screen card instead of
  in a dedicated actor swimlane.
- Backward-pointing flow arrows (whose target sits left of their source in
  the left-to-right layout) render with a dashed stroke, reverting to solid
  only while hovered.

## [v0.3.0] - 2026-09-06

This release adds rendering tooling while retaining full compatibility with the
v0.2.0 Event Modeling HCL language specification.

### Added

- `diagram <model.em.hcl> [-o <file>]` command for generating a self-contained,
  interactive HTML Event Model canvas from the validated typed IR.
- Workflow-level `screen_image` previews, canonical flow arrows, slice filters,
  scenario details, field metadata, chapters, and hotspots in rendered diagrams.
- Content-adaptive left-to-right slice layouts with dedicated actor and processor
  swimlanes, horizontally grouped events, and actor-to-screen highlighting.
- Event Storming pink styling for events owned by external bounded contexts,
  while domain events retain their orange styling.

### Changed

- The typed IR retains field-type ID/PII metadata and scenario examples needed
  by downstream renderers.

## [v0.2.0] - 2026-09-05

Implements the v0.2.0 native HCL language surface. See the
[specification changelog](https://github.com/event-modeling-hcl/spec/blob/main/CHANGELOG.md)
for the normative language history.

### Added

- `fmt` command that canonicalizes whitespace and attribute order idempotently
  while preserving semantic block and scenario order.
- Validation profiles `--profile workshop|valid|strict`, with `valid` as the
  default and `strict` escalating unreasoned commands and open hotspots to errors.
- Stable `EMxxx` diagnostic codes, reported as
  `file:line:column: Severity EMxxx: message`.
- Typed IR via `internal/model.Load`: a validation-gated model with effective
  titles, normalized source-to-target edges, and semantic/presentation separation.
- Invalid fixtures for reverse-flow and State-View `when` scenarios.

### Changed

- Moves the public Go module to
  `github.com/event-modeling-hcl/eventmodeling-hcl`.
- Enforces one canonical flow spelling per edge; reverse forms now fail validation.
- State View scenarios take one or more event `given` steps and `then` read-model
  steps, with no `when`.
- Titles are optional and derived from labels when absent.

### Removed

- The `query` scenario target and reverse flow spellings.

## [v0.1.0] - 2026-09-02

- Initial native HCL validator release.
