bounded_context "order" {
  title = "Order"

  aggregate "order" {
  }

  field_type "id" {
    type         = "Int"
    id_attribute = true
    example      = 1
  }

  event "order_placed" {
    title     = "Order Placed"
    aggregate = aggregate.order
  }
}
