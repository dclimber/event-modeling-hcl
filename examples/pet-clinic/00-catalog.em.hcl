team "clinic_team" {
  title = "Clinic team"
}

system "partner_system" {
  title    = "Partner system"
  external = true
}

actor "clinic_staff" {
  title         = "Clinic staff"
  auth_required = true
}

bounded_context "clinic" {
  title       = "Clinic"
  description = "Pet clinic operations and records."
  owner       = team.clinic_team

  aggregate "pet" {
    title       = "Pet"
    description = "A registered animal patient."
  }

  aggregate "owner" {
    title = "Owner"
  }

  field_type "pet_id" {
    cardinality  = "Single"
    type         = "UUID"
    id_attribute = true
    example      = "07917485-4a6b-4e4e-8920-92ce6062a129"
  }

  field_type "pet_name" {
    mapping             = "pet.name"
    schema              = "PetName"
    type                = "String"
    generated           = false
    id_attribute        = false
    optional            = false
    pii                 = true
    technical_attribute = false
    example             = "Mochi"
  }

  field_type "vaccinated" {
    type    = "Boolean"
    example = true
  }

  field_type "weight" {
    type    = "Double"
    example = 4.2
  }

  field_type "fee" {
    type    = "Decimal"
    example = 12.50
  }

  field_type "microchip" {
    type    = "Long"
    example = 1234567890
  }

  field_type "birth_date" {
    type    = "Date"
    example = "2020-01-12"
  }

  field_type "registered_at" {
    type    = "DateTime"
    example = "2025-01-12T10:30:00Z"
  }

  field_type "pet_profile" {
    cardinality = "List"
    type        = "Custom"
    example = [{
      id   = "07917485-4a6b-4e4e-8920-92ce6062a129"
      name = "Mochi"
    }]

    subfield "id" {
      type         = "UUID"
      id_attribute = true
    }

    subfield "name" {
      type = "String"
    }
  }

  event "pet_registered" {
    title                  = "Pet Registered"
    description            = "Records that a pet was registered."
    group_id               = "registration"
    tags                   = ["event", "pet"]
    aggregate              = aggregate.pet
    aggregate_dependencies = [aggregate.owner]
    service                = null
    list_element           = false
    prototype              = { topic = "clinic.pet_registered" }
    sketched               = false

    field "pet_id" {
      type = field_type.pet_id
    }

    field "pet_name" {
      type = field_type.pet_name
    }

    field "registered_at" {
      type = field_type.registered_at
    }
  }

  event "external_pet_imported" {
    field "pet_id" {
      type = field_type.pet_id
    }
  }

  event "owner_registered" {
  }
}

bounded_context "partner" {
  title    = "Partner"
  owner    = system.partner_system
  external = true

  event "pet_received" {
    title = "Pet Received"

    field "pet_id" {
      type = field_type.clinic.pet_id
    }
  }
}
