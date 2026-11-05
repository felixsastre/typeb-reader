# typeb-reader: reference

Technical detail for developers.

## Commands

```
go test -race ./...
go vet ./...
gofmt -l .
go test -run '^$' -bench . -benchtime 2s
go test -run '^$' -fuzz FuzzRead -fuzztime 60s
```

## Layout

| File | What it holds |
| --- | --- |
| `reader.go` | `Reader`, `Message`, `ParseError`, the envelope and the engine that reads a body |
| `grammar.go` | decoding and checking of the grammar files |
| `kinds.go` | the kinds a grammar can name (`date`, `time`, `weekdays`, `day_of_month`, `moment`, `duration`) and the `TimeOfDay` and `Moment` types they read |
| `ssm.go`, `mvt.go` | `ReadSSM`, `ReadMVT`: typed values taken from a message already read |
| `grammars/*.hcl` | one grammar per kind of message, embedded in the binary |
| `features/` | the specs, run by godog |

The Go version is in `go.mod`. Dependencies: `hashicorp/hcl/v2` for the grammar files and `cucumber/godog` for the feature tests.

## The envelope

A message is an envelope and a body. The envelope has two lines.

1. The priority line: a code (`QD`, `QK` or `QU`) and one or more destination addresses. The fields are separated by exactly one space. A leading or trailing space, two spaces in a row, a tab or any other whitespace is rejected on line 1, before the fields are checked, with `fields are separated by exactly one space, with no leading or trailing whitespace`.
2. The origin line: a period, the origin address and six digits, `ddhhmm`.

An address has 7 characters: 3 letters of location, 2 letters of function and 2 characters of airline. The timestamp carries no month or year, so none is invented. Lines have at most 63 characters. Lines are counted from 1 at the first line of the message, envelope included, and every error carries that number.

Only LF, or CR followed by LF, ends a line. Exactly one CR is dropped, the one of a CRLF. A message may end with one line break, which is not a line. A second break makes a blank line, and a blank line is rejected like any line the grammar does not expect (`unexpected line ""`, on its number). A CR anywhere else (a lone one at the end of the message, two before an LF, one in the middle of a line) is never stripped: the line that holds it is rejected, in the envelope or in the body, with `carriage return outside a line break` and its number.

The first body line is the message type. It is the whole line, matched exactly against a known grammar: `MVT EXTRA` and `MVT ` are not `MVT`, and fail on line 3 with `no grammar for message type`.

## The grammar format

```hcl
message "SSM" {
  line "period" {
    pattern    = "^(?P<start>[0-9]{2}[A-Z]{3}[0-9]{2}) (?P<end>[0-9]{2}[A-Z]{3}[0-9]{2}) (?P<days>[0-9]+)$"
    kinds      = { start = "date", end = "date", days = "weekdays" }
    not_before = { end = "start" }
  }
}
```

A message type is a list of lines read in order. A line has:

| Attribute | Meaning |
| --- | --- |
| `pattern` | a regular expression with named groups; each group becomes a value of the line |
| `kinds` | checks a group: `date` (05NOV26), `time` (0810), `weekdays` (digits 1 to 7, ascending, each once), `day_of_month` (01 to 31), `moment` (hhmm or ddhhmm), `duration` (hhmm, up to 59 minutes) |
| `not_before` | `{ end = "start" }`: the date in `end` must not be before the date in `start`; both must be `date` groups. The rule is skipped when either group took no part in the match (an optional date that is absent) |
| `repeat` | the line may appear several times, at least once |
| `optional` | the line may be absent; if it is present it must match |
| `when` | a block with `capture` and `in`: the line is only expected if an earlier group has one of the listed values |
| `at_most_one` | a list of group names: at most one of them may be given (non-empty) in the whole message. The line that gives a second one is rejected, with its own number |

The pattern is matched against the whole line. The engine compiles it as `^(?:pattern)$`, so a pattern written without anchors cannot accept a line that only contains it, and a pattern that already has `^` and `$` works the same. The pattern is compiled as written first, so a load error quotes what the author wrote.

`at_most_one` is how a grammar says that one fact may be written in two places but not in both. In `grammars/mvt.hcl`, the estimated on-block time can be on the movement line (`est_onblock`) or on its own line (`est_onblock_line`); `at_most_one = ["est_onblock", "est_onblock_line"]` on the own line rejects a message that has both, even when the times are equal. The engine knows only the names the grammar gives it.

