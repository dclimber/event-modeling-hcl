---
name: event-modeling-hcl
description: >-
  Turn product ideas, requirements, workshop notes, user stories, domain prose,
  existing behavior, or event-driven designs into valid Event Modeling HCL
  v0.4.0 models: one `.em.hcl` file or a folder of `.em.hcl` files. Use this
  skill whenever the user asks to create, extend, split, repair, review,
  validate, format, import, export, or explain an Event Model or `.em.hcl`
  file, or mentions Events, Commands, Read Models, State Change, State View,
  Automation, Translation, Event Modeling slices, information completeness, or
  Given/When/Then scenarios—even if they do not name HCL.
compatibility: >-
  Targets Event Modeling HCL Specification v0.4.0. For machine verification,
  use emhcl v0.9.0 or newer. emhcl v0.8.0 and older cannot load a folder model.
metadata:
  version: "1.1.0"
  author: workspace
  specification: "https://github.com/event-modeling-hcl/spec"
---

# Event Modeling HCL

Create one coherent, hand-authored Event Modeling HCL model from the user's
domain input. The model is one `.em.hcl` file or one folder of `.em.hcl`
files. Preserve Event Modeling's business-first discovery process: model facts
and information flow before implementation structure, then encode the result in
the strict native HCL language.

## Load only the references needed

- Read [references/methodology.md](references/methodology.md) when discovering a
  new model, interpreting prose, choosing slice boundaries, reviewing model
  quality, honoring status locks, or recording evidence provenance.
- Read [references/language-v0.4.md](references/language-v0.4.md) before writing
  or repairing HCL, splitting a model into a folder, or converting JSON. It is
  the local syntax and validation reference.
- Read [references/patterns.md](references/patterns.md) when a pattern shape or
  scenario form is uncertain. Adapt examples to the domain; never copy their
  placeholder vocabulary into the output.

For conflicts, the normative Event Modeling HCL v0.4.0 specification wins over
examples, existing files, or general HCL knowledge.

## Rules that are not syntax

Apply three layers, in this order when they conflict:

1. Validator and v0.4.0 grammar. Illegal HCL is never "more faithful".
2. Methodology judgment in `references/methodology.md`. A valid file can still
   be a poor model.
3. Canvas layout (columns, lanes, stickies). It has no attributes. Encode the
   intent with existing blocks, or record the gap.

Do not invent `linked_copy`, `element_copy`, `storyline`, `query`, `note`, or
a lineage DSL (`mapping = "session:…"`, `latest:`, `derived:`, `aggregate:`).
The current CLI accepts an unchecked `mapping` string; do not add one.

## Deliverable

Produce or update the requested `.em.hcl` file or model folder. A model is not
complete merely because a diagram or prose explanation looks plausible.

Unless the user asks for a workshop-only draft, the delivered model must:

1. encode all supplied business behavior and every named acceptance criterion;
2. use one workflow per pattern-sized capability. Never combine an independent state change and state view into one workflow. An automation or translation includes its own todo-list read model and command;
3. include precise fields where the input supplies them;
4. include scenarios for supplied business rules and important success/error
   paths;
5. pass formatting and default validation when the CLI is available;
6. pass strict validation before being called implementation-ready;
7. retain genuine unknowns as attached `hotspot` blocks instead of inventing
   facts.

Strict validation treats an open hotspot as an error. Keep the hotspot on a
workshop draft. Do not invent the missing rule to make strict pass, and do not
call the model implementation-ready while a material hotspot is open.

Choose the model shape:

- Keep an existing model in its current shape. Do not split a file or merge a
  folder unless the user asks.
- Write a new model as one file, unless the user asks for a folder or the
  model has many workflows or several owners.
- In a folder model, put catalog blocks in one file, all `chapter` blocks in
  one file, and each workflow in its own file. Name files with a number prefix
  so that the file name order is the reading order. Put every workflow in a
  chapter. See "Folder models" in `references/language-v0.4.md`.

Do not create implementation code, database schemas, brokers, APIs, retries, or
framework architecture unless the user explicitly asks. `api_endpoint` may
record a supplied API trigger, but implementation details do not drive the
model.

## Workflow

### 1. Ground in available evidence

