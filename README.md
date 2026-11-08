# typeb-reader

Airlines and airports still send each other short text messages over airline networks that began in the teletype era. These are called Type B messages. One of them tells a partner that a flight now leaves at a different time. Another says that a plane has just taken off. This library reads those messages and gives back the same simple result every time: the flight, the days it runs, the aircraft and the times.

It is a proof of concept written in Go. It is small and tested with invented messages. Supporting a new kind of message means adding a file.

> [!IMPORTANT]
> This is a proof of concept, made for research and out of curiosity about how airline
> distribution works. It is not prepared for production use, and it should not be used
> with real bookings, fares or traffic.

## Words used here

- Type B message: a short airline teletype message. It starts with who it is for, who sent it and when, and then comes the content.
- SSM: Standard Schedule Message, which announces a new flight, a cancelled one, a replaced one or a change of times.
- MVT: Movement message, which reports that an aircraft left the gate and took off, or landed and reached the gate, or that a departure is delayed.
- Grammar: a short file that says which lines a kind of message has and what each line looks like.

## What it does

Given a message, it returns plain values. This message announces a new flight:

```
QU MADRMXA
.BCNRMXA 051530
SSM
UTC
NEW
XA123
05NOV26 28MAR27 135
J 738 Y180
MAD0810 BCN0925
```

and the library reads it as: flight XA123, from 5 November 2026 to 28 March 2027 on Monday, Wednesday and Friday, a 738 aircraft, leaving Madrid at 08:10 and arriving in Barcelona at 09:25.

In code it takes a few lines:

```go
reader := typebreader.NewReader()
message, err := reader.Read(text)
ssm, err := typebreader.ReadSSM(message)
```

## It does not guess

When a message is wrong or incomplete, the library says so and names the line. It reports:

- an address, a date or a time that does not exist, such as 31 November or 25:75;
- a flight with no leg, which is never read as a flight that goes nowhere;
- a line it has no rule for, which is never skipped, and a line that only partly fits its rule;
- the same fact written twice in a movement message, such as the estimated on-block time, instead of choosing one;
- a kind of message it does not know, and an envelope with nothing after it;
- a first line with extra or odd spacing, such as a double space or a tab, and a carriage return that is not part of a line break, which it never strips quietly.

## Why there are grammar files

Each kind of message has its own lines, so each one gets a grammar file, and one piece of code reads them all. Adding a kind of message means writing a new file. The two that exist are `grammars/ssm.hcl` and `grammars/mvt.hcl`.

## What is in the repository

- `features/` describes the behavior in plain English. Each scenario there is also an automated test.
- `grammars/` has one file per kind of message.
- The `.go` files are the code.
- `REFERENCE.md` has the technical detail for developers.

## What it supports today

| Message | Actions |
| --- | --- |
| SSM | NEW, CNL, RPL, TIM |
| MVT | departure (AD), arrival (AA) and delay notice (ED, NI), with estimates, delays, passengers and notes |

It does not read the other schedule actions (EQT, CON, ADM, FLT, REV, SKD), the ad hoc schedule message (ASM) or the availability message (AVS). It reads one flight and one period per message. For MVT it reads only the elements listed under Limits in [`REFERENCE.md`](REFERENCE.md) and rejects every other one.

## About the data

Every message in this project is invented. The formats are inspired by public examples and by general knowledge of the industry. They have not been checked against the IATA manuals, which are paid, so this is not a compatible implementation and should not be used with real traffic.
