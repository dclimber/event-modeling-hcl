# Event Modeling HCL v0.4.0 reference

This is a compact authoring checklist derived from the normative v0.4.0
specification. A model is one `.em.hcl` file or a folder of `.em.hcl` files.
v0.4.0 adds only folder models. Every v0.3.0 file stays valid and keeps its
meaning.

## Language principles

- Block kind expresses the concept; quoted labels are stable identity.
- Labels match `^[a-z][a-z0-9_]*$`.
- Model order is file name order, then source order inside each file. Preserve
  the business-time story. In a folder model with two or more files, chapters
  set the workflow order (see "Folder models").
- Unknown syntax, unresolved references, wrong-kind references, computed values,
  variables, functions, and interpolation are errors.
- Relationships are unquoted traversals, not strings.
- Titles are optional and derived by title-casing labels, except required
  semantic data such as `readmodel.question`, `actor.auth_required`,
  `field_type.type`, `hotspot.question`, and `chapter.workflows`.

## Top-level blocks

```text
bounded_context, actor, team, system,
chapter, hotspot,
state_change, state_view, automation, translation
```

A bounded context may contain only `aggregate`, `field_type`, and canonical
`event` blocks. A workflow may contain `command`, `readmodel`, `screen`,
`processor`, `screen_image`, `table`, and `scenario` blocks, subject to its
pattern's flow rules.

## Catalog attributes

```hcl
team "fulfillment_team" {
  title       = "Fulfillment"
  description = "Optional description."
}

system "payment_provider" {
  title       = "Payment provider"
  description = "Optional description."
  external    = true
}

actor "customer" {
  title         = "Customer"
  description   = "Optional description."
  auth_required = true # required
}

bounded_context "orders" {
  title       = "Orders"
  description = "Optional description."
  external    = false
  owner       = team.fulfillment_team
}
```

`bounded_context.owner` may reference a bounded context, team, or system.
Teams are internal owner records; only systems have the `external` attribute.

## Context-owned contracts

```hcl
bounded_context "orders" {
  aggregate "order" {
    title       = "Order"
    description = "Consistency boundary for an order."
  }

  field_type "order_id" {
    type         = "UUID"
    cardinality  = "Single"
    example      = "0e42f9d1-2f55-4cc6-b9e2-3d1926bd21bf"
    id_attribute = true
  }

  event "order_placed" {
    title                  = "Order Placed"
    description            = "Optional description."
    group_id               = "checkout"
    tags                   = ["orders"]
    aggregate              = aggregate.order
    aggregate_dependencies = []
    service                = null
    fields                 = [field_type.order_id]
    sketched               = false
    prototype              = { route = "/orders" }
    list_element           = false
  }
}
```

Outside the context use qualified references:

```hcl
event.orders.order_placed
aggregate.orders.order
field_type.orders.order_id
```

Inside the owning context, local `aggregate.order` and `field_type.order_id`
forms are allowed.

## Fields

Built-in types:

```text
String Boolean Double Decimal Long Custom Date DateTime UUID Int
```

Cardinality is `Single` or `List`.

A reusable `field_type` must have an explicit built-in `type`. It may contain
nested `subfield` blocks. A plain `field` may use:

```hcl
field "customer_id" {
  type                = field_type.customers.customer_id # traversal or built-in string
  example             = "customer-42"
  cardinality         = "Single"
  optional            = false
  technical_attribute = false
  generated           = false
  id_attribute        = true
  pii                 = false
}
```

The current CLI also accepts unchecked `mapping` and `schema` strings. They are
not v0.4.0 contracts and not methodology lineage. `emhcl import` can write them
to keep JSON data. Keep them in an imported file, but do not add new ones. Do not write
`session:`, `latest:`, `derived:`, or `aggregate:` expressions. Record a
derivation in `description`, a scenario `comment`, or a `hotspot`.
`generated = true` means the system fills the value; it is not a source link.

When a plain field's name matches a uniquely resolvable `field_type`, omit
`type`:

```hcl
field "order_id" {}
```

Inside an Event/subfield, inference resolves in the owning context. On workflow
elements, tables, and scenario steps it resolves by a unique name in the whole
model, across all files of a folder model. If the name is ambiguous or
different, write the full type traversal.

Use `fields` for several reusable types with no per-field overrides:

```hcl
fields = [field_type.orders.order_id, field_type.orders.customer_id]
```

List entries become fields named after the final traversal segment. Do not
duplicate a name between `fields` and nested `field` blocks.

For structured data use `type = "Custom"`, nested `subfield` blocks, and
`cardinality = "List"` when applicable. Examples are native HCL values and must
match effective type/cardinality.

## Workflow attributes

