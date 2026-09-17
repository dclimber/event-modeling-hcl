# Canonical workflow patterns

These are shape references, not vocabulary to copy. Each snippet assumes the
referenced catalog declarations exist in the same document.

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

```hcl
bounded_context "notifications" {
  event "order_confirmation_requested" {}
}

automation "request_order_confirmation" {
  processor "confirmation_processor" {
    from = [event.orders.order_placed]
    to   = [command.request_confirmation]
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

Use a Read Model before the Processor when the machine reacts to a question or
to a set of accumulated facts:

```hcl
readmodel "orders_needing_confirmation" {
  question = "Which placed orders still need confirmation?"
  from     = [event.orders.order_placed]
  to       = [processor.confirmation_processor]
}
```

Automation consumes only internal Events.

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
  processor "payment_translator" {
    from = [event.payment_provider.payment_captured]
    to   = [command.confirm_order]
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

Translation consumes at least one Event from a bounded context marked
`external = true` and emits the receiving domain's language. Do not rename the
external fact and leak it as the internal Event.

## Failure scenarios

Only add failures supplied or justified by domain rules:

```hcl
scenario "order_without_lines_is_rejected" {
  when { command = command.place_order }
  then { error = "An order must contain at least one line" }

  comment {
    description = "The command is rejected before an order is created."
  }
}
```

Use a hotspot—not a fabricated failure rule—when the outcome is unresolved.

## Information-completeness example

If `order_placed` contains `order_id`, `customer_id`, and `total`, the deciding
Command must provide those values or make their explicit derivation clear. If
`order_summary` exposes the same fields, each must be obtainable from one of
its `from` Events. A field required only by infrastructure does not belong in a
domain contract unless it has contract meaning; mark known technical fields
with `technical_attribute = true`.
