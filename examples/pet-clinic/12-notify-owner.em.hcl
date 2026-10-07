automation "notify_owner" {
  title  = "Notify owner"
  status = "in_progress"

  readmodel "pets_needing_notification" {
    title    = "Pets needing notification"
    question = "Which pets require an owner notification?"
    from     = [event.clinic.pet_registered]
    to       = [processor.pet_notification]
  }

  processor "pet_notification" {
    title = "Pet notification"
    to    = [command.send_owner_notification]
  }

  command "send_owner_notification" {
    title     = "Send owner notification"
    aggregate = aggregate.clinic.owner
    to        = [event.clinic.external_pet_imported]
  }
}