Every workflow accepts `title`, `description`, `owner`, and `status`. Status is
one of:

```text
created planned assigned in_progress review blocked done informational
```

Commands, Read Models, Screens, and Processors accept:

```text
group_id, tags, title, description,
aggregate, aggregate_dependencies, api_endpoint, service,
creates_aggregate, external_trigger, triggers,
sketched, prototype, list_element,
from, to, fields, nested field blocks
```

A Read Model additionally requires `question`. A Screen additionally accepts
`actor`. Use only attributes with actual known meaning; do not fill metadata for
completeness.

Presentation blocks:

```hcl
screen_image "checkout_wireframe" {
  title = "Checkout wireframe"
  url   = "https://example.test/checkout.png"
}

table "example_orders" {
  title  = "Example orders"
  fields = [field_type.orders.order_id]
}
```

These blocks have no flow edges.

## Reference forms

| Target | Form |
| --- | --- |
| Bounded context | `bounded_context.orders` |
| Actor | `actor.customer` |
| Team | `team.fulfillment_team` |
| System | `system.payment_provider` |
| Workflow | `workflow.place_order` |
| Context Event | `event.orders.order_placed` |
| Context aggregate | `aggregate.orders.order` |
| Context field type | `field_type.orders.order_id` |
| Local workflow element | `command.place_order` |
| Qualified element for hotspot | `command.place_order.place_order` |

Reference lists use native HCL lists. Never quote a reference:

```hcl
to = [event.orders.order_placed] # correct
to = ["event.orders.order_placed"] # invalid
```

## Canonical flows

| Workflow | Legal canonical edges |
| --- | --- |
| `state_change` | `screen.to -> command`; `command.to -> event` |
| `state_view` | `event -> readmodel.from`; `readmodel.to -> screen` |
| `automation` | `event -> readmodel.from` or `processor.from`; `readmodel.to -> processor`; `processor.to -> command`; `command.to -> event` |
| `translation` | same as Automation, with at least one consumed external Event |

The Event belongs to the catalog, so the receiver records Event input with
`from`. Workflow-local sources record outgoing edges with `to`. Express each
edge exactly once. Reverse forms are errors.

An Automation cannot consume an Event from an external bounded context. A
Translation must consume at least one.

A Command should have an incoming edge, an `api_endpoint`, or
`external_trigger = true`; otherwise validation reports a modeling diagnostic.
The validator does not count issuers. Methodology still requires exactly one
screen or processor issuer. `external_trigger` does not replace that issuer
when the trigger can be modeled.

`triggers` is an unchecked string list. It does not create an issuer and does
not replace an event. Do not use it as an invisible signal.

## Attributes and blocks that do not exist

Do not invent `linked_copy`, `element_copy`, `storyline`, `query`, `note`,
`roles`, `expect_no_dispatch`, or a field-source traversal. Canvas linked
copies are one catalog event referenced from many workflows. Storylines are
multiple scenarios or ordered `given` steps. There is no backward-edge form;
a todo list includes its completion event in `readmodel.from`.

## Scenarios

Each step has exactly one target. `error` is a literal string.

| Workflow | Given | When | Then |
| --- | --- | --- | --- |
| State Change | zero or more Events | exactly one Command | one or more Events/Errors |
| State View | one or more Events | none | one or more Read Models/Errors |
| Automation/Translation | zero or more Events/Read Models | exactly one Processor/Command | one or more Events/Errors |

Canvas automation specs that use an empty When, a Then command, or "nothing
dispatched" are not this grammar. Do not encode them by inventing attributes.

Scenario and step attributes:

```hcl
scenario "success" {
  title       = "Optional title"
  description = "Optional description"

  given {
    title             = "Optional step title"
    tags              = ["setup"]
    examples          = [{ order_id = "order-42" }]
    expect_empty_list = false
    event             = event.orders.order_placed
    fields            = [field_type.orders.order_id]
  }

  comment {
    description = "Semantic note retained in the model."
  }
}
```

A step may target `event`, `command`, `readmodel`, `processor`, or `error` only
as allowed by its workflow and step kind. There is no `query` target. Normal HCL
comments are non-semantic.

## Workshop notation

```hcl
chapter "checkout" {
  title       = "Checkout"
  description = "Optional description."
  workflows   = [workflow.review_cart, workflow.place_order]
}
```

In a one-file model, the chapter workflows must be a non-empty contiguous
source-order range (`EM006`). In a folder model, `EM006` does not apply. The
chapter lists set the workflow order instead.

```hcl
hotspot "payment_authority" {
  question    = "Which payment status is authoritative?"
  description = "Optional context."
  status      = "open" # open or resolved
  on          = workflow.capture_payment
}
```

