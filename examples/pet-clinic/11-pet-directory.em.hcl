state_view "pet_directory" {
  title  = "Pet directory"
  status = "done"

  readmodel "pet_summary" {
    title    = "Pet summary"
    question = "Which pets are registered?"
    from     = [event.clinic.pet_registered]
    to       = [screen.pet_summary_screen]

    field "pet_profile" {
      type = field_type.clinic.pet_profile
    }
  }

  screen "pet_summary_screen" {
    title = "Pet summary screen"
    actor = actor.clinic_staff
  }
}