Read the user's prose and relevant project requirements, tests, domain types,
existing `.em.hcl` files, and examples. Reuse the repository's vocabulary.
Separate:

- explicit facts and rules;
- reasonable naming or ownership deductions;
- unresolved business questions.

Do not ask for information already present in files. If a missing decision
would materially change the model, either ask a focused question or—when useful
work can continue—record an open hotspot and proceed. Do not guess silently.
Label a non-obvious deduction in `description` as stated, implied, or assumed,
and name the source (requirement, test, type, or UI). Code and screens are
hypotheses; an explicit stakeholder rule outranks them. Keep document
thresholds and dates verbatim.

When extending an existing model, preserve its valid identity vocabulary and
business-time story. In a folder model, IDs are global across files, so search
every member file before you add an ID. Add a new workflow to the chapter where
it belongs. Migrate every affected reference; do not leave obsolete aliases or
duplicate contracts.

When the input is Event Modeling JSON, run `emhcl import` to get a first draft.
Then review it with this skill as you would review any other model. Read each
import warning. Import does not keep canvas IDs, and it does not invent missing
targets.

Honor workflow `status` before editing. Absent or `created` may be edited.
`planned`, `assigned`, `in_progress`, `review`, `blocked`, `done`, and
`informational` are locked: do not change that workflow, and do not apply a
chain edit that includes it, unless the user explicitly confirms that workflow
and those changes. Confirmation does not unlock siblings. Say exactly what was
left locked. Editing `done` reopens it; do not do that silently.

### 2. Discover the Event Model before authoring HCL

Use business time, left to right:

1. Brainstorm concrete domain Events as past-tense facts.
2. Put Events in the order in which the business observes them. Ask: what is the
   first Event, what is the last Event, and what happens next?
3. Group the story into small capabilities; do not force final boundaries yet.
4. Add rough Screens for actor-visible interactions.
5. Add Commands, Read Models, and machine Processors to explain input and
   output.
6. Identify actors, teams, systems, bounded contexts, and external ownership.
7. Detail one flow at a time with fields and scenarios.

One well-specified slice is better than several speculative slices.

### 3. Choose exactly one pattern per capability

- **State Change** — a human or API trigger expresses intent:
  `Screen/API -> Command -> Event`. One screen state issues one command.
- **State View** — existing facts answer one question:
  `Event(s) -> Read Model -> Screen`.
- **Automation** — the system reacts to internal facts:
  `internal Event(s) -> pending-work Read Model -> Processor -> Command -> Event`.
- **Translation** — an external context's fact becomes the receiving context's
  language:
  `external Event(s) -> pending-work Read Model -> Processor -> Command -> internal Event`.

Do not invent a fifth pattern. A visual gear maps to a workflow-local
`processor`; `automation` is the top-level workflow kind. The validator allows
a processor to consume events directly; the method still wants the pending-work
read model unless the user asked for a minimal legal workflow. Record that
exception. Do not use `external_trigger` or `triggers` to hide a missing issuer
or an invisible signal.

### 4. Establish ownership and contracts

Declare stable lower-snake-case identities. Put canonical `event`, `aggregate`,
and reusable `field_type` blocks inside their owning `bounded_context`.

Treat an aggregate as a consistency boundary, not as every noun in the domain.
A command that must atomically modify several aggregates is a boundary smell:
revisit the aggregate boundary or model eventual consistency as separate
slices.

Mark third-party systems and their bounded contexts `external = true`.
Translation must consume at least one Event owned by an external bounded
context; Automation must consume none.

Prefer labels whose derived titles are correct. Add `title` only to deliberately
override title-casing.

### 5. Add fields using ubiquitous language

Use reusable `field_type` declarations for recurring domain concepts. Use a
plain built-in type only for one-off fields. Mark identifiers, PII, optionality,
technical data, list cardinality, and examples only when known. Use
`generated = true` only for system-filled values, never for user input.

Run the information-completeness check for every edge and scenario:

- every Event field must be available to the deciding Command or legitimately
  generated by the domain;
- every Read Model field must be derivable from its linked Events;
- every Command triggered by a Screen, Read Model, or Processor must receive the
  data required to decide;
- every target field must have a traceable source or an explicit domain rule.