`hotspot.on` can reference catalog/workshop declarations, a workflow, or a
workflow-qualified element such as `processor.capture_payment.gateway`.

## Folder models

A model path is a file or a folder. Use a folder when one file is too large to
read, or when different people own different contexts or workflows.

The member files of a folder model obey these rules:

- A member file is a regular file directly in the folder. Its name ends in
  `.em.hcl` and does not start with `.`. A symlink to a file counts.
  Subfolders and other files are not members.
- The tool sorts member files by name, byte by byte. A number prefix such as
  `10-` is not language syntax. It only sets the file order.
- The model is the union of the top-level blocks of all files. A reference in
  one file can point to a block in any other file. Every check runs on the
  whole model. Each file must parse on its own.
- A block never spans files. Thus a `bounded_context` and all its events,
  aggregates, and field types are in one file.
- IDs are global across files. Each block kind has its own ID space, as in a
  one-file model. All workflow kinds share one space. The same ID twice is
  `EM002`. The detail names the first declaration as `file:line:column`.
- A folder with no member file is `EM001`. A folder with exactly one member
  file behaves exactly like that file. The chapter rules below apply only to
  two or more files.
- All `chapter` blocks are in one file. Chapters in several files are `EM013`.
  A workflow in two chapters is `EM014`.
- The workflow order is the chapter order in their file, then the order of each
  `workflows` list. A workflow in no chapter is judgment diagnostic `EM407`. It
  comes after all chaptered workflows, in model order.
- Each diagnostic names the member file of the source that it points at.

The language has no `include` or `import` block, no subfolder members, no
modules or name spaces, and no cross-folder references.

Use this layout when you split a model:

```text
model/
  00-catalog.em.hcl      # bounded_context, actor, team, system
  01-chapters.em.hcl     # every chapter block
  10-register-pet.em.hcl # one workflow per file
  11-pet-directory.em.hcl
  90-hotspots.em.hcl     # hotspot blocks
```

Put every workflow in a chapter. Move each block whole.

## Static completion checklist

- Each file name ends in `.em.hcl`. The model is one complete file or one
  complete folder.
- All labels are lower snake case and unique in their scope.
- Events, aggregates, and field types are owned by bounded contexts.
- All references are unquoted, resolvable, correctly qualified, and right-kind.
- Every edge appears once in canonical direction.
- Every Read Model has one concrete `question`.
- Every Command has a validator reason (incoming flow, `api_endpoint`, or
  `external_trigger`). Exactly one issuer is methodology, not a counted check.
- No invented `linked_copy`, `storyline`, `query`, or lineage `mapping` DSL.
- Scenario cardinality and targets match the workflow pattern.
- Field examples match effective type/cardinality; no duplicate shorthand
  fields.
- In a one-file model, chapter ranges are contiguous. In a folder model, all
  chapters are in one file, no workflow is in two chapters, and every workflow
  is in a chapter.
- Genuine unknowns are hotspots, not weakened or invented contracts.
- Model order (file order, then source order, or chapter order in a folder
  model) reads as business time.

## Tooling

Use emhcl v0.9.0 or newer. It implements specification v0.4.0. Release v0.8.0
and older load one file only.

```text
emhcl fmt -w model.em.hcl
emhcl validate model.em.hcl
emhcl validate --profile strict model.em.hcl
emhcl diagram model.em.hcl -o model.html
emhcl serve model.em.hcl
```

`validate`, `diagram`, `serve`, and `export` accept a file or a folder in
place of `model.em.hcl`. `fmt` formats one file and refuses a folder. For a
folder model, run `fmt -w` on each member file. `serve` reloads the page when a
member file is added, removed, renamed, or edited.

Profiles: `workshop` reports judgment diagnostics as information. The default
`valid` reports them as warnings. `strict` makes unreasoned Commands (`EM404`),
open hotspots (`EM406`), and unchaptered workflows in a folder model (`EM407`)
errors. Formatting is idempotent and preserves block and scenario order.

### JSON conversion

emhcl v0.8.0 and newer convert slice-based Event Modeling JSON. JSON
conversion is a tool feature, not part of the language.

```text
emhcl import model.json -o model.em.hcl
emhcl export model.em.hcl -o model.json
```

- `import` writes one formatted file and validates it. It exits with code 1 if
  the result has validation errors. Repair the written source.
- `export` accepts a file or a folder. It refuses an `-o` target that ends in
  `.em.hcl` or that is the source file.
- Conversion is not lossless. Read each warning. Chapters, hotspots, teams,
  systems, read-model questions, and canvas IDs have no JSON equivalent.
- Both commands fail when one workflow or slice has two or more actors.
- Do not use `export` and `import` to repair a model. Edit the HCL.
