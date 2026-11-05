# Assumption: an MVT body is the word MVT, a flight line, a primary line and then, in this order and each at most once: a delay line (DL), a passenger line (PX), an estimated on-block line (EB), a leg day line (FLD) and a free text line (SI).
# Assumption: the primary line is a departure (AD), an arrival (AA), an estimated departure (ED) or a next information time (NI). Any other kind of movement message is not supported and is rejected.
# Assumption: a flight line is the flight, a slash, the day of the month, a period, the aircraft registration, a period and the station (XA123/05.ECABC.MAD). The registration is kept as written. The day is the scheduled day of departure, and the reader does not check it against the month or against a day written inside a time.
# Assumption: a time is written hhmm, or ddhhmm when it carries the day of the month. All the times of an MVT are in UTC.
# Assumption: an AD line has the off-block time and then the airborne time (AD0805/0812). An AA line has the touchdown time and then the on-block time (AA0925/0931). Either time can be missing: AD0805 has only the first and AD/0812 only the second. A line with neither time is rejected.
# Assumption: a departure or arrival line may carry on the same line, in this order, an estimated take-off (EO), a return to ramp (RR), an estimated arrival with its airport (EA) and an estimated on-block time (EB).
# Assumption: a delay is DL, a 2-character delay code, a slash and a duration hhmm (DL72/0015). Two delays are DL, two codes and two durations, all separated by slashes (DL64/72/0015/0020). The codes are kept as written and not interpreted.
# Assumption: the estimated on-block time (EB) is written on the movement line or on its own line, never on both. When both are present the message is rejected, even if the two times are equal, and the error is on the own line, which comes second. The reader does not choose between them.
# Assumption: PX is the number of seats taken by passengers: one number, or several separated by slashes, one per destination.

