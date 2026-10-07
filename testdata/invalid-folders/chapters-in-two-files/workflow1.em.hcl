chapter "setup" {
  title     = "Setup"
  workflows = [workflow.add_product]
}

state_change "add_product" {
  title = "Add Product"

  command "add_product_cmd" {
    title     = "Add Product"
    aggregate = aggregate.store.product
    to        = [event.store.product_added]
  }
}
