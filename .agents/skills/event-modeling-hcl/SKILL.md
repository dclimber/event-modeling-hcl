---
name: event-modeling-hcl
description: >-
  Turn product ideas, requirements, workshop notes, user stories, domain prose,
  existing behavior, or event-driven designs into valid Event Modeling HCL
  v0.3.0 `.em.hcl` files. Use this skill whenever the user asks to create,
  extend, repair, review, validate, format, or explain an Event Model or
  `.em.hcl` file, or mentions Events, Commands, Read Models, State Change,
  State View, Automation, Translation, Event Modeling slices, information
  completeness, or Given/When/Then scenarios—even if they do not name HCL.
compatibility: >-
  Targets Event Modeling HCL Specification v0.3.0. For machine verification,
  use emhcl v0.4.0 or newer implementing specification v0.3.0.
metadata:
  version: "1.0.0"
  author: workspace
  specification: "https://github.com/event-modeling-hcl/spec"
---

# Event Modeling HCL

Create one coherent, hand-authored `.em.hcl` document from the user's domain
input. Preserve Event Modeling's business-first discovery process: model facts
and information flow before implementation structure, then encode the result in
the strict native HCL language.

## Load only the references needed

- Read [references/methodology.md](references/methodology.md) when discovering a
  new model, interpreting prose, choosing slice boundaries, or reviewing model
  quality.
- Read [references/language-v0.3.md](references/language-v0.3.md) before writing
  or repairing HCL. It is the local syntax and validation reference.
- Read [references/patterns.md](references/patterns.md) when a pattern shape or
  scenario form is uncertain. Adapt examples to the domain; never copy their
  placeholder vocabulary into the output.

For conflicts, the normative Event Modeling HCL v0.3.0 specification wins over
examples, existing files, or general HCL knowledge.

## Deliverable

Produce or update the requested `.em.hcl` file. A model is not complete merely
because a diagram or prose explanation looks plausible.

Unless the user asks for a workshop-only draft, the delivered model must:

1. encode all supplied business behavior and every named acceptance criterion;
2. use one top-level workflow per independently useful business capability;
3. include precise fields where the input supplies them;
4. include scenarios for supplied business rules and important success/error
   paths;
5. pass formatting and default validation when the CLI is available;
6. pass strict validation before being called implementation-ready;
7. retain genuine unknowns as attached `hotspot` blocks instead of inventing
   facts.

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
work can continue—record an open hotspot and proceed.

When extending an existing model, preserve its valid identity vocabulary and
source-order story. Migrate every affected reference; do not leave obsolete
aliases or duplicate contracts.

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

- **State Change** — a human/API/external trigger expresses intent:
  `Screen/API -> Command -> Event`.
- **State View** — existing facts answer a question:
  `Event(s) -> Read Model -> Screen`.
- **Automation** — the system reacts to internal facts:
  `internal Event(s) -> Read Model/Processor -> Command -> Event`.
- **Translation** — an external context's fact is translated into the receiving
  context's language:
  `external Event(s) -> Read Model/Processor -> Command -> internal Event`.

Do not invent a fifth pattern. A visual gear maps to a workflow-local
`processor`; `automation` is the top-level workflow kind.

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
technical data, list cardinality, and examples only when known.

Run the information-completeness check for every edge and scenario:

- every Event field must be available to the deciding Command or legitimately
  generated by the domain;
- every Read Model field must be derivable from its linked Events;
- every Command triggered by a Screen, Read Model, or Processor must receive the
  data required to decide;
- every target field must have a traceable source or an explicit domain rule.

Do not silently add data because an implementation might need it. Use a hotspot
when provenance or authority is unresolved.

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

Keep each `scenario` inside its workflow. Use pattern-specific grammar:

- State Change: zero or more Event `given`, exactly one Command `when`, one or
  more Event/Error `then`.
- State View: one or more Event `given`, no `when`, one or more Read Model/Error
  `then`.
- Automation/Translation: zero or more Event/Read Model `given`, exactly one
  Processor/Command `when`, one or more Event/Error `then`.

Each step has exactly one typed target. There is no Query concept and no
`query` attribute. Use `comment { description = ... }` for semantic notes;
ordinary HCL comments are non-semantic.

Model supplied success and failure rules. Do not fabricate error wording or
business decisions. Record missing rules as hotspots.

### 8. Review modeling quality

Before validation, inspect the model as a business story:

- **Left chair:** one Command produces many unrelated Events.
- **Right chair:** many Events feed one vague Read Model.
- **Bed:** one Screen triggers many unrelated Commands.
- **Shelf:** scenarios are concentrated in one workflow while other important
  workflows remain implicit.

Also check that every Command has a reason—an incoming canonical flow,
`api_endpoint`, or `external_trigger = true`—and every Read Model answers one
concrete question. A valid file can still be a poor model.

### 9. Format, validate, and repair the source

Prefer a repository-local compatible binary, then a PATH-installed binary. Do
not install tooling or execute untrusted downloads without the user's request.

Run:

```text
emhcl fmt -w <model.em.hcl>
emhcl validate <model.em.hcl>
```

For an implementation-ready model, also run:

```text
emhcl validate --profile strict <model.em.hcl>
```

Repair source errors rather than suppressing diagnostics. Stable diagnostic
families are: `EM0xx` structure, `EM1xx` references, `EM2xx` flow, `EM3xx`
scenarios, and `EM4xx` modeling judgment.

If no compatible CLI is available, perform the complete static checklist in
`references/language-v0.3.md` and report that machine validation was not run.
Never claim validation from inspection alone.

### 10. Report precisely

State:

- the output file path;
- the modeled bounded contexts and workflows;
- unresolved hotspots;
- exact formatter/validator commands run and their observed result.

Keep the final explanation short. The `.em.hcl` file is the primary artifact.
