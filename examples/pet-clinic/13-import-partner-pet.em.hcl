translation "import_partner_pet" {
  title       = "Import partner pet"
  description = "Translate the partner contract into the clinic language."

  processor "translate_pet" {
    title = "Translate partner pet"
    from  = [event.partner.pet_received]
    to    = [command.import_pet]
  }

  command "import_pet" {
    title = "Import pet"
    to    = [event.clinic.external_pet_imported]
  }
}
