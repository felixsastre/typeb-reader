# Assumption: the supported actions are NEW, CNL, RPL and TIM. Any other action (EQT, CON, ADM, FLT, REV, SKD) is reported as unsupported and never guessed.
# Assumption: an SSM body has these lines in order: the word SSM, the time mode (UTC or LT), the action, the flight, the period. NEW and RPL then have an equipment line and one routing line per leg. TIM has only the routing lines. CNL has nothing more.
# Assumption: one message carries one flight with one period. A message with more lines than its action allows is rejected.
# Assumption: a flight is an airline designator of 2 characters followed by a number of 1 to 4 digits.
# Assumption: a period line has a start date, an end date and the days of operation, separated by spaces. Dates are written ddMMMyy (05NOV26). The days are digits 1 to 7, with 1 as Monday, each at most once and in ascending order.
# Assumption: an equipment line has a service type (one letter, J is a scheduled passenger flight), a 3-character aircraft type and optionally a configuration. The configuration is kept as written and not interpreted.
# Assumption: a routing line is the departure station and time, a space, and the arrival station and time (MAD0810 BCN0925). A station is 3 letters and a time is hhmm. Times are kept in the time mode stated by the message and are never converted.
# Assumption: a flight with several legs has one routing line per leg, in flight order. The reader does not check that one leg ends where the next begins.

Feature: Reading Standard Schedule Messages
  An airline tells its partners about a change in its timetable with a Standard Schedule
  Message: a new flight, a cancelled one, a replaced one or a change of times. The reader turns
  the body into a flight with its period, days, aircraft and legs, and rejects what is wrong or
  missing instead of filling it in.

  Scenario: A new flight is read with its period, days, aircraft and leg
    Given the message
      """
      QU MADRMXA
      .BCNRMXA 051530
      SSM
      UTC
      NEW
      XA123
      05NOV26 28MAR27 135
      J 738 Y180
      MAD0810 BCN0925
      """
    When the message is read
    Then it is an SSM NEW for flight XA123 in UTC
    And the period runs from 5 Nov 2026 to 28 Mar 2027 on Monday, Wednesday and Friday
    And the aircraft is a 738 on a service of type J with configuration "Y180"
    And the leg goes from MAD at 08:10 to BCN at 09:25

  Scenario: A flight with two legs keeps them in order
    Given the message
      """
      QU MADRMXA
      .BCNRMXA 051530
      SSM
      LT
      NEW
      XA456
      05NOV26 28MAR27 1234567
      J 320
      MAD1000 BCN1115
      BCN1200 PMI1250
      """
    When the message is read
    Then it is an SSM NEW for flight XA456 in LT, operating every day
    And the legs are MAD 10:00 to BCN 11:15, then BCN 12:00 to PMI 12:50
    And there is no configuration

  Scenario: A cancellation names the flight and the period it cancels
    Given the message
      """
      QU MADRMXA
      .BCNRMXA 051530
      SSM
      UTC
      CNL
      XA123
      05NOV26 28MAR27 135
      """
    When the message is read
    Then it is an SSM CNL for flight XA123
    And the period runs from 5 Nov 2026 to 28 Mar 2027 on Monday, Wednesday and Friday
    And there are no legs and no aircraft

  Scenario: A replacement carries the whole flight again
    Given the message
      """
      QU MADRMXA
      .BCNRMXA 051530
      SSM
      UTC
      RPL
      XA123
      05NOV26 28MAR27 135
      J 321
      MAD0810 BCN0925
      """
    When the message is read
    Then it is an SSM RPL for flight XA123
    And the aircraft is a 321 and the leg goes from MAD at 08:10 to BCN at 09:25

  Scenario: A change of times carries the legs and no aircraft
    Given the message
      """
      QU MADRMXA
      .BCNRMXA 051530
      SSM
      UTC
      TIM
      XA123
      05NOV26 28MAR27 135
      MAD0830 BCN0945
      """
    When the message is read
    Then it is an SSM TIM for flight XA123
    And the leg goes from MAD at 08:30 to BCN at 09:45
    And there is no aircraft

  Scenario Outline: The days of operation are read
    Given a valid SSM NEW message whose period line is "05NOV26 28MAR27 <days>"
    When the message is read
    Then the days of operation are <weekdays>

    Examples:
      | days    | weekdays                |
      | 1234567 | every day               |
      | 135     | Monday, Wednesday, Friday |
      | 7       | Sunday                  |

  Scenario Outline: A period line that cannot be right is rejected
    Given a valid SSM NEW message whose period line is "<period>"
    When the message is read
    Then reading fails on line 7
    And the error names "<culprit>"

    Examples:
      | period              | culprit |
      | 31NOV26 28MAR27 135 | 31NOV26 |
      | 05NOV26 28MAR27 138 | 138     |
      | 05NOV26 28MAR27 531 | 531     |
      | 05NOV26 28MAR27 1135| 1135    |
      | 28MAR27 05NOV26 135 | 05NOV26 |

  Scenario: A time that does not exist is rejected
    Given a valid SSM NEW message whose routing line is "MAD2575 BCN0925"
    When the message is read
    Then reading fails on line 9
    And the error names "MAD2575"

  Scenario: A new flight without a routing line is rejected
    Given a valid SSM NEW message without its routing line
    When the message is read
    Then reading fails on line 9
    And the error says a leg is missing

  Scenario Outline: An action or a time mode the reader does not support is rejected
    Given a valid SSM NEW message whose <line> line is "<value>"
    When the message is read
    Then reading fails on line <number>
    And the error names "<value>"

    Examples:
      | line      | value | number |
      | action    | EQT   | 5      |
      | time mode | GMT   | 4      |

  Scenario: A line the action does not allow is rejected
    Given a valid SSM CNL message followed by the line "J 738 Y180"
    When the message is read
    Then reading fails on line 8
    And the error names "J 738 Y180"
