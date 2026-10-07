state_change "create_item" {
  title = "Create Item"

  command "create_item_cmd" {
    title     = "Create Item"
    aggregate = aggregate.shared.item
    to        = [event.shared.item_created]
  }
}

