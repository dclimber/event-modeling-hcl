# Architecture Guide

This guide describes how `emhcl` is organized and how its design
uses Axiomatic Design to keep changes predictable. It is descriptive of the
current repository. The language contract itself remains in the
[Event Modeling HCL specification](https://github.com/event-modeling-hcl/spec).

## Axiomatic Design in Brief

Axiomatic Design separates what a system must achieve from the mechanisms used
to achieve it:

- A **customer need** is an outcome expected by a user or maintainer.
- A **functional requirement (FR)** is a testable, solution-independent outcome.
- A **design parameter (DP)** is the mechanism selected to satisfy an FR.
- The **Independence Axiom** asks that FRs remain independently controllable. An
  uncoupled design has a diagonal FR-DP influence matrix. A decoupled design has
  a triangular matrix and is valid when its required sequence is preserved.
- The **Information Axiom** prefers, among designs that satisfy the Independence
  Axiom, the design with the highest probability of meeting all requirements.
  Information content is `I = -log2(p)`. Source lines or package counts are not
  substitutes for measured success probability.

The tool is intentionally a decoupled pipeline. Later stages depend on facts
established by earlier stages, while later-stage choices cannot change an
earlier-stage result.

## Needs, Requirements, and Parameters

| Need | Functional requirement | Acceptance criterion | Design parameter |
| --- | --- | --- | --- |
| N1: Authors receive trustworthy feedback | FR1: Parse native `.em.hcl` syntax | Syntax errors retain HCL source locations | DP1: `internal/syntax` grammar and parser |
| N1 | FR2: Decode shared source facts once | Catalog and reference interpretation has one owner | DP2: `internal/source` decoded document |
| N1 | FR3: Enforce language and modeling rules | Invalid models produce stable diagnostics for the selected profile | DP3: `internal/validator` semantic passes |
| N2: Every frontend sees the same model | FR4: Construct one canonical IR | Only validated input can reach model construction | DP4: `internal/model` lowering |
| N3: Models can be consumed in useful forms | FR5: Format or render deterministically | Formatting and HTML generation contain no OS actions | DP5: `internal/formatter` and `internal/renderer` |
| N4: CLI, browser, and server behavior agree | FR6: Compose and expose the use cases | Entry points delegate to one application service | DP6: `internal/app` plus thin runtime adapters |

## Independence Matrix

`X` means that changing the DP materially influences the FR. `0` means there is
no material influence within the supported single-document operating range.

| FR \ DP | DP1 Syntax | DP2 Source | DP3 Validation | DP4 Model | DP5 Output | DP6 Adapters |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| FR1 Parse syntax | X | 0 | 0 | 0 | 0 | 0 |
| FR2 Decode source facts | X | X | 0 | 0 | 0 | 0 |
| FR3 Enforce validity | X | X | X | 0 | 0 | 0 |
| FR4 Construct canonical IR | X | X | X | X | 0 | 0 |
| FR5 Produce deterministic output | X | X | X | X | X | 0 |
| FR6 Expose consistent operations | X | X | X | X | X | X |

This lower-triangular matrix is **decoupled**. Its control sequence is:

```text
syntax.Parse
  -> source.Decode
  -> validator.ValidateDecodedDocument
  -> model.Build
  -> renderer.Render
```

The sequence is enforced by `validator.ValidatedDocument`: `model.Build` cannot
accept merely parsed input. Formatting is a deliberate side branch because it
needs syntactically parseable HCL but must not require a semantically valid
model.

The important zeros are directional. Renderer or CLI changes cannot alter what
syntax is accepted; model or renderer changes cannot make invalid input valid;
and adapters cannot define a second diagnostic or rendering policy.

## Module Contracts

| Module | Owns | Input | Output and failure contract |
| --- | --- | --- | --- |
| `internal/syntax` | HCL body schemas and parsing | Filename and source bytes | Parsed document or HCL syntax diagnostics |
| `internal/source` | Shared source catalogs and reference normalization | Parsed document | Immutable decoded source facts; no I/O or policy |
| `internal/validator` | Structural, reference, scenario, and smell policy | Decoded source plus profile | Diagnostics and, only without errors, `ValidatedDocument` |
| `internal/model` | Canonical renderer-facing IR and normalized edges | `ValidatedDocument` | Deterministic `Model`; no parsing, validation, or I/O |
| `internal/formatter` | Canonical HCL layout | Source bytes | Formatted bytes or parse diagnostics |
| `internal/renderer` | Standalone HTML representation | Canonical `Model` | HTML or template/serialization error |
| `internal/app` | Use-case sequencing and plain diagnostics | In-memory source or a one-shot file path | Validate, format, and render results shared by all adapters |
| CLI, WASM, `internal/serve` | Runtime-specific actions | Arguments, files, signals, HTTP, JavaScript values | Exit codes, files, browser values, and server lifecycle |

OS interactions remain at the boundary. The server injects file, listener,
browser, stream, and signal actions so lifecycle behavior can be tested without
changing language calculations.

## Change Rules

1. Change the language grammar in `internal/syntax` and the normative spec
   first; update source decoding and validator rules in that order.
2. Add a shared interpretation to `internal/source`. Do not independently
   reconstruct catalogs, inferred addresses, or traversals in validators or
   renderers.
3. Keep validation policy in `internal/validator`. A model or renderer must not
   silently compensate for invalid input.
4. Keep the canonical IR presentation-independent. Renderer-specific layout
   belongs in `internal/renderer`.
5. Add a use case once in `internal/app`; runtime adapters translate inputs and
   outputs but do not reassemble the pipeline.
6. Route new OS effects through an adapter or injected server environment and
   define cancellation and failure behavior for every worker.

A change violates the Independence Axiom when, for example, a renderer starts
deciding validity, a new frontend reconstructs diagnostics, model construction
accepts unvalidated HCL, or a grammar fact acquires multiple owners.

## Information Axiom and Evidence

The repository does not claim a numerical information-content score because it
does not yet measure the probability of satisfying each FR in production. The
design reduces credible failure modes through one-way dependencies, capability-
based sequencing, stable diagnostics, injected effects, and automated checks.

`make verify` supplies repeatable evidence through formatting, module-tidiness,
vet, unit, race, static-analysis, vulnerability, WASM-build, and example checks.
Release history, escaped defect counts, flaky-test rates, and change lead time
would be appropriate measurements for comparing this architecture with a
future alternative under the Information Axiom.