A grammar is checked when it is loaded: a pattern that does not compile, an unknown kind, a kind on a name that is not a group, a `not_before` on a group that is not a date, a `when` that names a group no earlier line captures (the empty name included), an `at_most_one` with fewer than two different names, a name listed twice, a name that neither the line nor an earlier line captures, or no name on the line itself, a message with no lines and a source with no `message` block at all are all errors. A line the grammar does not expect is an error at read time, never skipped. Regular expressions use Go's `regexp`, which runs in time linear in the input.

A new kind of message is `Reader.AddGrammar(text)`. A built-in kind cannot be redefined. The zero `Reader` (`var r Reader`) works: it has no built-in grammars, so it knows only what `AddGrammar` gives it, and `Read` on it fails with `no grammar for message type`.

## Public API

| Name | What it does |
| --- | --- |
| `NewReader()` | returns a `Reader` that knows `SSM` and `MVT` |
| `(*Reader).AddGrammar(text)` | adds the message types a grammar file defines |
| `(*Reader).Read(text)` | reads the envelope and checks the body; returns `*Message` or `*ParseError` |
| `(*Message).Value(name)` | the first non-empty value captured under `name`, or an empty string |
| `ReadSSM(message)`, `ReadMVT(message)` | typed values from a message of that type; a failure is a `*ParseError` |

`ParseError` has the `Line` and a `Detail` that quotes the text that failed. `ReadSSM` and `ReadMVT` report the line of the value that failed to convert, and the line of the message type (line 3) when the message is of the other type. Given a message that was never read (`nil`, or the zero `Message`) they report line 1, the closest line there is, instead of panicking. When they fail they return the zero `SSM` or `MVT`. A line that fails a kind reports the value and the whole line, for example `invalid time "2575" in "MAD2575 BCN0925"`.

## Tests

- `features/`: 51 scenarios, 86 once the outlines are expanded (336 steps).
- Unit tests for the kinds, for the checking of grammars, for the anchoring of patterns, for the first error reported (repeated 200 times, since it once depended on map order), for the line of the typed readers' errors, for `not_before` with an absent date, for the zero `Reader`, for a source with no message, for the zero value returned on error and for the carriage return.
- Property tests (`testing/quick`): the number and order of legs read equals the number written; the days read equal the days written; a time written with its day reads back the same.
- Fuzz test `FuzzRead`: reading never panics, a failure is always a `*ParseError` on a line from 1, a message that reads can also be converted to its typed value, and the typed reader of the other type refuses it with a `*ParseError`.

## Measurements

On an Intel i5-8257U at 1.4 GHz, one core, `go test -bench . -benchtime 2s`:

| Benchmark | Time | Memory | Allocations |
| --- | --- | --- | --- |
| `ReadSSM` (9 lines) | 7.3 µs | 3.3 KB | 32 |
| `ReadMVT` (5 lines) | 3.7 µs | 1.4 KB | 13 |
| `NewReader` | 1.5 ms | 713 KB | 6,431 |

About 137,000 SSM messages per second on one core. `NewReader` is paid once. The grammar files are decoded only there, so the cost of HCL is not in the reading path. The regular expressions are the likely cost, but it has not been profiled.

## Limits

- One flight and one period per message. A real SSM can carry several.
- The MVT elements were checked against a public description of the IATA movement message elements, not against the manual. Only the elements in `grammars/mvt.hcl` are read: AD, AA, ED and NI, with EO, RR (return to ramp), EA and EB on the movement line, then DL, PX, EB on its own line, FLD and SI. Every other element is rejected, for example fuel, weights, crew report time, a return from airborne and a revised arrival estimate. The lines after the movement line must come in the order delay, passengers, estimated on-block, leg day, information.
- The envelope was written from a description of the format, not from the IATA specification.
- The configuration of the cabin in an SSM equipment line is kept as written, not interpreted.
- A multi-leg flight has one routing line per leg, and the reader does not check that one leg ends where the next begins.
- Times are kept in the time mode of the message (UTC or LT) and never converted.
- Only ASCII is expected. A line is counted in characters, not bytes.
