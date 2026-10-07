hotspot "notification_channel" {
  status   = "open"
  question = "Which notification channel should be used?"
  on       = processor.notify_owner.pet_notification
}
