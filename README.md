# Event Modeling HCL Specification

[![CI](https://github.com/event-modeling-hcl/eventmodeling-hcl/actions/workflows/ci.yml/badge.svg)](https://github.com/event-modeling-hcl/eventmodeling-hcl/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/event-modeling-hcl/eventmodeling-hcl.svg)](https://pkg.go.dev/github.com/event-modeling-hcl/eventmodeling-hcl)
[![License](https://img.shields.io/badge/license-Apache--2.0-blue.svg)](LICENSE)

`emhcl` v0.7.2 implements Event Modeling HCL Specification v0.3.0.
This repository provides a strict validator, canonical formatter, typed semantic model,
normative language documentation, and executable examples. The domain reference is the upstream [Event Modeling
Specification](https://github.com/dilgerma/event-modeling-spec); the HCL
specification and validator in this repository define its native port. The
format is not a JSON embedding or conversion format.

The language is designed for hand authoring: bounded contexts own canonical
events, reusable field types, and aggregates; workflow kind is the top-level
block keyword; typed HCL traversals express canonical relationships; and
derivable bookkeeping is not written. Native scenarios and workshop blocks
preserve the rules and durable artifacts of an Event Modeling session.

## Requirements and Installation

Install the tagged command with Go 1.25 or newer:

```bash
go install github.com/event-modeling-hcl/eventmodeling-hcl/cmd/emhcl@v0.7.2
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
archive=emhcl_0.7.2_linux_amd64.tar.gz
grep " ${archive}$" checksums.txt | sha256sum -c -
# macOS: grep " ${archive}$" checksums.txt | shasum -a 256 -c -
```

Stable release archives and `checksums.txt` also have GitHub build provenance.
With GitHub CLI 2.49.0 or newer, verify the downloaded archive was produced by
this repository's release workflow:

```bash
gh attestation verify emhcl_0.7.2_linux_amd64.tar.gz \
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
field examples, and workflow patterns. `internal/app` exposes the shared
application operations, and `internal/model.Build` constructs a typed IR only
from a validator-issued `ValidatedDocument`. The tool does not support event
groups, context maps, multi-file loading, cross-file reference resolution,
JSON conversion, or editor integration.

## License

Licensed under the [Apache License, Version 2.0](LICENSE).
