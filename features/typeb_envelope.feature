# Assumption: a message is an envelope followed by a body. The envelope has two lines: a priority line (a priority code and one or more destination addresses) and an origin line (a period, the origin address and a six-digit timestamp).
# Assumption: an address has 7 characters: 3 letters for the location, 2 for the function and 2 for the airline.
# Assumption: the priority codes are QD, QK and QU. Any other code is rejected.
# Assumption: the timestamp is the day, hour and minute of sending (ddhhmm). It has no month or year, and none is invented.
# Assumption: a line has at most 63 characters.
# Assumption: the priority line is read strictly: its fields are separated by exactly one space, with no leading or trailing whitespace, no tab and no empty field. Any other layout is rejected on line 1 before the fields are checked, with the error "fields are separated by exactly one space, with no leading or trailing whitespace".
# Assumption: the whole first body line is the message type, matched exactly. It selects the grammar that describes the rest of the body. A line with more than the type (MVT EXTRA) or with a trailing space (MVT ) is not that type, and is rejected like any type the reader has no grammar for.
# Assumption: lines are counted from 1 at the first line of the whole message, envelope included.
# Assumption: a message type is defined by a grammar alone; adding a type does not change the reader.
# Assumption: a line of the body must match its grammar pattern from its first character to its last. A pattern written without anchors is read as if it were anchored, so a line that only contains the pattern is not accepted.
# Assumption: a message may end with one line break, a line feed or a carriage return and a line feed. It is not a line. Any further line break makes a blank line, and a blank line is rejected like any other line the grammar does not expect.
# Assumption: only a line feed or a carriage return followed by a line feed ends a line. A carriage return in any other place, such as one left at the end of the message or a second one before the line feed, is not a line break: it is part of the line that holds it, that line is rejected, and the carriage return is never stripped. The error names a carriage return.

Feature: Reading the envelope of a Type B message
  Airlines and airports exchange short teletype messages. Each one starts with an envelope that
  says who it is for, who sent it and when, followed by a body whose first line names the kind
  of message. The reader separates the envelope from the body and refuses anything it cannot
  place, so a damaged message is never read as a different one.

  Scenario: The envelope of a message is read
    Given the message
      """
      QU MADRMXA
      .BCNRMXA 051530
      MVT
      XA123/05.ECABC.MAD
      AD0805/0812
      """
    When the message is read
    Then the priority is QU and the destination is MADRMXA
    And the origin is BCNRMXA, sent on day 5 at 15:30
    And the message type is MVT

  Scenario: A message for two destinations
    Given the message
      """
      QK MADRMXA PMIRMXA
      .BCNRMXA 051530
      MVT
      XA123/05.ECABC.MAD
      AD0805/0812
      """
    When the message is read
    Then the destinations are MADRMXA and PMIRMXA

  Scenario Outline: An address that is not 7 characters is rejected
    Given a message whose priority line is "QU <address>"
    When the message is read
    Then reading fails on line 1
    And the error names "<address>"

    Examples:
      | address  |
      | MADRMX   |
      | MADRMXAB |
      | MA1RMXA  |

  Scenario: An unknown priority code is rejected
    Given a message whose priority line is "ZZ MADRMXA"
    When the message is read
    Then reading fails on line 1
    And the error names "ZZ"

  Scenario Outline: A priority line that is not separated by exactly one space is rejected
    Given a message whose priority line has <layout>
    When the message is read
    Then reading fails on line 1
    And the error names "exactly one space"

    Examples:
      | layout                                  |
      | a leading space                         |
      | a trailing space                        |
      | a double space between the fields       |
      | a double space between two destinations |
      | a tab between the fields                |

  Scenario Outline: An origin line that cannot be read is rejected
    Given a message whose origin line is "<origin>"
    When the message is read
    Then reading fails on line 2

    Examples:
      | origin           |
      | BCNRMXA 051530   |
      | .BCNRMXA 051575  |
      | .BCNRMXA 321530  |

  Scenario: A line longer than 63 characters is rejected
    Given a message whose body has a line of 64 characters
    When the message is read
    Then reading fails on line 4
    And the error names "64"

  Scenario: A kind of message the reader has no grammar for is rejected
    Given a message of type XYZ
    When the message is read
    Then reading fails on line 3
    And the error names "XYZ"

  Scenario Outline: A first body line that is not exactly a message type is rejected
    Given a message whose message type line is MVT followed by <tail>
    When the message is read
    Then reading fails on line 3
    And the error names "no grammar for message type"

    Examples:
      | tail               |
      | a space and a word |
      | a space            |

  Scenario: An envelope without a body is not an empty message
    Given a message that has only the envelope
    When the message is read
    Then reading fails on line 3

  Scenario: A new kind of message is added with a grammar alone
    Given a grammar that defines the message type TST as one line holding a flight
    And the message
      """
      QD MADRMXA
      .BCNRMXA 051530
      TST
      XA123
      """
    When the message is read
    Then the message type is TST
    And the flight is XA123

  Scenario: A line that matches a pattern must match it entirely
    Given a grammar that defines the message type TST as one flight line written without anchors
    And the message
      """
      QD MADRMXA
      .BCNRMXA 051530
      TST
      XA123
      """
    When the message is read
    Then the message type is TST
    And the flight is XA123

  Scenario Outline: A line that only contains the pattern is rejected
    Given a grammar that defines the message type TST as one flight line written without anchors
    And a message of type TST whose flight line is "<line>"
    When the message is read
    Then reading fails on line 4
    And the error names "<line>"

    Examples:
      | line            |
      | junk XA123      |
      | XA123 junk      |
      | junk XA123 junk |

  Scenario Outline: One line break at the end of a message is tolerated
    Given a valid MVT departure ending with <ending>
    When the message is read
    Then reading succeeds
    And it is a departure, off the gate at 08:05 and airborne at 08:12

    Examples:
      | ending                            |
      | a line feed                       |
      | a carriage return and a line feed |

  Scenario Outline: A carriage return that is not part of a line break is rejected
    Given a valid MVT departure ending with <ending>
    When the message is read
    Then reading fails on line 5
    And the error names "carriage return"

    Examples:
      | ending                                    |
      | a lone carriage return                    |
      | two carriage returns before the line feed |

  Scenario: A carriage return in the middle of a line is rejected
    Given a valid MVT departure whose movement line has a carriage return in the middle
    When the message is read
    Then reading fails on line 5
    And the error names "carriage return"

  Scenario Outline: Blank lines after the message are rejected
    Given a valid MVT departure ending with <ending>
    When the message is read
    Then reading fails on line 6
    And the error names "unexpected line"

    Examples:
      | ending                              |
      | two line feeds                      |
      | three line feeds                    |
      | two carriage returns and line feeds |

  Scenario Outline: A date rule is skipped when one of its two dates is absent
    Given a grammar that defines the message type PER as one period line with an optional start and an optional end
    And a message of type PER whose period line is "<line>"
    When the message is read
    Then reading succeeds
    And the message type is PER

    Examples:
      | line            |
      | 05NOV26         |
      | /06NOV26        |
      | 05NOV26/06NOV26 |

  Scenario: A date rule still rejects an end before the start when both are given
    Given a grammar that defines the message type PER as one period line with an optional start and an optional end
    And a message of type PER whose period line is "05NOV26/04NOV26"
    When the message is read
    Then reading fails on line 4
    And the error names "04NOV26"
