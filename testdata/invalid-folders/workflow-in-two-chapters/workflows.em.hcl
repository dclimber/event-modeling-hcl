chapter "chapter1" {
  title     = "Chapter 1"
  workflows = [workflow.place_order]
}

chapter "chapter2" {
  title     = "Chapter 2"
  workflows = [workflow.place_order]
}

state_change "place_order" {
  title = "Place Order"

  command "place_order_cmd" {
    title     = "Place Order"
    aggregate = aggregate.order.order
    to        = [event.order.order_placed]
  }
}
