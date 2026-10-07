bounded_context "registrar" {
  title    = "Registrar"
  external = true

  event "student_registered" {
    title = "Student Registered"

    field "student_id" {
      type         = "String"
      id_attribute = true
      example      = "STU-2026-0042"
    }

    field "name" {
      type    = "String"
      example = "Anna Müller"
    }

    field "course_limit" {
      type    = "Int"
      example = 2
    }
  }
}

bounded_context "course_subscriptions" {
  title = "Course Subscriptions"

  aggregate "course" {
    title = "Course"
  }

  aggregate "enrollment" {
    title = "Enrollment"
  }

  field_type "student_id" {
    type         = "String"
    id_attribute = true
    example      = "STU-2026-0042"
  }

  field_type "name" {
    type    = "String"
    example = "Anna Müller"
  }

  field_type "course_limit" {
    type    = "Int"
    example = 2
  }

  field_type "course_id" {
    type         = "String"
    id_attribute = true
    example      = "EM-2024-001"
  }

  field_type "capacity" {
    type    = "Int"
    example = 10
  }

  field_type "title" {
    type    = "String"
    example = "Intro to Event Modeling"
  }

  field_type "number_of_subscriptions" {
    type    = "Int"
    example = 3
  }

  field_type "subscription_count" {
    type    = "Int"
    example = 2
  }

  field_type "subscribed_courses" {
    cardinality = "List"
    type        = "String"
    example     = ["EM-2024-001", "EM-2024-002"]
  }

  event "student_registered" {
    title     = "Student Registered"
    aggregate = aggregate.enrollment

    field "student_id" {
    }

    field "name" {
    }

    field "course_limit" {
    }
  }

  event "course_registered" {
    title     = "Course Registered"
    aggregate = aggregate.course

    field "course_id" {
    }

    field "title" {
    }

    field "capacity" {
    }
  }

  event "course_capacity_changed" {
    title     = "Course Capacity Changed"
    aggregate = aggregate.course

    field "course_id" {
    }

    field "capacity" {
    }
  }

  event "student_subscribed" {
    title     = "Student Subscribed"
    aggregate = aggregate.enrollment

    field "course_id" {
    }

    field "student_id" {
    }
  }

  event "student_unsubscribed" {
    title     = "Student Unsubscribed"
    aggregate = aggregate.enrollment

    field "course_id" {
    }

    field "student_id" {
    }
  }
}

translation "register_student" {
  title = "Register Student"

  readmodel "student_to_register" {
    title    = "Student to Register"
    question = "Which students do I need to register?"
    from     = [event.registrar.student_registered]
    to       = [processor.register_student]

    field "student_id" {
    }

    field "name" {
    }

    field "course_limit" {
    }
  }

  processor "register_student" {
    title = "Register Student"
    to    = [command.register_student]
  }

  command "register_student" {
    title = "Register Student"
    to    = [event.course_subscriptions.student_registered]

    field "student_id" {
    }

    field "name" {
    }

    field "course_limit" {
    }
  }

  scenario "register_student_happy_path" {
    title = "Register student - happy path"

    when {
      command = command.register_student
    }

    then {
      event = event.course_subscriptions.student_registered
    }
  }

  scenario "student_already_registered" {
    title = "Student already registered"

    given {
      event = event.course_subscriptions.student_registered
    }

    when {
      command = command.register_student
    }

    then {
      error = "Student already registered"
    }
  }
}

actor "course_manager" {
  title         = "Course Manager"
  auth_required = true
}

state_change "register_course" {
  title = "Register Course"

  screen "register_new_course" {
    title = "Register New Course"
    actor = actor.course_manager
    to    = [command.register_course]
  }

  command "register_course" {
    title             = "Register Course"
    aggregate         = aggregate.course_subscriptions.course
    creates_aggregate = true
    to                = [event.course_subscriptions.course_registered]

    field "course_id" {
    }

    field "title" {
    }

    field "capacity" {
    }
  }

  scenario "course_registered_happy_path" {
    title = "Course registered — happy path"

    when {
      command = command.register_course
    }

    then {
      event = event.course_subscriptions.course_registered
    }
  }

  scenario "course_already_registered" {
    title = "Course already registered"

    given {
      event = event.course_subscriptions.course_registered
    }

    when {
      command = command.register_course
    }

    then {
      error = "Course already registered"
    }
  }
}

actor "student" {
  title         = "Student"
  auth_required = true
}

