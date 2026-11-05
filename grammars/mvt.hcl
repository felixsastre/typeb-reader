# Movement message about one flight: a departure (AD), an arrival (AA) or a delay notice (ED, NI),
# followed by optional lines in a fixed order. Any other kind of line is rejected.
#
# A time is written hhmm, or ddhhmm with the day of the month, and is UTC.
message "MVT" {
  line "flight" {
    pattern = "^(?P<flight>[A-Z0-9]{2}[0-9]{1,4})/(?P<day>[0-9]{2})[.](?P<registration>[A-Z0-9]+)[.](?P<station>[A-Z]{3})$"
    kinds   = { day = "day_of_month" }
  }

  # AD: off the gate, then airborne. AA: touched down, then at the gate. Either time may be missing
  # (AD0805, AD/0812). On the same line, in this order: EO estimated take-off, RR return to ramp,
  # EA estimated arrival with its airport, EB estimated on-block. ED and NI start a delay notice.
  line "movement" {
    pattern = "^(?:(?P<code>AD|AA)(?:(?P<first>[0-9]{4}(?:[0-9]{2})?)(?:/(?P<second>[0-9]{4}(?:[0-9]{2})?))?|/(?P<only_second>[0-9]{4}(?:[0-9]{2})?))(?: EO(?P<est_takeoff>[0-9]{4}(?:[0-9]{2})?))?(?: RR(?P<return_ramp>[0-9]{4}(?:[0-9]{2})?))?(?: EA(?P<est_arrival>[0-9]{4}(?:[0-9]{2})?) (?P<est_arrival_airport>[A-Z]{3}))?(?: EB(?P<est_onblock>[0-9]{4}(?:[0-9]{2})?))?|(?P<delay_code>ED|NI)(?P<delay_time>[0-9]{6}))$"
    kinds = {
      first          = "moment"
      second         = "moment"
      only_second    = "moment"
      est_takeoff    = "moment"
      return_ramp    = "moment"
      est_arrival    = "moment"
      est_onblock    = "moment"
      delay_time     = "moment"
    }
  }

  # DL72/0015: a delay code and how long it lasted. Two delays: DL64/72/0015/0020.
  line "delay" {
    pattern  = "^DL(?:(?P<code_1>[0-9]{2})/(?P<duration_1>[0-9]{4})|(?P<pair_code_1>[0-9]{2})/(?P<pair_code_2>[0-9]{2})/(?P<pair_duration_1>[0-9]{4})/(?P<pair_duration_2>[0-9]{4}))$"
    kinds    = { duration_1 = "duration", pair_duration_1 = "duration", pair_duration_2 = "duration" }
    optional = true
  }

  # PX183: seats taken by passengers. PX12/134/10: one number per destination.
  line "passengers" {
    pattern  = "^PX(?P<passengers>[0-9]{1,3}(?:/[0-9]{1,3})*)$"
    optional = true
  }

  # EB may also be on the movement line, but not on both: the message is rejected on this line.
  line "estimated_on_block" {
    pattern     = "^EB(?P<est_onblock_line>[0-9]{4}(?:[0-9]{2})?)$"
    kinds       = { est_onblock_line = "moment" }
    optional    = true
    at_most_one = ["est_onblock", "est_onblock_line"]
  }

  line "leg_day" {
    pattern  = "^FLD(?P<leg_day>[0-9]{2})$"
    kinds    = { leg_day = "day_of_month" }
    optional = true
  }

  # SI: free text.
  line "information" {
    pattern  = "^SI (?P<information>.+)$"
    optional = true
  }
}
