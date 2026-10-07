state_change "register_pet" {
  description = "Register a new pet for an owner."
  owner       = bounded_context.clinic
  status      = "created"

  screen "pet_screen" {
    title  = "Pet screen"
    actor  = actor.clinic_staff
    fields = [field_type.clinic.pet_name]
    to     = [command.register_pet_command]

    field "pet_id" {
    }
  }

  screen_image "pet_form" {
    title = "Pet form"
    url   = "https://example.test/pet-form.png"
  }

  command "register_pet_command" {
    title                  = "Register pet"
    description            = "Records a pet."
    group_id               = "registration"
    tags                   = ["write", "pet"]
    aggregate              = aggregate.clinic.pet
    aggregate_dependencies = [aggregate.clinic.owner]
    api_endpoint           = "POST /pets"
    service                = null
    creates_aggregate      = true
    triggers               = ["submit"]
    list_element           = false
    prototype              = { route = "/pets", method = "POST" }
    sketched               = false
    to                     = [event.clinic.pet_registered]

    field "pet_id" {
      type = field_type.clinic.pet_id
    }

    field "pet_name" {
      type = field_type.clinic.pet_name
    }

    field "request_id" {
      type                = "UUID"
      technical_attribute = true
    }
  }

  table "pets" {
    title = "Pets"

    field "pet_id" {
      type = field_type.clinic.pet_id
    }
  }

  scenario "register_pet_specification" {
    title = "Register a pet"

    given {
      title             = "Owner exists"
      tags              = ["setup"]
      examples          = [{ owner_id = 9 }]
      expect_empty_list = false
      event             = event.clinic.owner_registered
    }

    when {
      title   = "Register pet"
      command = command.register_pet_command

      field "pet_name" {
        type = field_type.clinic.pet_name
      }
    }

    then {
      title = "Pet registered"
      event = event.clinic.pet_registered
    }

    then {
      title             = "Validation error"
      expect_empty_list = true
      error             = "Pet registration is invalid"
    }

    comment {
      description = "The owner relationship is assumed to exist."
    }
  }
}
