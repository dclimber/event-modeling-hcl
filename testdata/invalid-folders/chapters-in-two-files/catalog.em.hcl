bounded_context "store" {
  title = "Store"

  aggregate "product" {
  }

  field_type "id" {
    type         = "Int"
    id_attribute = true
    example      = 1
  }

  event "product_added" {
    title     = "Product Added"
    aggregate = aggregate.product
  }
}
