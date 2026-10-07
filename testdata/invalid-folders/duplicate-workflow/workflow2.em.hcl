state_change "create_item" {
  title = "Create Item (Duplicate)"

  command "duplicate_cmd" {
    title     = "Create Item (Dup)"
    aggregate = aggregate.shared.item
    to        = [event.shared.item_created]
  }
}
