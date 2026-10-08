# Canonical workflow patterns

These are shape references, not vocabulary to copy. Each snippet assumes the
referenced catalog declarations exist in the same model, in the same file or in
another member file of the same folder.


Shapes below are legal HCL. Methodology additions (todo-list read models, one
command per screen, translation before domain work) are judgment: follow them
unless the user asked for a minimal legal workflow. Do not add `linked_copy`,
`storyline`, or `mapping` attributes. Scenario grammar here is HCL, not the
canvas automation form (empty When, Then command, or "nothing dispatched").

## State Change

A human-facing trigger:

```hcl
actor "customer" {
  auth_required = true
}

bounded_context "orders" {
  aggregate "order" {}

  field_type "order_id" {
    type         = "UUID"
    id_attribute = true
  }

  event "order_placed" {
    aggregate = aggregate.order
    field "order_id" {}
  }
}

state_change "place_order" {
  screen "checkout" {
    actor = actor.customer
    to    = [command.place_order]
    field "order_id" {}
  }

  command "place_order" {
    aggregate = aggregate.orders.order
    to        = [event.orders.order_placed]
    field "order_id" {}
  }

  scenario "order_is_placed" {
    when { command = command.place_order }
    then { event = event.orders.order_placed }
  }
}
```

For an API trigger, omit a fake Screen and use:

```hcl
command "place_order" {
  api_endpoint = "POST /orders"
  to           = [event.orders.order_placed]
}
```

Use `external_trigger = true` only when the external trigger is real but not
modeled as a Screen/API edge.

A screen `to` list with more than one command is the bed: any second command,
not only an unrelated one. Split the actions into separate screen states. The
validator warns (`EM401`) and does not reject the file.

## State View

```hcl
state_view "view_order" {
  readmodel "order_summary" {
    question = "What is the current status of this order?"
    from     = [event.orders.order_placed]
    to       = [screen.order_summary]
    field "order_id" {}
  }

  screen "order_summary" {
    actor = actor.customer
  }

  scenario "placed_order_is_visible" {
    given { event = event.orders.order_placed }
    then  { readmodel = readmodel.order_summary }
  }
}
```

State View scenarios never have `when` and never use `query`.

## Automation

Methodology shape: a pending-work read model, then the processor. The
validator also accepts `processor.from = [event…]` with no read model. Prefer
the read model; it is not a hard error to omit it.

```hcl
automation "request_order_confirmation" {
  readmodel "orders_needing_confirmation" {
    question = "Which placed orders still need confirmation?"
    from     = [event.orders.order_placed]
    to       = [processor.confirmation_processor]
  }

  processor "confirmation_processor" {
    to = [command.request_confirmation]
  }

  command "request_confirmation" {
    to = [event.notifications.order_confirmation_requested]
  }

  scenario "confirmation_is_requested" {
    given { event = event.orders.order_placed }
    when  { processor = processor.confirmation_processor }
    then  { event = event.notifications.order_confirmation_requested }
  }
}
```

The read model is a queue of pending items, not a status flag on the entity.
A completion event is added to that read model's `from` list; do not reverse
an edge, and do not invent a closer attribute. Automation consumes only
internal events. `when` is the processor or the command it issues. `then` is
an event or an error, never the command, and never empty.

## Translation

```hcl
system "payment_provider" {
  external = true
}

bounded_context "payment_provider" {
  owner    = system.payment_provider
  external = true

  event "payment_captured" {}
}

bounded_context "orders" {
  event "order_confirmed" {}
}

translation "confirm_paid_order" {
  readmodel "payments_to_record" {
    question = "Which captured payment facts need translation into order facts?"
    from     = [event.payment_provider.payment_captured]
    to       = [processor.payment_translator]
  }

  processor "payment_translator" {
    to = [command.confirm_order]
  }

  command "confirm_order" {
    to = [event.orders.order_confirmed]
  }

  scenario "captured_payment_confirms_order" {
    given { event = event.payment_provider.payment_captured }
    when  { processor = processor.payment_translator }
    then  { event = event.orders.order_confirmed }
  }
}
```

Translation consumes at least one event from a bounded context marked
`external = true` and emits the receiving domain's language. The internal
event is a new fact, not a linked copy of the external one. Do not rename the
external fact and leak it as the internal event. Do not add the internal event
to the translation read model's `from` list as a "closer". The list is a relay
of foreign facts awaiting translation, not a completed-state tracker: membership
is not "not yet recorded on an order". A later worker is a separate `automation`
only when it makes a new decision.

## Failure scenarios

Only add failures supplied or justified by domain rules.

A refused command records nothing. HCL expresses that as an error target, not
as an empty Then and not as `expect_empty_list`:

```hcl
scenario "order_without_lines_is_rejected" {
  when { command = command.place_order }
  then { error = "An order must contain at least one line" }

  comment {
    description = "The command is rejected before an order is created."
  }
}
```

A failure the business tracks is an event (`payment_failed`), specified with
`then { event = ... }`. Do not collapse that into an error string.

Use a hotspot—not a fabricated failure rule—when the outcome is unresolved.
Do not invent `expect_no_dispatch` for "the processor does nothing".

## Information-completeness example

If `order_placed` contains `order_id`, `customer_id`, and `total`, the deciding
command must provide those values, or `generated = true` plus a `description`
must state the derivation. If `order_summary` exposes the same fields, each
must be obtainable from one of its `from` events. Do not encode that lineage
with `mapping`, `session:`, `latest:`, `derived:`, or `aggregate:`. A field
required only by infrastructure does not belong in a domain contract unless it
has contract meaning; mark known technical fields with
`technical_attribute = true`.
