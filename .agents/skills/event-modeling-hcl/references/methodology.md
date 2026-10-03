# Event Modeling methodology

Use this reference to turn incomplete domain input into a coherent business-time
model, then encode it with the blocks in `language-v0.3.md`. It is not a second
syntax manual.

Authoritative source:
[EVENT_MODELING_METHODOLOGY.md](https://github.com/event-modeling-hcl/spec/blob/main/guides/EVENT_MODELING_METHODOLOGY.md).
Where that guide's sources disagree, its Appendix C states the tension and the
operative rule. Follow Appendix C over a conflicting worked example. HCL
grammar still wins over both when the operative rule is not legal syntax.

The canvas method is technology-agnostic. HCL v0.3.0 is the encoding. When they
disagree, write legal HCL and keep the methodology rule as judgment, a
`description`, a scenario `comment`, or a `hotspot`. Never invent an attribute
to close the gap.

Three layers:

| Layer | What it decides | If it fails |
| --- | --- | --- |
| Validator / spec | Block kinds, references, canonical edges, scenario grammar, externality | The file is invalid. Repair it. |
| Methodology | Naming, causality, slice boundaries, lineage, scenario coverage, ownership | The file can still validate. Do not call that a good model. |
| Canvas layout | Columns, lanes, stickies, linked copies, storyline drawings | No HCL syntax. Encode the intent or record the gap. |

Event Modeling does not require Event Sourcing. Do not add brokers, databases,
retries, or handlers unless the user asks for implementation.

## Postures

Do not mix these in one pass.

- **Modeling** (discover, extend, encode): capture the process. Flag a naming
  slip or incomplete precondition and continue. Do not stop for a full audit.
- **Critic** (review, completeness, "is this ready?"): report every violation.
  Record findings as `hotspot` blocks or scenario `comment`s. Do not quietly
  restructure unless the user asked for fixes.

Structural diagnostics (`EM0xx`–`EM3xx`) are not critic-mode opinions. When the
user asked for a model file, repair those. Judgment diagnostics (`EM4xx`) and
the rules below are methodology unless a profile escalates them.

An open `hotspot` is the right place for a real unknown. `emhcl validate
--profile strict` turns open hotspots and unreasoned commands into errors, so a
strict-clean file is not a place to leave them. Resolve the question, or record
an explicit assumption in `description` and do not call the model
implementation-ready while the hotspot stays open. Never invent the business
rule just to clear the diagnostic.

## What not to invent

These methodology words are not HCL attributes or block kinds. Do not add them.

| Canvas idea | Legal encoding | Do not write |
| --- | --- | --- |
| Linked copy of an event | One catalog `event`, referenced from every workflow that uses that fact | `linked_copy`, `element_copy`, a second event block with the same meaning |
| Later update to a view | Same `readmodel.from` gains the later event, or a new `state_view` if the question changed | A copied read model plus a link attribute |
| Storyline / beats | Several `scenario` blocks, or one scenario with ordered `given` events | `storyline { }` |
| Field lineage expressions | Same `field_type` / field name across the connected edge; `generated = true` when the system fills the value; prose in `description`, `comment`, or `hotspot` | `mapping = "session:…"`, `latest:`, `derived:`, `aggregate:`, or any other mapping DSL |
| Invisible signal / timer | A real `event` that opens work, then a `processor` | `triggers = ["daily"]` as the only cause; a pseudo-screen for a scheduler |
| Role catalog | `actor` / `system` / `team` plus `description` of can and cannot | `roles = […]` (not in v0.3.0) |
| Notes lane | `chapter.description`, element `description`, scenario `comment`, `hotspot` | `note` blocks |
| Spec lane wiring | Scenarios are nested in the workflow and have no `from` / `to` | Edges into scenarios |
| Query / When-query | State view has no `when` | `query`, a `Get…` command |

`mapping` and `schema` happen to be accepted as unchecked strings by the
current CLI. They are not a lineage language and not a specification contract.
Do not add them to new or repaired models. If an existing file already has
them, leave them unless the user asked to repair that metadata.

`list_element` is presentation. It does not mean "todo list", and a field with
`cardinality = "List"` is not a list-shaped read model. Say the list question
in `readmodel.question`.

## Elements

Model the process as it would work on paper. A spinner, cache refresh, or
session check is not a business step.

- **Event:** an immutable past-tense business fact. Specific, usually a few
  words (`order_placed`, not `record_updated`, `sidebar_opened`, `api_called`,
  or `inventory_level_recalculated`). A correction is a new event. No computed
  value that changes when source data changes. Each fact has one owning
  bounded context and one aggregate consistency boundary. Payload-free events
  are allowed when the name is the fact; otherwise carry identity plus the
  facts that make it meaningful. Do not pad.
- **Command:** rejectable imperative intent (`place_order`, not `load_orders`).
  Exactly one issuer. Success records one or more events. A precondition
  failure records nothing: an error scenario, not a silent drop. A failure the
  business tracks (`payment_failed`, `inventory_reservation_failed`) is an
  event. Commands are checked against prior facts, never against a read model.
- **Read model:** a rebuildable projection for one question. Name the data
  (`order_summary`), not a projector or repository. It never validates a
  command.
- **Screen:** one human decision surface. One screen state issues at most one
  command. Several actions are several screens (and usually several
  `state_change` workflows). No human in the step means no screen: use a
  processor. Same titles do not create identity; HCL has no page-composition
  link.
- **Processor:** the gear. Business name (`billing_scheduler`), not
  `order_handler`. It reads pending work, decides, and issues a command. It
  does not write the target's events directly.
- **Hotspot:** an open pain point or dispute. Not an event. `question` is
  required. Resolving means answering, not deleting the concern without a
  trace; set `status = "resolved"` only when the answer is recorded.

Generic `user` is not an actor. Give each human role an `actor` and each
external system a `system` with `external = true` plus an external
`bounded_context`. Put can / cannot in `description`. A role with no command
and no view is decorative: give it real work or remove it.

**Decision-time totals** (Appendix C): do not store values that recalculate as
data changes. A total fixed when the command is accepted is an unresolved
domain choice, not a license to copy a worked example. Record it as a hotspot
unless the user already decided.

## Causality and workflows

Information flow:

```text
screen or processor -> command -> event -> read model -> screen or processor
```

A story starts with a state view, or with an automation/translation reacting to
an event that already happened. It does not start with an unmotivated command.

HCL already encodes a slice as one workflow of one pattern. Do not combine an
independent state change and state view into one workflow. The validator
rejects that child mix: a `state_change` cannot contain a read model, and a
`state_view` cannot contain a command. An `automation` or `translation` does
contain its own todo-list read model and the command that processor issues.
Do not split the issuing screen away from its command into a second workflow.
The view of the result is a separate `state_view`.

| Pattern | HCL workflow | Shape to author |
| --- | --- | --- |
| State change | `state_change` | `screen.to -> command.to -> event`. API: `api_endpoint` on the command, no fake screen. |
| State view | `state_view` | `readmodel.from = [event…]`, `readmodel.to -> screen`. No command. |
| Automation | `automation` | Internal events only. Recommended: todo-list `readmodel` -> `processor` -> `command` -> internal event. |
| Translation | `translation` | At least one event from an external bounded context. Same shape as automation, emitting this domain's language. |

Validator, not merely advice:

- `automation` that consumes an external event is `EM012`. Use `translation`.
- `translation` with no external event is `EM011`.
- Reverse edges (`command.from`, `screen.from`, `processor.from = [readmodel…]`,
  `command.from = [processor…]`) are `EM201`.
- A command with no incoming canonical flow, no `api_endpoint`, and no
  `external_trigger` is `EM404` (warning; error in strict).

Methodology is stricter than those escapes:

- `external_trigger = true` is only for a real issuer that is deliberately
  outside the model. Say why. It is not a way to skip a screen, a processor,
  or a translation chain.
- A command has one issuer, never two. The validator does not count issuers.
  Two screens or a screen and a processor targeting the same command is still
  a modeling error.
- `api_endpoint` records a supplied route. It does not turn a query into a
  command.

Name the workflow after its defining element (the command, the read model, or
the automation's command). Never `state_change "order_management"`. Slices
communicate only by events: "depends on `order_placed`", not "depends on the
place-order slice".

Valid story transitions are view → change, change → view, change → automation,
automation → change, automation → view. Change → change with no new screen or
processor between them is a gap. In HCL that means two `state_change`
workflows in a row whose second command has no issuer of its own.

## Discovery order

Enter at the first incomplete step. Do not rerun finished discovery unless the
user asks. For a full model: events, then commands and screens, then read
models, then scenarios, then make every command / read model / automation its
own workflow. Plot the story before specifying fields. One finished slice is
better than several speculative ones.

1. Scope in a few sentences when it is not already clear. State perspective
   (bidder vs issuer, customer vs warehouse) when the source is ambiguous.
2. Role catalog, then past-tense facts. Assign each fact to one owning context.
   Chapters are journeys, not verbs and not pages. About 6–15 workflows is a
   smell threshold, not a quota. When in doubt, merge. A branch that starts a
   different story is a new chapter; a branch that only changes how this story
   ends stays here.
3. Order facts in business time. Show failures and cancellations, not only the
   happy path. Source order is that order. `chapter.workflows` must be a
   contiguous range.
4. Screens for human decisions; processors where no person acts. Deadlines and
   "submissions closed" are processors fed by events, not screens.
5. Commands: one per screen state, specific actor, preconditions as prior
   events, success events, and each distinct failure.
6. Outputs: every view screen needs a read model. An input screen needs prior
   state unless it is a genuinely blank creation form — rare, and the exemption
   must be written down. "Session context" is not an exemption.
7. Ownership: `bounded_context.owner` is a team, system, or context. External
   facts enter through translation.
8. Scenarios, then a critic pass, then stop. Do not invent extra slices once
   every command, read model, and automation has a workflow.

After authoring, a cold reader should see scope, identities, assumptions, and
rejected alternatives in `chapter.description`. Do not pad a simple chapter.

## Fields and lineage

Every command, event, read model, and screen field that the input supplies
must be traceable. A field with no source is a gap, not a detail to fill later.

HCL checks that references resolve. It does not check lineage. You check it:

- A command field is user input (present on the issuing screen), carried from
  a prior event the command's scenarios already use, or system-filled
  (`generated = true`: new id, timestamp). User input is never `generated`.
- An event field is copied from the accepted command (same name, or a documented
  rename in `description`) or is a documented derivation. Command inputs must
  not disappear.
- A read-model field comes from a linked event, or is derived from those
  fields. Derived totals live here, not in a recalculation event. Write the
  derivation in `description`. Do not invent a `derived:` attribute.
- A processor-issued command field comes from the consumed event or todo-list
  read model, or from a documented external result on a translation.
- Screen fields shown to the user come from the connected read model or are
  the inputs of the one command that screen issues.
- Reuse one realistic example value along a chain (`jane@example.com` stays
  that email). Do not overwrite a non-empty example. No `foo` / `test`.
- Prefer `Decimal` for money. Map prose types onto the built-ins in
  `language-v0.3.md` (`Date` vs `DateTime`, `Int` / `Long` / `Double` /
  `Decimal`). There is no `Text` or `Number` type.
- Identity is `id_attribute = true`. Two identity fields are a compound
  identity. Do not invent a compound-key block.
- `optional`, `pii`, and `technical_attribute` only when known. Do not mark
  infrastructure guesses as domain fields.

If the source element is not connected, add the legal edge or a hotspot. Do
not invent a connection, and do not point a mapping string at an unconnected
element.

Carry a rename or added field through the connected chain only. If any
workflow on that chain is status-locked, change none of the chain until the
user confirms the locked part.

## Scenarios: methodology grammar is not HCL

Author scenarios with the HCL grammar. The canvas sources use a different
Given/When/Then for automations and errors. Do not copy those forms.

| | Methodology sources | HCL v0.3.0 (required) |
| --- | --- | --- |
| State change | Given events; When at most one command; Then events or an "empty" error flag | Zero or more `given` events; **exactly one** `when` command; one or more `then` events **or** `then { error = "…" }` |
| State view | Given events; **empty When**; Then **exactly one** read model | One or more `given` events; **no `when`**; one or more `then` read models or errors. Zero givens is `EM301`. |
| Automation / translation | Given events (setup, trigger last); **empty When**; Then the **command issued, or nothing** | Zero or more `given` events or read models; **exactly one** `when` processor or command; one or more `then` events or errors. A Then command is illegal. There is no "dispatch nothing" target. |
| Prose given ("customer exists") | Used in some workshop notes | Illegal. The given target is an `event` (or a `readmodel` in automation/translation). Put prose in `description` or `comment`. |
| Storyline beats | Ordered lifecycle across elements | No `storyline` block. Use several scenarios, or ordered `given` steps in one scenario. |
| Cross-chapter empty Given | Allowed, with a title note | Illegal on a state view. Reference the real catalog event in `given` even when another chapter produced it. Do not copy the event block. |

Other scenario rules:

- Commands are specified with GWT, not a storyline. One isolated transition.
- Rejection is `then { error = "precise business reason" }`. That is not an
  empty list. `expect_empty_list = true` belongs only on a `then` read model
  that is intentionally empty. Do not put it on error steps.
- A recorded business failure is `then { event = … }`, not an error string.
- Do not mix an event and an error in one Then as if both happened. Separate
  scenarios.
- Name the scenario after the rule (`late_bid_is_rejected`), not `test_1`.
- HCL does not check that scenario targets are wired. You do. Given events
  are the prior facts the rule needs; Then events are events the command or
  processor actually produces.
- Methodology's "seven questions" (happy path, invalid input, illegal state,
  duplicate, alternate success, external failure, compensation) are a review
  list, not seven mandatory blocks. Write each one the input supports. Skip a
  type only when it cannot occur, and say why in a `comment`. Do not fabricate
  error wording.
- Every read model needs at least one view scenario, including every todo
  list. A state view with only command tests elsewhere is incomplete. List
  outcomes show concrete rows via step `field` examples, or explicitly expect
  an empty list.
- Idempotency: specify the target command's "already done" rejection. HCL
  cannot mark that rejection as a successful no-op. Use a `comment` on the
  automation scenario if a repeated trigger must be harmless. Do not invent
  `expect_no_dispatch`.
- Compensation examples that put an event in When and a command plus an event
  in Then are illegal in both the formal method and HCL. Model the reversing
  command as its own state change or automation.

`comment { description = "…" }` is semantic. `//` comments are not.

## Automations and todo lists

Methodology: every automation has a todo-list read model. No pure-relay
exemption. The validator still allows `processor.from = [event…]` with no read
model. Prefer the todo list. Omit it only when the user asked for a minimal
legal workflow, and say so in a hotspot or description.

Recommended shape, in one `automation` or `translation` workflow:

```text
event(s) -> readmodel.from
readmodel.to -> processor
processor.to -> command
command.to -> event(s)
```

The read model's question is pending work ("Which placed orders still need a
confirmation?"), not entity status. List membership means open. Do not add a
`status` field to mark rows done; a completion event drops the row. There is
no HCL attribute that deletes a row. Specify that in a scenario: after the
completion event, the todo read model is empty (`expect_empty_list = true`) or
no longer contains that row.

- Worker (automation): opened only by internal events. Closed by the completion
  event, usually its own result, by including that event in `readmodel.from`.
  That is the HCL form of the canvas's single backward-arrow exception. Do not
  reverse an edge.
- Translation: opened by the external event. Do not also feed it the internal
  event as a closer. It relays; it does not track done.
- A following worker exists only when it makes a new decision (invariant,
  choice, or data the internal event does not carry). If the next event would
  only rename an already-decided fact, stop after translation and project a
  state view from the internal event.
- Define the business failure event and the technical retry outcome in
  prose or scenarios. A technical retry that leaves the row is not a new event
  unless the business records the failure.
- A long-running wait, human approval, compensation, or fan-in is several
  workflows connected by events, not one processor with a hidden workflow
  engine. HCL has no saga block.

## Translation

Another system's fact — external system or another team's context — is not a
worker input. Translate first.

```text
external event -> todo-list read model -> processor -> command -> internal event
```

The internal event is a new fact in this domain's language, not a linked copy,
even when the business name matches (`payment_captured` outside,
`payment_authorized` inside). Reject transport names (`payment_signal_received`,
`order_synced`). Keep the external id only as a deduplication field
(`payment_gateway_ref`), never as `id_attribute` of the domain entity.

Before recording the internal fact, recover this domain's identity. If the
model starts the external action, record the correlation (our id sent outward,
or a pairing event) on our side. Missing correlation is a failure scenario and
often a manual-review hotspot, not a guessed join.

Translation is idempotent on the external reference: a second delivery does
not record a second domain fact. Specify that as the command's duplicate
rejection.

Classify each external field: copy into a domain field, enrich from our
events, ignore, or infer. Never silently assume. Do not copy a foreign payload
shape into the domain event. Cents-to-decimal and similar conversions are
derivations written in `description`, not `mapping` expressions.

Appendix C tension: the chain is not a business decision, but it still
validates and can fail. Keep both. Do not add a second "decision" event that
only repeats the translation.

## Read models

One question per read model. A screen area a person would point at separately
(stats tile vs live availability) is a separate `state_view`, not one wide
read model and not a highlighted screen copy. HCL has no highlight syntax.
`screen_image` may illustrate; it has no edges.

More than three source events is a methodology candidate to split (the right
chair). The validator warns earlier: `EM403` when `readmodel.from` lists more
than one event. Neither warning is an automatic split. Remove edges no field
uses. If one field really needs the whole lifecycle, say which field and why
in `description`. A wide field does not justify unrelated fields riding along.

Place the producing events in source order before the view workflow when the
story is one chapter. HCL does not enforce column position. Do not bunch every
read model at the end of the file if the narrative uses them earlier.
`chapter.workflows` order is the build/story order.

A read model with no consumer is an orphan. In HCL the consumer is `to` a
screen or, in an automation/translation, `to` a processor.

## Aggregates

An aggregate is a consistency boundary: the history of exactly one business
identity. "All orders" or a mixed log is a read model, not an aggregate.

Length is not a reason to split. Split when events do not belong to the same
entity's lifecycle. A child with its own lifecycle is its own aggregate; a
small child that is created and destroyed with the parent can stay inside it.

A command that must atomically change two aggregates is a boundary smell.
Prefer a fact in one aggregate and a later workflow in the other. The set of
prior events a rule needs is a property of that rule, not of the event type.
Scenarios show the scope. Do not widen `aggregate_dependencies` to paper over
an unclear boundary.

## Ownership

System shape follows who owns the facts. Each bounded context owns its events.
Cross-context facts are consumed by translation (other team or external
system), not by an automation that reaches into foreign events. The validator
only checks `external = true` on the event's context, not "different team".
A same-model internal context that another team owns should still be
translated if the user described an ownership boundary; mark that context
`external = true` only when it is outside the modeled system. If it stays
internal, an `automation` may consume its events — and you should still
hotspot a team boundary the validator cannot see.

No circular "team A waits on team B waits on team A" without an explicit
event contract. Redesign the boundary rather than adding a hidden call.

Do not create one workflow lane per team. Ownership is `owner`, not a swimlane
attribute. HCL has no swimlanes.

## Shapes

Internal names only. With stakeholders, describe the concern in plain words.

| Shape | Methodology | Validator |
| --- | --- | --- |
| Bed | **Always wrong:** a screen wired to **any** second command, related or not. One screen state, one command. Split the actions. | `EM401` when `screen.to` contains more than one command reference. Warning in default and strict. Not limited to "unrelated" commands. |
| Left chair | Candidate when one command produces **more than two** events. Ask whether they always happen together. | `EM402` when `command.to` contains more than one event. Warning, not a hard error. |
| Right chair | Candidate when one read model is built from **more than three** events. Judge fields, not the count alone. | `EM403` when `readmodel.from` contains more than one event. Warning. |
| Shelf | One workflow has noticeably more scenarios than its neighbors. No fixed count. | `EM405` only when one workflow holds every scenario, another workflow exists, and the total is at least two. Warning. |

Do not "fix" a justified multi-event command solely to silence `EM402`. Do not
ignore a bed because the commands feel related. Drop a candidate only after
the events, fields, or scenarios show a single job.

## Governance and status locks

Workflow `status` is the lock. The validator checks the enum and does not
enforce the lock. You do.

| Status | Agent behavior |
| --- | --- |
| absent or `created` | Editable. `created` is the only status the method treats as freely editable. |
| `planned`, `assigned`, `in_progress`, `review`, `blocked`, `done`, `informational` | Read freely. Do not change, move, rename, delete, or add fields, examples, scenarios, or edges in that workflow. |
| `informational` | Reference only. |
| `done` | Editing it reopens the work. Do not change `status` away from `done` unless the user confirmed that reopening. |

Confirmation unlocks only the named workflow and the named changes. It does
not unlock siblings. An unclear "ok" is not confirmation. If a request touches
locked and unlocked workflows, edit the unlocked ones and state exactly what
was left and which status blocked it. A chain-wide rename that includes a
locked workflow is blocked as a whole: change nothing on that chain.

Do not set `status` to the value it already has. Do not bump status along the
lifecycle (`created` → `planned` → … → `done`) unless the user asked. Status
is not a substitute for a hotspot.

Hotspot `status` is only `open` or `resolved`. Do not copy workflow statuses
onto hotspots.

## Evidence provenance

The source of a modeling choice must be visible when it is not obvious from
the user's latest instruction.

- **Documents** (tenders, specs, tickets): the text is authoritative. Do not
  drop a deadline, exclusion, or penalty. Keep numbers and dates verbatim in
  scenarios or `description`. Conflicting passages become one hotspot citing
  both, not a silent pick. Label each non-obvious choice as stated, implied,
  or assumed.
- **Code and schemas:** a hypothesis. Persisted state changes suggest events;
  a person-triggered write suggests a screen and command; a job or inbound
  call suggests a processor; a query suggests a read model. Technical type
  names are not element names. Tests and UI confirm vocabulary; they do not
  outrank an explicit stakeholder statement. Record the construct and the
  conclusion in `description` (`Assumed from OrderService.confirm; not
  confirmed by a domain expert`). An assumed material rule also gets an open
  hotspot.
- **Existing UI:** capture intent ("confirm the order"), not button labels.
  A new screen state is a navigation or content change, not a cosmetic one.
  Do not model every page combination.
- **Drift:** if implementation and model are both in hand, report each
  difference once at its origin. Do not fabricate a comparison. A `created` or
  `planned` workflow with no code is expected, not drift. Diagnosis does not
  authorize edits to locked workflows.

When nobody can answer and the user said not to block: take the most
reasonable assumption, write it where a reader will see it, and keep it
correctable. Never guess silently. One focused question beats a pile of
speculative slices.

## Review checklist

Validator-clean is necessary and not sufficient.

- Events are past-tense business facts; commands are imperative intent; names
  are not CRUD, UI, or framework words.
- Every command has one issuer. No screen issues two commands.
- No state change follows another without a new issuer.
- External facts are translated; workers consume internal events only.
- Every automation/translation the user asked to model fully has a pending-work
  read model, unless a recorded exception says otherwise.
- Every supplied field has a visible source. No new `mapping` DSL.
- Scenarios use HCL grammar. Rejections and recorded failures are not confused.
  Empty-list assertions are not used as errors.
- Aggregates answer "the history of which entity?".
- Unknowns are hotspots or labeled assumptions, not invented rules.
- Locked workflows were not edited.
- Both critic questions, in prose: could a new modeler follow the story
  quickly, and could a projection change without rewriting event history?

## Operative tensions

Do not "resolve" these by adding syntax. The operative rule is already above.

- Decision-time calculated fields in events: unresolved unless the user
  decided. Recalculating aggregates are never events.
- Refused command → error scenario and no event. Tracked business failure →
  event. The domain expert draws the line.
- Worker vs translation trigger: workers are internal-only; translation is the
  exception that may consume an external event.
- External ids: reconciliation fields only, never primary identity.
- Fan-in / fan-out counts differ between the method and `EM401`–`EM403`. Honor
  both: split a bed always; treat chair warnings as questions, using the
  methodology thresholds when deciding whether to split.
- State-view When is empty. There is no query step.
- Given steps are typed events, not prose.
- Slice status is checked for completeness in the method before the slicing
  step finishes. In HCL, declare the workflow when you specify the slice; do
  not invent a separate slice block.
- One command per screen **state**. Several actions mean several screens.
- Chapter-at-a-time (documents) vs all-timelines-then-detail (workshops) is a
  process choice. Source order still reads as business time.
- Planning numbers in the sources are illustrations. Estimate in workflows if
  asked; do not invent velocity.