Feature: Reading Movement messages
  An airline or airport reports what an aircraft just did: it left the gate and took off, or it
  landed and reached the gate. Other messages report that a departure is delayed. The reader
  turns the report into the flight, the aircraft, the station and the times, and rejects any
  line it does not understand.

  Scenario: A departure is read with its two times
    Given the message
      """
      QU MADRMXA
      .MADRMXA 050812
      MVT
      XA123/05.ECABC.MAD
      AD0805/0812
      """
    When the message is read
    Then it is a movement of flight XA123 on day 5 by aircraft ECABC at MAD
    And it is a departure, off the gate at 08:05 and airborne at 08:12

  Scenario: An arrival is read with its two times
    Given the message
      """
      QU BCNRMXA
      .BCNRMXA 050931
      MVT
      XA123/05.ECABC.BCN
      AA0925/0931
      """
    When the message is read
    Then it is a movement of flight XA123 on day 5 by aircraft ECABC at BCN
    And it is an arrival, touched down at 09:25 and at the gate at 09:31

  Scenario Outline: A departure can give one time, both, or each with its day
    Given a valid MVT departure whose movement line is "<line>"
    When the message is read
    Then the departure shows off the gate at <off> and airborne at <airborne>

    Examples:
      | line            | off             | airborne        |
      | AD0805          | 08:05           | none            |
      | AD/0812         | none            | 08:12           |
      | AD210805/210812 | 08:05 on day 21 | 08:12 on day 21 |

  Scenario Outline: An arrival can give one time, both, or each with its day
    Given a valid MVT arrival whose movement line is "<line>"
    When the message is read
    Then the arrival shows touchdown at <touchdown> and at the gate at <gate>

    Examples:
      | line            | touchdown       | gate            |
      | AA0925          | 09:25           | none            |
      | AA/0931         | none            | 09:31           |
      | AA220925/220931 | 09:25 on day 22 | 09:31 on day 22 |

  Scenario: A departure carries its estimates on the same line
    Given a valid MVT departure whose movement line is "AD0805/0812 EO0820 EA0925 BCN"
    When the message is read
    Then the estimated take-off is 08:20
    And the estimated arrival is 09:25 at BCN

  Scenario: A departure that returned to the ramp
    Given a valid MVT departure whose movement line is "AD0805 RR0815"
    When the message is read
    Then the return to ramp is at 08:15

  Scenario: An arrival carries its estimated on-block time
    Given a valid MVT arrival whose movement line is "AA0925 EB0935"
    When the message is read
    Then the estimated on-block time is 09:35

  Scenario: The estimated on-block time can be on its own line
    Given a valid MVT arrival followed by the line "EB0935"
    When the message is read
    Then the estimated on-block time is 09:35

  Scenario Outline: An estimated on-block time given twice is rejected
    Given a valid MVT arrival whose movement line is "AA0925 EB0935" and followed by the line "<own line>"
    When the message is read
    Then reading fails on line 6
    And the error names "<own line>"

    Examples:
      | own line |
      | EB0940   |
      | EB0935   |

  Scenario: A delay, the passengers and a note follow the departure
    Given a valid MVT departure followed by the lines
      """
      DL72/0015
      PX183
      SI DEICING
      """
    When the message is read
    Then the delay is code 72 for 15 minutes
    And there are 183 passengers
    And the information is "DEICING"

  Scenario: Two delays are read in pairs
    Given a valid MVT departure followed by the line "DL64/72/0015/0020"
    When the message is read
    Then the delays are code 64 for 15 minutes and code 72 for 20 minutes

  Scenario: Passengers can be given per destination
    Given a valid MVT departure followed by the line "PX12/134/10"
    When the message is read
    Then the passengers per destination are 12, 134 and 10

  Scenario: An arrival can name the day of the flight leg
    Given a valid MVT arrival followed by the line "FLD05"
    When the message is read
    Then the leg day is 5

  Scenario: A delayed departure gives its new estimate
    Given the message
      """
      QU MADRMXA
      .MADRMXA 050700
      MVT
      XA123/05.ECABC.MAD
      ED051130
      DL72/0025
      """
    When the message is read
    Then it is a delay notice with the estimated departure at 11:30 on day 5
    And the delay is code 72 for 25 minutes

  Scenario: An indefinite delay says when more will be known
    Given the message
      """
      QU MADRMXA
      .MADRMXA 050700
      MVT
      XA123/05.ECABC.MAD
      NI051150
      SI ENGINE TROUBLE
      """
    When the message is read
    Then it is a delay notice with the next information at 11:50 on day 5
    And the information is "ENGINE TROUBLE"

  Scenario Outline: The day of the month must exist
    Given a valid MVT departure whose flight line is "XA123/<day>.ECABC.MAD"
    When the message is read
    Then <outcome>

    Examples:
      | day | outcome                   |
      | 01  | the movement is on day 1  |
      | 31  | the movement is on day 31 |
      | 00  | reading fails on line 4   |
      | 32  | reading fails on line 4   |

  Scenario Outline: A movement line that cannot be right is rejected
    Given a valid MVT departure whose movement line is "<line>"
    When the message is read
    Then reading fails on line 5
    And the error names "<line>"

    Examples:
      | line            |
      | AD0875/0812     |
      | AD320805/320812 |
      | AD              |
      | AD0805/         |

  Scenario: A line the grammar knows but not in its place is rejected
    Given a valid MVT departure followed by the lines
      """
      SI DEICING
      DL72/0015
      """
    When the message is read
    Then reading fails on line 7
    And the error names "DL72/0015"

  Scenario: A delay with unequal codes and durations is rejected
    Given a valid MVT departure followed by the line "DL72/0015/0020"
    When the message is read
    Then reading fails on line 6
    And the error names "DL72/0015/0020"

  Scenario: A line of another kind is rejected, never skipped
    Given a valid MVT departure followed by the line "TOF6400"
    When the message is read
    Then reading fails on line 6
    And the error names "TOF6400"

  Scenario: A movement without its movement line is rejected
    Given a valid MVT departure without its movement line
    When the message is read
    Then reading fails on line 5
    And the error says a movement line is missing
