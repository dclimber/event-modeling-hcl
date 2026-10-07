state_change "add_pet" {
  title = "Add Pet"

  screen "add_pet_form" {
    title = "Add Pet Form"
    to    = [command.add_pet_command]
  }

  command "add_pet_command" {
    title     = "Add Pet"
    aggregate = aggregate.pet_management.pet
    to        = [event.pet_management.pet_added]

    field "pet_id" {
      type = field_type.pet_management.pet_id
    }
  }
}