state_view "course_catalog" {
  title = "Course Catalog"

  readmodel "course_catalog" {
    title    = "Course Catalog"
    question = "Which courses are in the catalog, with their capacity and subscription counts?"
    from = [
      event.course_subscriptions.course_registered,
      event.course_subscriptions.course_capacity_changed,
      event.course_subscriptions.student_subscribed,
      event.course_subscriptions.student_unsubscribed,
    ]
    to = [screen.course_catalog]

    field "course_id" {
    }

    field "title" {
    }

    field "capacity" {
    }

    field "number_of_subscriptions" {
    }
  }

  screen "course_catalog" {
    title = "Course Catalog"
    actor = actor.course_manager
  }

  scenario "courses_are_shown_with_title_and_capacity" {
    title = "Courses are shown with their title and capacity"

    given {
      event = event.course_subscriptions.course_registered
    }

    then {
      readmodel = readmodel.course_catalog
    }
  }

  scenario "empty_catalogue" {
    title = "Empty catalogue"

    given {
      event = event.registrar.student_registered
    }

    then {
      readmodel = readmodel.course_catalog
    }

    comment {
      description = "No course has been registered yet; the catalogue is empty."
    }
  }

  scenario "account_for_changes_in_capacity" {
    title = "Account for changes in capacity"

    given {
      event = event.course_subscriptions.course_registered
    }

    given {
      event = event.course_subscriptions.course_capacity_changed
    }

    then {
      readmodel = readmodel.course_catalog
    }
  }

  scenario "subscriptions_are_counted" {
    title = "Subscriptions are counted"

    given {
      event = event.course_subscriptions.course_registered
    }

    given {
      event = event.course_subscriptions.student_subscribed
    }

    then {
      readmodel = readmodel.course_catalog
    }
  }
}

state_view "available_courses" {
  title = "Available Courses"

  readmodel "available_courses" {
    title    = "Available Courses"
    question = "Which courses can I subscribe to?"
    from = [
      event.course_subscriptions.course_registered,
      event.course_subscriptions.course_capacity_changed,
      event.course_subscriptions.student_subscribed,
      event.course_subscriptions.student_unsubscribed,
    ]
    to = [screen.available_courses]

    field "course_id" {
    }

    field "title" {
    }

    field "capacity" {
    }

    field "number_of_subscriptions" {
    }
  }

  screen "available_courses" {
    title = "Available Courses"
    actor = actor.student
  }

  scenario "courses_are_shown_with_title_and_capacity" {
    title = "Courses are shown with their title and capacity"

    given {
      event = event.course_subscriptions.course_registered
    }

    then {
      readmodel = readmodel.available_courses
    }
  }

  scenario "empty_catalogue" {
    title = "Empty catalogue"

    given {
      event = event.registrar.student_registered
    }

    then {
      readmodel = readmodel.available_courses
    }

    comment {
      description = "No course has been registered yet; the catalogue is empty."
    }
  }

  scenario "account_for_changes_in_capacity" {
    title = "Account for changes in capacity"

    given {
      event = event.course_subscriptions.course_registered
    }

    given {
      event = event.course_subscriptions.course_capacity_changed
    }

    then {
      readmodel = readmodel.available_courses
    }
  }

  scenario "subscriptions_are_counted" {
    title = "Subscriptions are counted"

    given {
      event = event.course_subscriptions.course_registered
    }

    given {
      event = event.course_subscriptions.student_subscribed
    }

    then {
      readmodel = readmodel.available_courses
    }
  }
}

state_change "change_course_capacity" {
  title = "Change Course Capacity"

  screen "change_course_capacity" {
    title = "Change Course Capacity"
    actor = actor.course_manager
    to    = [command.change_course_capacity]
  }

  command "change_course_capacity" {
    title     = "Change Course Capacity"
    aggregate = aggregate.course_subscriptions.course
    to        = [event.course_subscriptions.course_capacity_changed]

    field "course_id" {
    }

    field "capacity" {
    }
  }

  scenario "capacity_changed_happy_path" {
    title = "Capacity changed — happy path"

    given {
      event = event.course_subscriptions.course_registered
    }

    when {
      command = command.change_course_capacity
    }

    then {
      event = event.course_subscriptions.course_capacity_changed
    }
  }

  scenario "unknown_course" {
    title = "Unknown course"

    when {
      command = command.change_course_capacity
    }

    then {
      error = "Unknown course"
    }
  }

  scenario "same_capacity_rejected" {
    title = "Same capacity — rejected"

    given {
      event = event.course_subscriptions.course_registered
    }

    when {
      command = command.change_course_capacity
    }

    then {
      error = "New capacity must differ from the current capacity"
    }
  }

  scenario "cannot_reduce_capacity_below_current_subscriptions" {
    title = "Cannot reduce capacity below current subscriptions"

    given {
      event = event.course_subscriptions.course_registered
    }

    given {
      event = event.course_subscriptions.student_subscribed
    }

    when {
      command = command.change_course_capacity
    }

    then {
      error = "Capacity cannot be reduced below the current number of subscriptions"
    }
  }
}

