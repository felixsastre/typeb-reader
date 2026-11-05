# Standard Schedule Message: NEW, CNL, RPL and TIM for one flight and one period.
#
# The format of this file is described in REFERENCE.md, "The grammar format".
#
# The lines come in this order: time mode, action, flight, period, equipment, legs. The equipment
# line is expected only for NEW and RPL, and the leg lines (at least one) only for NEW, RPL and TIM,
# so a CNL ends with the period line.
message "SSM" {
  line "time_mode" { pattern = "^(?P<time_mode>UTC|LT)$" }
  line "action" { pattern = "^(?P<action>NEW|CNL|RPL|TIM)$" }
  line "flight" { pattern = "^(?P<flight>[A-Z0-9]{2}[0-9]{1,4})$" }

  line "period" {
    pattern    = "^(?P<start>[0-9]{2}[A-Z]{3}[0-9]{2}) (?P<end>[0-9]{2}[A-Z]{3}[0-9]{2}) (?P<days>[0-9]+)$"
    kinds      = { start = "date", end = "date", days = "weekdays" }
    not_before = { end = "start" }
  }

  line "equipment" {
    pattern = "^(?P<service>[A-Z]) (?P<aircraft>[A-Z0-9]{3})(?: (?P<configuration>.+))?$"
    when {
      capture = "action"
      in      = ["NEW", "RPL"]
    }
  }

  line "leg" {
    pattern = "^(?P<from>[A-Z]{3})(?P<dep>[0-9]{4}) (?P<to>[A-Z]{3})(?P<arr>[0-9]{4})$"
    kinds   = { dep = "time", arr = "time" }
    repeat  = true
    when {
      capture = "action"
      in      = ["NEW", "RPL", "TIM"]
    }
  }
}
