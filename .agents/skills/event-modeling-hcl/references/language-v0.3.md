# Event Modeling HCL v0.3.0 reference

This is a compact authoring checklist derived from the normative v0.3.0
specification. One model is one `.em.hcl` document.

## Language principles

- Block kind expresses the concept; quoted labels are stable identity.
- Labels match `^[a-z][a-z0-9_]*$`.
- Source order is model order; preserve the business-time story.
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
  mapping             = "Optional mapping metadata"
  schema              = "Optional schema metadata"
  cardinality         = "Single"
  optional            = false
  technical_attribute = false
  generated           = false
  id_attribute        = true
  pii                  = false
}
```

When a plain field's name matches a uniquely resolvable `field_type`, omit
`type`:

```hcl
field "order_id" {}
```

Inside an Event/subfield, inference resolves in the owning context. On workflow
elements, tables, and scenario steps it resolves by unique document-wide name;
write the full type traversal if ambiguous or differently named.

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

## Scenarios

Each step has exactly one target. `error` is a literal string.

| Workflow | Given | When | Then |
| --- | --- | --- | --- |
| State Change | zero or more Events | exactly one Command | one or more Events/Errors |
| State View | one or more Events | none | one or more Read Models/Errors |
| Automation/Translation | zero or more Events/Read Models | exactly one Processor/Command | one or more Events/Errors |

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

Chapter workflows must be a non-empty contiguous source-order range.

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

## Static completion checklist

- File name ends in `.em.hcl`; model is one complete document.
- All labels are lower snake case and unique in their scope.
- Events, aggregates, and field types are owned by bounded contexts.
- All references are unquoted, resolvable, correctly qualified, and right-kind.
- Every edge appears once in canonical direction.
- Every Read Model has one concrete `question`.
- Every Command has a reason.
- Automation/Translation externality is correct.
- Scenario cardinality and targets match the workflow pattern.
- Field examples match effective type/cardinality; no duplicate shorthand
  fields.
- Chapter ranges are contiguous.
- Genuine unknowns are hotspots, not weakened or invented contracts.
- Source order reads as business time.

## Tooling

Use a CLI implementing specification v0.3.0 (implementation v0.4.0+):

```text
eventmodeling-hcl fmt -w model.em.hcl
eventmodeling-hcl validate model.em.hcl
eventmodeling-hcl validate --profile strict model.em.hcl
eventmodeling-hcl diagram model.em.hcl -o model.html
```

Profiles: `workshop` reports judgment diagnostics as information; default
`valid` reports them as warnings; `strict` escalates unreasoned Commands and
open hotspots to errors. Formatting is idempotent and preserves block/scenario
order.