state_change "confirm_subscription" {
  title = "Confirm Subscription"

  screen "confirm_subscription" {
    title = "Confirm Subscription"
    actor = actor.student
    to    = [command.subscribe_student]
  }

  command "subscribe_student" {
    title     = "Subscribe Student"
    aggregate = aggregate.course_subscriptions.enrollment
    to        = [event.course_subscriptions.student_subscribed]

    field "course_id" {
    }

    field "student_id" {
    }
  }

  scenario "subscribed_happy_path" {
    title = "Subscribed — happy path"

    given {
      event = event.course_subscriptions.course_registered
    }

    when {
      command = command.subscribe_student
    }

    then {
      event = event.course_subscriptions.student_subscribed
    }
  }

  scenario "unknown_course" {
    title = "Unknown course"

    when {
      command = command.subscribe_student
    }

    then {
      error = "Unknown course"
    }
  }

  scenario "course_is_full" {
    title = "Course is full (capacity 2 → 3rd rejected)"

    given {
      event = event.course_subscriptions.course_registered
    }

    given {
      event = event.course_subscriptions.course_capacity_changed
    }

    when {
      command = command.subscribe_student
    }

    then {
      error = "Course is full"
    }
  }

  scenario "already_subscribed" {
    title = "Already subscribed"

    given {
      event = event.course_subscriptions.course_registered
    }

    given {
      event = event.course_subscriptions.student_subscribed
    }

    when {
      command = command.subscribe_student
    }

    then {
      error = "Student already subscribed to this course"
    }
  }

  scenario "student_at_subscription_limit" {
    title = "Student at subscription limit (2)"

    given {
      event = event.course_subscriptions.student_registered
    }

    given {
      event = event.course_subscriptions.course_registered
    }

    when {
      command = command.subscribe_student
    }

    then {
      error = "Student has reached their subscription limit"
    }
  }
}

state_view "my_subscribed_courses" {
  title = "My Subscribed Courses"

  readmodel "student_course_subscriptions" {
    title    = "Student Course Subscriptions"
    question = "Which courses am I subscribed to?"
    from = [
      event.course_subscriptions.student_registered,
      event.course_subscriptions.student_subscribed,
      event.course_subscriptions.student_unsubscribed,
    ]
    to = [screen.my_subscribed_courses]

    field "student_id" {
    }

    field "subscription_count" {
    }

    field "course_limit" {
    }

    field "courses" {
      type = field_type.course_subscriptions.subscribed_courses
    }
  }

  screen "my_subscribed_courses" {
    title = "My Subscribed Courses"
    actor = actor.student
  }

  scenario "no_course_subscriptions" {
    title = "No course subscriptions"

    given {
      event = event.course_subscriptions.student_registered
    }

    then {
      readmodel = readmodel.student_course_subscriptions
    }

    comment {
      description = "The student has registered but has not subscribed to any course yet."
    }
  }

  scenario "one_subscription" {
    title = "One subscription"

    given {
      event = event.course_subscriptions.student_registered
    }

    given {
      event = event.course_subscriptions.student_subscribed
    }

    then {
      readmodel = readmodel.student_course_subscriptions
    }
  }

  scenario "two_subscriptions" {
    title = "Two subscriptions"

    given {
      event = event.course_subscriptions.student_registered
    }

    given {
      event = event.course_subscriptions.student_subscribed
    }

    given {
      event = event.course_subscriptions.student_subscribed
    }

    then {
      readmodel = readmodel.student_course_subscriptions
    }
  }

  scenario "unsubscribed" {
    title = "Unsubscribed"

    given {
      event = event.course_subscriptions.student_subscribed
    }

    given {
      event = event.course_subscriptions.student_unsubscribed
    }

    then {
      readmodel = readmodel.student_course_subscriptions
    }
  }
}

state_change "unsubscribe_student" {
  title = "Unsubscribe Student"

  screen "my_course_subscription" {
    title = "My Course Subscription"
    actor = actor.student
    to    = [command.unsubscribe_student]
  }

  command "unsubscribe_student" {
    title     = "Unsubscribe Student"
    aggregate = aggregate.course_subscriptions.enrollment
    to        = [event.course_subscriptions.student_unsubscribed]

    field "course_id" {
    }

    field "student_id" {
    }
  }

  scenario "unsubscribed_happy_path" {
    title = "Unsubscribed — happy path"

    given {
      event = event.course_subscriptions.course_registered
    }

    given {
      event = event.course_subscriptions.student_subscribed
    }

    when {
      command = command.unsubscribe_student
    }

    then {
      event = event.course_subscriptions.student_unsubscribed
    }
  }

  scenario "unknown_course" {
    title = "Unknown course"

    when {
      command = command.unsubscribe_student
    }

    then {
      error = "Unknown course"
    }
  }

  scenario "not_subscribed" {
    title = "Not subscribed"

    given {
      event = event.course_subscriptions.course_registered
    }

    when {
      command = command.unsubscribe_student
    }

    then {
      error = "Student is not subscribed to this course"
    }
  }
}
