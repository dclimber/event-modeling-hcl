bounded_context "shared" {
  title = "Shared"

  aggregate "item" {
  }

  field_type "id" {
    type         = "Int"
    id_attribute = true
    example      = 1
  }

  event "item_created" {
    title     = "Item Created"
    aggregate = aggregate.item
  }
}