Lineage is the same field name, or a documented rename, across a real edge,
plus `description`, `comment`, or `hotspot` when the value is derived. Do not
encode it with a `mapping` attribute. Do not silently add data because an
implementation might need it. Use a hotspot when provenance is unresolved.

### 6. Encode canonical flows

Relationships are unquoted HCL traversals, never strings. Write each edge once:

- when a workflow element is the source, write `to` on that source;
- when a catalog Event is the source, write `from` on the receiving workflow
  element.

Never write reverse edges such as `command.from = [screen...]`,
`screen.from = [readmodel...]`, `processor.from = [readmodel...]`, or
`command.from = [processor...]`.

Use fully qualified context-owned references outside their context, for example
`event.orders.order_placed`, and local workflow references inside a workflow,
for example `command.place_order`.

### 7. Specify observable business behavior

Keep each `scenario` inside its workflow. Use HCL grammar, not canvas
Given/When/Then variants:

- State Change: zero or more Event `given`, exactly one Command `when`, one or
  more Event/Error `then`.
- State View: one or more Event `given`, no `when`, one or more Read Model/Error
  `then`.
- Automation/Translation: zero or more Event/Read Model `given`, exactly one
  Processor/Command `when`, one or more Event/Error `then`.

Methodology notes that leave an automation When empty, put the issued command
in Then, or assert "nothing dispatched" are not legal scenarios. Prose givens
and `storyline` blocks are not legal either. See `references/methodology.md`.

Each step has exactly one typed target. There is no Query concept and no
`query` attribute. Use `comment { description = ... }` for semantic notes;
ordinary HCL comments are non-semantic. Rejection is `then { error = "..." }`,
not `expect_empty_list`. A tracked business failure is an event.

Model supplied success and failure rules. Do not fabricate error wording or
business decisions. Record missing rules as hotspots.

### 8. Review modeling quality

Before validation, inspect the model as a business story. On a review request,
report these findings; do not silently restructure. When authoring a requested
model, fix them.

- **Bed:** one Screen's `to` lists more than one Command. Any extra command is
  the problem, not only an "unrelated" one. Split into separate screen states.
  `EM401` warns at that threshold; it is not a hard error.
- **Left chair:** one Command produces more than one Event (`EM402`). The method
  treats more than two as a candidate to discuss, not an automatic split.
- **Right chair:** one Read Model consumes more than one Event (`EM403`). The
  method investigates fan-in above three, field by field.
- **Shelf:** scenarios concentrated in one workflow while other workflows have
  none (`EM405`). Also ask when one workflow has noticeably more cases than
  its neighbors.

Also check that every Command has one issuer and a validator reason—an incoming
canonical flow, `api_endpoint`, or a justified `external_trigger`—and every
Read Model answers one concrete question. A valid file can still be a poor
model.

### 9. Format, validate, and repair the source

Prefer a repository-local compatible binary, then a PATH-installed binary.
Check the version with `emhcl version`. A folder model needs v0.9.0 or newer.
Do not install tooling or execute untrusted downloads without the user's
request.

Run, for a one-file model:

```text
emhcl fmt -w <model.em.hcl>
emhcl validate <model.em.hcl>
```

For a folder model, `fmt` refuses the folder. Format each member file, then
validate the folder:

```text
emhcl fmt -w <folder>/<file>.em.hcl   # once for each member file
emhcl validate <folder>
```

For an implementation-ready model, also run:

```text
emhcl validate --profile strict <model.em.hcl | folder>
```

Repair source errors rather than suppressing diagnostics. Stable diagnostic
families are: `EM0xx` structure, `EM1xx` references, `EM2xx` flow, `EM3xx`
scenarios, and `EM4xx` modeling judgment. In a folder model, each diagnostic
names its member file. Folder-only codes are `EM013` (chapters in several
files), `EM014` (workflow in two chapters), and `EM407` (workflow in no chapter,
an error in `strict`).

If no compatible CLI is available, perform the complete static checklist in
`references/language-v0.4.md` and report that machine validation was not run.
Never claim validation from inspection alone.

### 10. Report precisely

State:

- the output file path, or the folder path and its member files;
- the modeled bounded contexts and workflows;
- unresolved hotspots;
- exact formatter/validator commands run and their observed result.

Keep the final explanation short. The model file or folder is the primary
artifact.
