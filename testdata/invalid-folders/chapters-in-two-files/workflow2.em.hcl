chapter "checkout" {
  title     = "Checkout"
  workflows = [workflow.checkout_order]
}

state_change "checkout_order" {
  title = "Checkout Order"

  command "checkout_cmd" {
    title     = "Checkout"
    aggregate = aggregate.store.product
    to        = [event.store.product_added]
  }
}
