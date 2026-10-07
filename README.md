# Event Modeling HCL Specification

[![CI](https://github.com/event-modeling-hcl/eventmodeling-hcl/actions/workflows/ci.yml/badge.svg)](https://github.com/event-modeling-hcl/eventmodeling-hcl/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/event-modeling-hcl/eventmodeling-hcl.svg)](https://pkg.go.dev/github.com/event-modeling-hcl/eventmodeling-hcl)
[![License](https://img.shields.io/badge/license-Apache--2.0-blue.svg)](LICENSE)

`emhcl` v0.8.0 implements Event Modeling HCL Specification v0.3.0. The current
source also loads folder models from Specification v0.4.0 (RFC 0002). No
release has folder models yet. The Unreleased section of
[CHANGELOG.md](CHANGELOG.md) lists them.
This repository provides a strict validator, canonical formatter, typed semantic model,
normative language documentation, and executable examples. The domain reference is the upstream [Event Modeling
Specification](https://github.com/dilgerma/event-modeling-spec); the HCL
specification and validator in this repository define its native port. The
native language is not a JSON embedding. The `import` and `export` commands
convert between native models and slice-based Event Modeling JSON.

The language is designed for hand authoring: bounded contexts own canonical
events, reusable field types, and aggregates; workflow kind is the top-level
block keyword; typed HCL traversals express canonical relationships; and
derivable bookkeeping is not written. Native scenarios and workshop blocks
preserve the rules and durable artifacts of an Event Modeling session.

## Requirements and Installation

Install the tagged command with Go 1.25 or newer:

```bash
go install github.com/event-modeling-hcl/eventmodeling-hcl/cmd/emhcl@v0.8.0
```

Go writes the executable to `GOBIN`, or to `$(go env GOPATH)/bin` when
`GOBIN` is unset. Add that directory to `PATH`, then verify the installation:

```bash
export PATH="$(go env GOPATH)/bin:$PATH"
emhcl version
```

To build from this checkout instead:

```bash
go build -o bin/emhcl ./cmd/emhcl
```

Published release archives for Linux, macOS, and Windows are available from
the [GitHub Releases page](https://github.com/event-modeling-hcl/eventmodeling-hcl/releases).
Verify a downloaded archive with that release's `checksums.txt`:

```bash
archive=emhcl_0.8.0_linux_amd64.tar.gz
grep " ${archive}$" checksums.txt | sha256sum -c -
# macOS: grep " ${archive}$" checksums.txt | shasum -a 256 -c -
```

Stable release archives and `checksums.txt` also have GitHub build provenance.
With GitHub CLI 2.49.0 or newer, verify the downloaded archive was produced by
this repository's release workflow:

```bash
gh attestation verify emhcl_0.8.0_linux_amd64.tar.gz \
  --repo event-modeling-hcl/eventmodeling-hcl
```

Validate one complete model per invocation:

```bash
emhcl validate examples/minimal.em.hcl
emhcl validate examples/complete.em.hcl
emhcl validate examples/pet-management-detailed.em.hcl
emhcl validate --profile strict examples/complete.em.hcl
emhcl fmt -w examples/complete.em.hcl
```

A valid document prints `<path> valid`. Diagnostics use the form
`file:line:column: Severity EMxxx: message`. The default `valid` profile keeps
modeling judgment as warnings; `workshop` reports it as information and `strict`
escalates unreasoned commands and open hotspots to errors.

Model files must use the `.em.hcl` extension. HCL is the language; the suffix
identifies a complete Event Modeling document to this validator. Check the
installed binary version with `emhcl version`.

Render a valid model as a self-contained, interactive HTML canvas with four
switchable views and a shared detail drawer for each slice's scenarios:

```bash
emhcl diagram examples/complete.em.hcl -o complete.html
emhcl diagram examples/complete.em.hcl > complete.html
```

The `diagram` command validates with the default `valid` profile before
rendering. Errors prevent output; modeling warnings are reported without
blocking the diagram. The generated file embeds its CSS, JavaScript, and model
data; nothing loads over the network except a user-supplied `screen_image`
URL, if present.

- **Model** — content-adaptive slices, left-to-right flow stages, dedicated
  screen, processor, model, and event swimlanes, and typed flow arrows. Domain
  events use Event Storming orange; events owned by external bounded contexts
  use pink. Each actor card renders directly beside its associated screen
  card, and flow arrows that point backward in the left-to-right layout
  (their target sits left of their source) use a dashed stroke, turning solid
  only while hovered.
- **Storming** — the same model laid out as tactical Event Storming: one row
  per workflow with sticky notes for screens, commands, aggregates, events,
  automations/translations, and read models, plus an actor card beside every
  screen that has one. A workflow connected to another by an edge starts to
  the right of the workflow it depends on; independent workflows run in
  parallel rows.
- **Context Map** *(experimental)* — a bounded-context map derived from
  cross-context event consumption: one node per bounded context, with
  upstream → downstream edges labeled customer/supplier, or anti-corruption
  when the consumer is a `translation` workflow. The derivation is heuristic
  and both the relationship labels and the layout may change in a future
  release.
- **Compact** — the same slices and cards as Model, with one events row instead of a lane per aggregate. Within a slice, a bounded context's box always sits to the right of the command that emits its events and to the left of the read model its events feed; an automation or translation slice therefore shows the upstream (given) event's box first, left of its read model, then the box for the event the command produces. Each bounded context is a dashed, rounded rectangle with a translucent colour and its title. Colours are assigned in context order (colour 1 to the first non-external context, colour 2 to the next); they are not part of the HCL spec. External bounded contexts are always pink. Arrows go from commands to events (solid) and from events to read models, exactly as in Model. The event's aggregate (cube icon) is a sticky stuck on top of the event with no arrows of its own; an external event carries a pink external-system sticky (globe icon) in its place.

For a local edit-and-render loop, serve one model and keep the browser open
while the file changes:

```bash
emhcl serve examples/minimal.em.hcl
```

`serve` binds to `127.0.0.1:8080` by default. Use `--port 0` to request an
available port; the actual browser URL is printed after binding.

## Multi-file models

Folder models (Specification v0.4.0, RFC 0002) are in the current source and
are not in a release yet. The Unreleased section of
[CHANGELOG.md](CHANGELOG.md) lists them. The v0.8.0 release loads one file
only.

A model can be one `.em.hcl` file or a folder of `.em.hcl` files. A folder
model is one model that you write in several files. This follows Specification
v0.4.0 (RFC 0002). Use a folder when one file becomes too large to read, or
when different people own different bounded contexts or slices.

### Split a file into a folder

1. Make a new folder for the model.
2. Move the catalog blocks (`bounded_context`, `actor`, `team`, `system`) into one file, for example `00-catalog.em.hcl`.
3. Move every `chapter` block into one file, for example `01-chapters.em.hcl`.
4. Move each workflow block into its own file, for example `10-register-pet.em.hcl`.
5. Move the other blocks, such as `hotspot`, into any file, for example `90-hotspots.em.hcl`.
6. Move each block whole. Do not split one block over two files.
7. Add each workflow to a chapter. A workflow that is in no chapter gives a warning.
8. Run `emhcl validate <folder>` and fix each error that it prints.

The folder `examples/pet-clinic` is `examples/complete.em.hcl` split this way:

```text
examples/pet-clinic/
  00-catalog.em.hcl
  01-chapters.em.hcl
  10-register-pet.em.hcl
  11-pet-directory.em.hcl
  12-notify-owner.em.hcl
  13-import-partner-pet.em.hcl
  90-hotspots.em.hcl
```

The number prefix is not part of the language. It only keeps the files in
reading order, because the tool sorts files by name.

### How the files join

Every `.em.hcl` file directly in the folder is a member of the model. The tool
ignores subfolders, other files, and files with a name that starts with a dot.
It sorts the member files by name, byte by byte. Model order is the order of
the blocks after this sort: file order first, then source order in each file.

The model contains all top-level blocks of all member files. A reference in
one file can point to a block in any other member file. IDs are global across
files, within the same ID space as in a one-file model. Each block kind has its
own ID space: actors, teams, systems, bounded contexts, chapters, and hotspots
each have a separate space, and all workflow kinds share one space. The same ID
twice in one space is error EM002, in one file or in two. The detail of that
error gives the location of the first declaration as `file:line:column`.

### Board order and chapters

A chapter is a named group of workflows. In a folder model, the chapters set
the order of the workflows on the board. The tool places the workflows in
chapter order, and in list order in each chapter. All `chapter` blocks must be
in one file. Chapters in two or more files give error EM013. A workflow in two
chapters gives error EM014.

A workflow that is in no chapter gives warning EM407. The tool places it after
all chaptered workflows, in model order. Thus, a new file name can move it on
the board. The `strict` profile makes EM407 an error, and the `workshop`
profile makes it information only.

### Commands

The `validate`, `diagram`, `serve`, and `export` commands accept a file or a
folder. Each diagnostic gives the member file that it points to. The `serve`
command reloads the page when you add, remove, rename, or edit a member file.
If no member file is left, the page shows error EM001 until a member file
returns. The `fmt` command formats one file at a time and refuses a folder. The
`import` command writes one file.

```bash
emhcl validate examples/pet-clinic
emhcl serve examples/pet-clinic
```

A folder with exactly one member file gives the same result as that file. A
folder with no member file gives error EM001.

## Import and export JSON

Convert slice-based Event Modeling JSON to and from native HCL:

```bash
emhcl import testdata/valid/axoniq-vsa-sample-news.json -o news.em.hcl
emhcl export news.em.hcl -o news.json
```

Import validates input against the
[published JSON schema](https://github.com/dilgerma/event-modeling-spec/blob/main/eventmodeling.schema.json),
and export checks its output against the same rules before writing it.
Schema conformance is separate from semantic convertibility: a schema-valid
slice with more than one actor is a hard import error, even if it has no
screens. The document root contains only `slices`; actor declarations and
aggregate titles belong to individual slices. Unknown properties, missing
required properties (such as `sliceType`, element `fields`/`dependencies` or
specification `linkedId`), values outside an enum, mistyped values and trailing
content are import errors that name the offending JSON path.

The native language remains Specification **v0.3.0**. Actors are declared at
the top level and referenced by screens; workflows have no actor attribute
or actor inheritance.

`import` writes formatted `.em.hcl` and validates it. Without `-o`, it writes
to stdout and labels diagnostics `<stdout>`. A schema-valid document with no
slices imports as an empty model and exits 0. Qualified IDs retain native labels; foreign IDs and field names
map to valid, collision-safe HCL labels. **Original canvas IDs and field-name
spellings are not retained.** Distinct event IDs remain distinct, even when
their titles match. Event copies resolve through `linkedId`; workflow-local
copies become local snapshots because HCL has no copy-lineage construct.
An external context whose name an internal context also uses imports with an
`_external` label suffix, so internal and external events stay in separate
native contexts. Events without a context never fall into an external context,
and a canonical event ID such as `event.billing.paid` shares its context with
foreign-ID events that name the same context. Events without an `id` keep
their own flows; an empty `linkedId` never binds to another event.

The declared `sliceType` is authoritative. Unrepresentable children and
dependencies are omitted with warnings. A cross-slice
`state_view` read model → processor flow, for example, has no legal canonical
HCL spelling without changing the workflow structure. Missing targets are
not invented. Specifications retain typed targets, repeated step instances,
tags, fields, object examples and comments; step `index` determines order.
Unknown/incorrect targets are warned about, and a specification that cannot
form a legal native scenario is omitted with a warning.

Prototype objects and object examples keep their complete JSON values,
including nested data, code strings, empty objects and nested `null`;
conversion does not execute prototype code. Object keys keep their source
order and exact text. Keys that are HCL keywords (`for`, `in`, `if`, `true`,
`false`, `null`) or not identifiers (`a-b`) are written quoted. Keys that
differ only by Unicode normalization (`é` and `e\u0301`) stay distinct.
Numeric/boolean/list field examples encoded as JSON strings become native
values. A singleton object sample for a `Custom`/`List` field becomes a
one-item list, with a warning. Incompatible typed samples are omitted with
warnings, not converted into invented values. JSON-text `null` examples
decode to native `null` for non-textual or `List` fields.

`export` validates HCL and writes one slice per workflow, without document-level
catalogs. It refuses an `-o` target with the `.em.hcl` extension or one that
is the source file (exit 2), so it cannot overwrite the model.
Actors are exported only in slices where screens reference them;
aggregate references are exported as titles. IDs are qualified native
references, such as `command.register_pet.register_pet_command`. A bounded
context is exported by its title when importing that title restores its
label, otherwise by its label. A workflow owned by an external context is
exported with that context's label, because a slice `context` cannot say
"external"; the flag returns on import only through the context's `EXTERNAL`
events, and export warns when there are none. Dependencies come from canonical
flows and keep the author order of each `to`/`from` list. Export-import-export
stability is covered for tested, representable inputs; it is not a guarantee
of equality with arbitrary original tool JSON. Conversion normalizes IDs,
field names, reusable types and example encodings, and discloses losses
through warnings. Reusable field types are flattened; explicit field
overrides, including `false`, empty strings and `null`, win. A field type that
contains itself is expanded one level, with a warning.
The schema permits only string or object field examples. Other values,
including numbers, booleans, lists and `null`, are carried as JSON text strings
and decoded on import for non-textual or `List` fields.

Conversion is **not a lossless canvas archive**. Warnings disclose omitted
storylines, assignments/tickets, actor roles/tags and specification
attachment/layout bookkeeping. Properties outside the schema are rejected
rather than omitted.
The JSON actor list cannot associate different actors with individual screens.
Import maps a single slice actor to its screens. Export supports per-screen
references to a single actor only; more than one distinct actor in a workflow
is a hard export error, not a choice of the first actor or a warning-only loss.
If only some screens reference the sole actor, export omits slice associations
with a warning instead of assigning that actor to previously unassigned screens.
Chapters, hotspots, teams/systems, aggregate catalog identities, unused actor
declarations, context/aggregate/actor descriptions, non-context workflow
owners, read-model questions, `external_trigger`, error-step titles that differ
from the error text, explicit context titles equal to their label, bounded
contexts no slice exposes and events no workflow references also have no
equivalent and produce warnings. An event is exported as a copy only when
another workflow produces it; an event nothing produces stays an original.
A read model's question is derived from its description or title on import.

Warnings do not change the exit code. If native validation still reports
errors, `import` writes the generated source for repair and exits with code 1;
decode or semantic conversion failures leave an existing output file unchanged.
The schema-conformant Martin Dilger fixtures in `testdata/valid/*.json`
exercise interchange shapes; they are not examples of every native HCL
language rule.

## Authoring

Top-level `bounded_context` blocks define domain contracts. Top-level
`state_change`, `state_view`, `automation`, and `translation` blocks define the
four workflow patterns. Labels carry identity and use lower snake_case. Events
are declared only inside bounded contexts and participate in workflows through
typed flow or scenario references.

```hcl
bounded_context "pet_management" {
  title = "Pet Management"

  aggregate "pet" {}

  field_type "pet_id" {
    type         = "Int"
    id_attribute = true
  }

  event "pet_added" {
    title     = "Pet Added"
    aggregate = aggregate.pet

    field "pet_id" { type = field_type.pet_id }
  }
}

actor "clinic_staff" {
  title         = "Clinic staff"
  auth_required = true
}

state_change "add_pet" {
  title = "Add Pet"

  screen "add_pet_form" {
    title = "Add pet form"
    actor = actor.clinic_staff
    to    = [command.add_pet]
  }

  command "add_pet" {
    title     = "Add Pet"
    aggregate = aggregate.pet_management.pet
    to        = [event.pet_management.pet_added]

    field "pet_id" { type = field_type.pet_management.pet_id }
  }
}
```

A `field` whose name matches a `field_type` may drop the `type`
(`field "pet_id" {}`), and `fields = [field_type.pet_management.pet_id]` adds
several typed fields at once. Screens and other workflow elements carry fields
the same way events do.

Source position is model order. A flow edge has one canonical spelling: the
source element uses `to`, except catalog events flow through the receiver's
`from`. References are unquoted traversals and are checked for scope, kind, and
existence. Ordinary values are native HCL literals; variables, functions, and
interpolation are rejected.

Read models state the question they answer, and scenarios use typed targets:

```hcl
state_view "pet_directory" {
  readmodel "pets" {
    question = "Which pets are registered?"
    from     = [event.pet_management.pet_added]
  }

  scenario "pets_are_listed" {
    given { event = event.pet_management.pet_added }
    then  { readmodel = readmodel.pets }
  }
}
```

Use native values for field examples:

```hcl
field "pets" {
  type        = "Custom"
  cardinality = "List"
  example = [{
    id   = 5
    name = "Mochi"
  }]
}
```

## Documentation and Examples

- [Authoritative language specification](https://github.com/event-modeling-hcl/spec):
  normative grammar, references, canonical flow, scenarios, and compatibility.
- [Specification examples](https://github.com/event-modeling-hcl/spec/tree/main/examples):
  complete, independently valid examples for each Event Modeling pattern.
- [Learning guide](https://github.com/event-modeling-hcl/spec/blob/main/guides/learning-event-modeling-hcl.md):
  practice-first Event Modeling and `.em.hcl` instruction.
- [Minimal example](examples/minimal.em.hcl): smallest useful model.
- [Complete reference](examples/complete.em.hcl): every supported HCL construct.
- [Pet-management port](examples/pet-management-detailed.em.hcl): the supplied
  five-workflow golden model in native HCL.
- [Language decisions and migration guidance](https://github.com/event-modeling-hcl/spec):
  authoritative ADRs, migration material, examples, and RFCs.
- [Architecture guide](architecture-guide.md): package responsibilities,
  pipeline contracts, and the repository's Axiomatic Design rationale.

## Verification

Run the complete local verification suite before submitting a change:

```bash
make verify
```

`make help` lists individual commands. The most commonly used are `make build`,
`make test`, and `make validate-examples`.

The test suite validates every `.em.hcl` file in `examples/` and asserts every
fixture in `testdata/invalid/` fails validation.

### Pre-commit Hooks

Install [pre-commit](https://pre-commit.com/), then install the repository hook
after cloning:

```bash
make pre-commit-install
```

The hook checks common repository hygiene, formats changed Go files, checks
module consistency when `go.mod` or `go.sum` changes, and runs `go vet ./...`
and `go test ./...`. Run the same hooks against the whole checkout with:

```bash
make pre-commit-run
```

`make verify` remains the full local quality gate; it also runs race, static,
exhaustiveness, vulnerability, example-validation, and release checks.

## Boundaries

The supported HCL Specification v0.3.0 resolves references within a single
document and enforces scoped identity, canonical typed flows, scenario shape,
field examples, and workflow patterns. Folder models from Specification v0.4.0
resolve references across all files of one folder, as the
[Multi-file models](#multi-file-models) section describes. `internal/app`
exposes the shared application operations, and `internal/model.Build`
constructs a typed IR only from a validator-issued `ValidatedDocument`. The
tool does not support event groups, include or import blocks, subfolders as
part of a model, or editor integration. JSON conversion is a warned
projection, not canvas persistence; the Context Map view is an experimental
rendering heuristic.

## License

Licensed under the [Apache License, Version 2.0](LICENSE).
