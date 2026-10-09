# Architecture Guide

This guide describes how `emhcl` is organized. It also shows how the design uses Axiomatic Design to keep changes predictable. The guide describes the current repository. The language contract is in the [Event Modeling HCL specification](https://github.com/event-modeling-hcl/spec). The tool implements Specification v0.4.0, which includes folder models.

## Axiomatic Design terms

Axiomatic Design separates what a system must do from the mechanisms that do it. This guide uses these terms:

- A need is an outcome that a user or a maintainer expects.
- A functional requirement (FR) is a testable outcome that does not name a mechanism.
- A design parameter (DP) is the mechanism that satisfies an FR.
- The Independence Axiom states that each FR must stay controllable by its own DP.
- The Information Axiom prefers the design with the highest probability of meeting all FRs.

An influence matrix shows which DP changes which FR. Each row is an FR and each column is a DP. `X` means that a change to the DP changes the FR. `0` means that the DP has no material influence on the FR in the supported range. The supported range is one model per run. A model is one file or one folder of files.

A design is uncoupled when the matrix has `X` only on the diagonal. A design is decoupled when the matrix is triangular. A decoupled design is valid when you set the DPs in the order of the matrix. A design is coupled when no order makes the matrix triangular.

Information content is `I = -log2(p)`, where `p` is the probability of meeting the requirement. Source lines and package counts do not measure it.

## Needs, requirements, and parameters

| Need | Functional requirement | Acceptance criterion | Design parameter |
| --- | --- | --- | --- |
| N1: Authors receive feedback that they can trust | FR1: Parse native `.em.hcl` syntax | Syntax errors keep their HCL source locations | DP1: `internal/syntax` grammar and parser |
| N2: Every frontend sees the same model | FR8: Load a model from a file or a folder | A path to a file or a folder gives the same ordered files every time, and each diagnostic names its own file | DP8: Model loader (`app.ReadModel`, `syntax.ParseFiles`, `syntax.Document.SourceOf`) |
| N1 | FR2: Decode shared source facts once | One owner interprets catalogs and references | DP2: `internal/source` decoded document |
| N1 | FR3: Enforce language and modeling rules | Invalid models produce stable diagnostics for the selected profile | DP3: `internal/validator` semantic passes |
| N2: Every frontend sees the same model | FR4: Construct one canonical model | Only validated input reaches model construction | DP4: `internal/model` lowering |
| N3: Users can consume models in useful forms | FR5: Format or render the same output for the same input | Formatting and HTML generation do no OS actions | DP5: `internal/formatter` and `internal/renderer` |
| N5: Users exchange models with Event Modeling JSON tools | FR7: Convert between the canonical model and slice-based JSON | Each conversion gives a schema-valid document or a native model, or it fails with a stated reason | DP7: `internal/interchange` |
| N4: The CLI, the browser, and the server agree | FR6: Compose and expose the use cases | Each entry point delegates to one application service | DP6: `internal/app` and thin runtime adapters |

Slice-based JSON is the document format that the [published Event Modeling JSON schema](https://github.com/dilgerma/event-modeling-spec/blob/main/eventmodeling.schema.json) defines. A slice is one JSON entry that matches one native workflow.

## Independence matrix for the tool

| FR \ DP | DP1 Syntax | DP8 Loader | DP2 Source | DP3 Validation | DP4 Model | DP5 Output | DP7 Interchange | DP6 Adapters |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| FR1 Parse syntax | X | 0 | 0 | 0 | 0 | 0 | 0 | 0 |
| FR8 Load a model from a file or a folder | X | X | 0 | 0 | 0 | 0 | 0 | 0 |
| FR2 Decode source facts | X | X | X | 0 | 0 | 0 | 0 | 0 |
| FR3 Enforce validity | X | X | X | X | 0 | 0 | 0 | 0 |
| FR4 Construct canonical model | X | X | X | X | X | 0 | 0 | 0 |
| FR5 Produce the same output | X | X | X | X | X | X | 0 | 0 |
| FR7 Convert to and from JSON | X | X | X | X | X | 0 | X | 0 |
| FR6 Expose consistent operations | X | X | X | X | X | X | X | X |

The matrix is lower triangular, so the design is decoupled. The control sequence for the native pipeline is:

```text
app.ReadModel
  -> syntax.ParseFiles
  -> source.Decode
  -> validator.ValidateDecodedDocument
  -> model.Build
  -> renderer.Render
```

`validator.ValidatedDocument` enforces this sequence. `model.Build` does not accept input that was only parsed. Formatting is a separate branch. It needs HCL that parses, but it does not need a valid model.

The important zeros go in one direction. A change to the renderer or the CLI cannot change what syntax the parser accepts. A change to the model or the renderer cannot make invalid input valid. Adapters do not define a second diagnostic policy or a second rendering policy.

The DP8 column has an `X` in every row from FR8 down, and a `0` in the FR1 row. The FR8 entry is the diagonal. FR1 has a `0` because the syntax rules do not depend on which files the loader finds. The other entries are real influences:

- FR2: `source.Decode` reads the merged body that DP1 and DP8 produce, so file discovery changes the facts that it decodes.
- FR3: The validator uses `FileCount()` to turn on EM013, EM014, and EM407 and to turn off EM006, so file discovery changes validation. The sort order also changes which duplicate declaration counts as the first one.
- FR4: The sort order changes the order of catalog items and of unchaptered workflows, and it changes which duplicate is first.
- FR5, FR7, and FR6: These rows depend on DP8 through DP2, DP3, and DP4, in the same way that the guide treats DP1.

DP8 comes after DP1 in the build order because `syntax.File` and `syntax.ParseFiles` belong to the syntax package.

The FR7 row has `X` under DP1 to DP4 for two reasons. Export reads only the canonical model. Import must write HCL that the grammar and the validator accept. The `0` under DP5 is correct because `interchange.Import` returns unformatted source. `internal/app` formats it afterward. The `0` under DP6 is correct because the conversion result is the same for every entry point.

## Interchange sequence

`internal/app` runs each conversion in one fixed sequence.

Export:

```text
app.ValidatedModel (profile valid)
  -> interchange.Export
  -> interchange.MarshalDocument (schema check)
```

Import:

```text
interchange.ParseDocument (schema check, then decode)
  -> interchange.Import
  -> app.Format
  -> app.Validate (profile valid)
```

Import sends its result through the native formatter and the native validator. Thus `internal/validator` stays the only owner of model validity. `internal/interchange` depends only on `internal/model` and the HCL libraries.

## Requirements for the interchange module

FR7 has six child requirements. The table gives each one its mechanism.

| Child FR | Acceptance criterion | DP | Location |
| --- | --- | --- | --- |
| FR7.1: Accept and write only schema-valid documents | Import accepts a document only when `ajv` accepts it. Export applies the same rules to its output. | DP7.1: Schema check | `conform.go`, `decode.go` |
| FR7.2: Map JSON identities to stable native labels | One JSON context gives one native context. An event without an `id` keeps its own flows. | DP7.2: Identity mapping | `import.go` context and event-occurrence maps |
| FR7.3: Keep JSON values unchanged | Object keys keep their text and their order. Keys such as `for` and `a-b` produce valid HCL. | DP7.3: Value writer and reader | `jsonTokens` in `import.go`, `jsonValue` in `internal/model` |
| FR7.4: Write only flows that the grammar allows | Import writes no illegal flow. Each omitted flow has a warning. | DP7.4: Flow and scenario policy | `import_policy.go` |
| FR7.5: Keep at most one actor per slice | Import refuses a slice with two or more actors. Export refuses a workflow whose screens name two or more actors. | DP7.5: Actor gate | `rejectMultipleActors` in `import.go`, `slice` in `export.go` |
| FR7.6: Disclose each loss | Each value that a conversion drops produces a warning. | DP7.6: Warnings at the point of loss | `warn` calls in `import.go`, `import_policy.go`, `export.go` |

The actor gate follows from the language. Specification v0.4.0 gives a screen one optional actor and gives a workflow no actor attribute. A JSON slice lists actors without links to screens. Thus a slice with two actors has no correct native form. The gate stops the conversion instead of guessing.

Round-trip stability is an acceptance criterion, not a separate FR. A round trip is export, then import, then a second export. The round trip is stable when the second export is identical to the first, byte for byte. It is the result of FR7.1 to FR7.6 together, so it has no DP of its own.

## Independence matrix for the interchange module

| FR \ DP | DP7.1 Schema | DP7.2 Identity | DP7.3 Values | DP7.4 Flows | DP7.5 Actors | DP7.6 Warnings |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| FR7.1 Schema-valid documents | X | 0 | 0 | 0 | 0 | 0 |
| FR7.2 Stable labels | X | X | 0 | 0 | 0 | 0 |
| FR7.3 Unchanged values | X | 0 | X | 0 | 0 | 0 |
| FR7.4 Legal flows | X | X | 0 | X | 0 | 0 |
| FR7.5 One actor per slice | X | 0 | 0 | 0 | X | 0 |
| FR7.6 Disclosed losses | X | X | X | X | X | X |

This matrix is also lower triangular, so the module is decoupled. Set the DPs in this order: schema check, identity mapping, value writer, flow policy, actor gate, warnings.

The first column has `X` in every row because each later step reads only the decoded document. The repository gave evidence for the FR7.3 entry. `ParseDocument` once encoded the checked document a second time. That step sorted the keys in every prototype. It now decodes the original bytes.

FR7.4 depends on DP7.2 because each flow refers to identities that DP7.2 resolves. An empty event `id` once sent flows to the wrong event. Event occurrences now have their own map.

FR7.6 depends on every DP, because each DP decides what it drops. For this reason, each DP writes its own warning where the loss occurs. No separate step tries to find losses later.

The zeros in the FR7.3 row are correct. Labels and flows do not change the content of a value.

## Influences across branches

Two influences cross from other branches into FR7. Each one has a cost when it changes.

DP3 influences FR7.4. `import_policy.go` repeats the flow rules of `internal/validator` so that import writes no illegal flow. If you change a flow rule in the validator, you must change `import_policy.go` in the same change.

DP4 influences FR7.3. Export reads JSON values from the canonical model. `jsonValue` in `internal/model` builds each value from the HCL source text. It does not convert the value through `cty` first, because `cty` normalizes Unicode keys. Normalization would merge the keys `é` and `e\u0301`.

`internal/serve` imports `internal/syntax` only for the `syntax.File` type that `app.ReadModel` returns. It hashes those files to detect a change. It does not parse or select member files, so the loader rules R1 to R4 keep one owner.

## Requirements for the diagram viewer

FR5 includes the HTML diagram. `internal/renderer` writes one file that holds the model as JSON and the viewer scripts in `internal/renderer/assets`. The browser runs the viewer. The viewer has eight child requirements.

| Child FR | Acceptance criterion | DP | Location |
| --- | --- | --- | --- |
| FR5.1: Scope each page to one chapter or to the whole model | A fragment gives the same chapter every time. Slices outside every chapter form an Ungrouped chapter. | DP5.1: Route grammar and load-time scoping | `parseHash`, `normalizeChapters`, and `MODEL` at the top of `viewer.js` |
| FR5.2: Give every view the same read-only facts | Each view reads elements, edges, labels, and the slice timeline from one place | DP5.2: Shared index and helpers | `ELEMENTS`, `EDGES`, `NEIGHBORS`, and `EMC.sliceTimeline` in `viewer.js` |
| FR5.3: Show the details of one slice on request | The drawer shows the scenarios and elements of the slice that the reader selects | DP5.3: Detail drawer | `openSlice` in `viewer.js` |
| FR5.4: Filter the boards by status and bounded context | One filter state applies to every board. The controls offer only values on the current board. | DP5.4: Filter owner | `filterState`, `applyFilters`, and `EMC.onFilterChange` in `viewer.js` |
| FR5.5: Draw the Model, Compact, Storming, and Context Map boards | Each board renders on its own and applies the filter state | DP5.5: Board renderers | `renderEventModel` in `viewer.js`, `viewer.compact.js`, `viewer.eventstorming.js`, `viewer.contextmap.js` |
| FR5.6: Preview every chapter | Each card shows the first and last slice of its chapter | DP5.6: Chapter overview | `viewer.chapters.js` |
| FR5.7: Present one slice to readers who do not know Event Modeling | The slide shows the screen, then the examples, then the event model of the slice | DP5.7: Slides | `viewer.slides.js` |
| FR5.8: Navigate between pages and views | Each fragment shows one page. Browser Back works. Links work inside the sandboxed `srcdoc` frame of the playground. | DP5.8: Navigation | `parseRoute`, `renderNavigation`, `showPage`, and the fragment click handler in `viewer.js` |

## Independence matrix for the diagram viewer

| FR \ DP | DP5.1 Scope | DP5.2 Index | DP5.3 Drawer | DP5.4 Filters | DP5.5 Boards | DP5.6 Overview | DP5.7 Slides | DP5.8 Navigation |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| FR5.1 Scope | X | 0 | 0 | 0 | 0 | 0 | 0 | 0 |
| FR5.2 Shared facts | X | X | 0 | 0 | 0 | 0 | 0 | 0 |
| FR5.3 Detail drawer | X | X | X | 0 | 0 | 0 | 0 | 0 |
| FR5.4 Filters | X | X | 0 | X | 0 | 0 | 0 | 0 |
| FR5.5 Boards | X | X | X | X | X | 0 | 0 | 0 |
| FR5.6 Overview | X | X | 0 | 0 | 0 | X | 0 | 0 |
| FR5.7 Slides | X | X | 0 | 0 | 0 | 0 | X | 0 |
| FR5.8 Navigation | X | 0 | 0 | 0 | X | X | X | X |

The matrix is lower triangular, so the viewer is decoupled. Set the DPs in this order: scope, shared index, drawer, filters, boards, overview, slides, navigation.

The off-diagonal entries are these influences:

- Every view reads the scoped `MODEL`, so DP5.1 influences FR5.2 to FR5.7. Navigation reads the route grammar of DP5.1.
- The boards open the drawer when the reader clicks a slice, and they apply the filter state of DP5.4.
- The overview and the slides draw a slice with `EMC.sliceTimeline` from DP5.2.
- Navigation calls the render function of the page that it shows, so DP5.5, DP5.6, and DP5.7 influence FR5.8.

The important zeros are these:

- No board influences another board. Each board registers its own filter function with `EMC.onFilterChange` when it renders. A Storming page that you open directly does not render the Model board.
- The slides do not use the filter state. They show every fact of one slice.
- The overview does not influence the slides. The slice timeline belongs to DP5.2, not to the overview.
- Navigation does not change what a view draws. It only selects the page and passes the route values, such as the slide number.

The viewer had three couplings that this decomposition removed:

- The filter state and its controls were inside `renderEventModel`. That function also called the filter functions of the Storming and Compact boards. Thus FR5.4 depended on the Model board, and the Storming and Compact pages rendered the Model board first. Now DP5.4 owns the state and the controls, and each board registers itself.
- The slides used a function that the overview script exported. A change to the overview changed the slides. Now both use `EMC.sliceTimeline` and the shared `.timeline` styles.
- Two regular expressions defined the route grammar: one for load-time scoping and one for navigation. A new route needed two changes. Now `parseHash` is the only grammar.

Chapter scoping is set when the page loads, before every other DP. When the reader opens another chapter, navigation reloads the page at the new fragment. This reload is the control junction that keeps the decoupled order. A change of view inside the same chapter does not reload the page.

Some residual shared items are intentional. Each board places hotspots by its own layout rules, but DP5.1 decides which hotspots are in scope. Each view writes its own entry in the shared `LEGENDS` object. The drawer and the popovers both close on Escape.

## Module contracts

| Module | Owns | Input | Output and failure contract |
| --- | --- | --- | --- |
| `internal/syntax` | HCL body schemas and parsing | One or more files, each with a filename and source bytes | Parsed document or HCL syntax diagnostics. Each diagnostic names its own file. |
| `internal/source` | Shared source catalogs and reference normalization | Parsed document | Decoded source facts that do not change. No I/O and no policy. |
| `internal/validator` | Structural, reference, scenario, and smell policy | Decoded source and a profile | Diagnostics. Without errors, also a `ValidatedDocument`. |
| `internal/model` | Canonical model for renderers and interchange, with normalized edges | `ValidatedDocument` | The same `Model` for the same input. No parsing, no validation, no I/O. |
| `internal/formatter` | Canonical HCL layout | Source bytes | Formatted bytes or parse diagnostics |
| `internal/renderer` | Standalone HTML and the viewer scripts | Canonical `Model` | HTML, or a template or serialization error. The viewer obeys the matrix for the diagram viewer. |
| `internal/interchange` | Schema check and conversion for slice-based JSON | JSON bytes, or a canonical `Model` | A document or unformatted HCL with warnings. A returned error means that the input has no correct conversion. No I/O. |
| `internal/app` | Use-case sequence and plain diagnostics | Source in memory, or a model path (one file or one folder) | Results for validate, format, render, import, and export that all adapters share |
| CLI, WASM, `internal/serve` | Actions for each runtime | Arguments, files, signals, HTTP, JavaScript values | Exit codes, files, browser values, and server lifecycle |

The loader (DP8) is `app.ReadModel` with `syntax.ParseFiles` and `syntax.Document.SourceOf`. Its contract has four rules:

- R1: A path to a file is a one-file model. A path to a folder is a folder model.
- R2: A folder model uses every regular file directly in the folder whose name ends in `.em.hcl` and does not start with a dot. Symlinks to files count. Subfolders and other files are ignored.
- R3: `app.ReadModel` sorts the member files by name, byte by byte, and not by locale. Model order is file order, then source order inside each file.
- R4: A folder with no member file gives error EM001 with the detail `folder <path> has no .em.hcl files`.

`syntax.ParseFiles` takes the files in that order and parses each one on its own. If any file has a syntax error, it returns no document and all the syntax diagnostics. `SourceOf` returns the bytes of the file whose name equals the filename of an HCL range, so each diagnostic and each source excerpt names its own file. A folder with one member file behaves like that file.

OS actions stay at the boundary. The server injects file, listener, browser, stream, and signal actions. Thus tests can examine lifecycle behavior without a change to language calculations.

The CLI owns output-file safety. `emhcl export` refuses an `-o` target that has the `.em.hcl` extension or that is the source file. It exits with code 2. When a decode or a conversion fails, `emhcl import` does not change an existing output file.

## Change rules

1. Change the language grammar in `internal/syntax` and in the normative specification first. Then update source decoding and validator rules, in that order.
2. Add a shared interpretation to `internal/source`. Do not rebuild catalogs, inferred addresses, or traversals in validators or renderers.
3. Keep validation policy in `internal/validator`. A model or a renderer must not correct invalid input without a diagnostic.
4. Keep the canonical model independent of presentation. Layout for a renderer belongs in `internal/renderer`.
5. Add a use case one time, in `internal/app`. Runtime adapters translate inputs and outputs. They do not build the pipeline again.
6. Send new OS effects through an adapter or the injected server environment. Define cancellation and failure behavior for each worker.
7. Keep the JSON schema rules in `conform.go`. Import and export must both use them.
8. Decode the original JSON bytes. Do not encode an accepted document a second time, because that changes key order.
9. Write JSON values with `jsonTokens`. Do not send object keys through `cty` on import or on export.
10. If a conversion has no correct native form, return an error. If it drops data, write a warning at the place where it drops the data.
11. If you change a flow rule in `internal/validator`, change `import_policy.go` in the same change.
12. After each change to the interchange mapping, make sure that the round trip stays stable. Do this for each file in `examples/` and `testdata/valid/`.
13. Renaming a member file keeps the order of chaptered workflows, because chapters set that order. It can move unchaptered workflows, because they follow file name order. It can also change which duplicate declaration EM002 reports as the first one.
14. Add a route only in `parseHash`. Load-time scoping and navigation read the same grammar.
15. A board must not render or call another board. A new board registers its filter function with `EMC.onFilterChange` and its layout function with a `relayout` hook.
16. Put a slice presentation that two views share in `viewer.js` under `EMC`. Do not export it from one view to another.
17. Change the fragment with `location.hash` or `replaceHash`. Do not let the browser follow a fragment link, because inside the `srcdoc` frame of the playground the link loads the playground page.

A change breaks the Independence Axiom when, for example:

- A renderer starts to decide validity.
- A new frontend builds its own diagnostics.
- Model construction accepts HCL that the validator did not accept.
- A grammar fact gets two owners.
- The importer writes a model that the validator did not accept.
- A viewer page renders another board to get filters, data, or a helper.
- A viewer route gets a second parser.

## Information Axiom and evidence

The repository does not give a number for information content. It does not measure the probability that each FR succeeds in production. The design reduces known failure modes with these mechanisms:

- Dependencies that go in one direction.
- A sequence that the types enforce.
- Stable diagnostics.
- Injected effects.
- Automated tests.

`make verify` runs a repeatable set of steps. They cover formatting, module tidiness, `go vet`, unit tests, the race detector, static analysis, vulnerabilities, the WASM build, and the examples.

The interchange module has this evidence:

- Regression tests for each defect from the review of the `json-cmd` branch. They are in `review_regressions_test.go` and `export_value_regressions_test.go` in `internal/interchange`, and in `internal/cli/interchange_test.go`.
- `TestExportImport_RoundTripsEveryShippedExample`, which compares the first and the second export of each example.
- Import, `ajv`, and round-trip runs on the five fixtures in `testdata/valid/`. The source files and the exported files pass `ajv`, and the second export is identical to the first.

Some limits are known. String values (not keys) pass through `cty` on both import and export. Thus Unicode normalization can change the text of a string value. Two read models in the Cart fixtures have different `aggregate` metadata. No evidence shows which value is correct.

The diagram viewer has less evidence. The Go tests check that the HTML contains the viewer code and the model JSON. No automated test runs the viewer in a browser. The matrix for the viewer comes from a manual browser check: a Storming page that opened directly rendered no Model board and applied the context filter. The route checks covered valid, unknown, and malformed fragments. A browser test of the routes, the filters, and the slides would turn these checks into repeatable evidence.

To compare this architecture with an alternative under the Information Axiom, measure release history, escaped defect counts, flaky-test rates, and change lead time.
