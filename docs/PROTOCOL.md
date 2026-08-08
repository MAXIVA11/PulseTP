# PulseTP Protocol Specification

PulseTP is a UDP-based protocol that encodes information in the *timing*
between packets, not their contents. This document specifies the wire
format, timing rules, and receiver state machine precisely enough to
reimplement the protocol from scratch.

## 1. Transport

PulseTP runs directly over UDP. UDP is chosen specifically because it does
not buffer, coalesce, retransmit, or reorder packets the way a TCP stack
does — all of which would corrupt inter-packet timing before it ever
reached the application. PulseTP has no notion of a connection: a sender
simply fires pulses at a destination `host:port`, and a listener decodes
whatever arrives at a bound port.

## 2. The pulse packet

Every packet on the wire is a **pulse**: a fixed 15-byte struct with no
message content.

| Offset | Size | Field    | Description                                   |
|-------:|-----:|----------|------------------------------------------------|
| 0      | 2    | Magic    | ASCII `"PT"` — identifies a PulseTP pulse       |
| 2      | 1    | Version  | Protocol version, currently `1`                |
| 3      | 4    | Seq      | Sequence number, big-endian `uint32`, starts at 0 |
| 7      | 8    | SentAt   | Send timestamp, big-endian `int64`, Unix nanoseconds |

`Seq` and `SentAt` are diagnostic only — they let a receiver detect dropped
packets and measure clock skew, but **no bit of the message is ever encoded
in packet contents.** The entire signal is the wall-clock gap between one
pulse's arrival and the next.

## 3. Gap encoding

Let `gap` be the time elapsed between the arrival of pulse `n-1` and pulse
`n`. A sender transmits two nominal gap durations:

- **short gap** (default `40ms`) — encodes bit `0`
- **long gap** (default `160ms`) — encodes bit `1`

A receiver decides each bit by comparing the observed gap to a calibrated
**threshold** `X` (default hint `100ms`, but see §4 — the real threshold
is always calibrated, never hardcoded):

```
bit = 0   if gap <  X
bit = 1   if gap >= X
```

Bits are assembled into bytes most-significant-bit first, 8 bits per byte,
in transmission order.

## 4. Preamble and calibration

PulseTP has **no explicit header**. Instead, the first 8 bits of every
transmission are a fixed, receiver-known pattern:

```
0 1 0 1 0 1 0 1
```

sent at the sender's nominal short/long gap durations — exactly like a
modem or radio handshake tone. The receiver knows this pattern in advance,
so it can measure the *actual* gap durations that arrived for each known 0
and each known 1, independent of whatever jitter, latency, or clock drift
the real network introduces.

From those eight measured gaps the receiver computes:

- `avgShort` — mean of the four gaps known to encode `0`
- `avgLong` — mean of the four gaps known to encode `1`
- `threshold = (avgShort + avgLong) / 2`
- `tolerance = |avgLong - avgShort| / 4`, floored at `1ms`

`threshold` replaces the hardcoded hint for the rest of the session.
`tolerance` defines a window around the threshold inside which a decoded
bit is flagged **low-confidence** (still decoded — a decision is always
made — but visually distinguishable to the operator, e.g. in the CLI's
live rhythm view).

Only after all 8 preamble bits have been observed does the receiver
transition from **preamble phase** to **data phase**. Every pulse from that
point on is real message data.

## 5. Framing and end-of-message

There is no length field and no explicit terminator packet. Instead:

- The sender transmits the preamble, then the message bytes (as bits, per
  §3), then simply **stops**.
- The receiver resets a silence timer on every received pulse.
- If no pulse arrives within `EndSilence` (default `2s`), the receiver
  declares the message complete and finalizes whatever bytes it has
  assembled so far.

This mirrors how a person tapping out Morse code signals "I'm done" —
by going quiet, not by sending a special "stop" symbol.

## 6. Receiver state machine

```
        first pulse arrives
WAITING ────────────────────▶ PREAMBLE
                                 │  8 gaps observed → calibrate
                                 ▼
                               DATA ──────────────▶ (silence > EndSilence)
                                 │                          │
                                 │ every 8 bits → 1 byte     ▼
                                 └─────────────────────▶ DONE
```

A receiver in any state that sees `EndSilence` elapse immediately finalizes
and stops — including mid-preamble, which simply yields an empty message.

## 7. Configuration parameters

| Parameter      | Default | Meaning                                          |
|----------------|--------:|---------------------------------------------------|
| `ShortGap`     | 40ms    | Nominal gap the sender uses for bit `0`           |
| `LongGap`      | 160ms   | Nominal gap the sender uses for bit `1`           |
| `Threshold`    | 100ms   | Fallback 0/1 boundary, used only before calibration completes |
| `EndSilence`   | 2s      | Silence duration that marks end-of-message         |
| `PreambleBits` | `01010101` | Known calibration pattern sent before real data |

Sender and receiver must agree on `ShortGap`/`LongGap`/`PreambleBits` for
calibration to converge cleanly; `Threshold` only matters if the message is
shorter than the preamble (i.e., never, since the preamble always completes
first) and is otherwise cosmetic.

## 8. Design notes and known limitations

- **No packet-loss recovery in v1.** If a pulse is dropped, every gap
  measured from that point on is doubled (or worse), which reads as a
  spurious `1` bit and desynchronizes byte alignment for the rest of the
  message. Repeat-and-majority-vote coding (transmitting each bit as an odd
  number of pulses and taking the majority) is the natural next step and is
  tracked as a stretch goal.
- **Timing precision is bounded by the OS scheduler and Go's runtime
  timer**, not by PulseTP itself. On a loaded machine, `time.After` and
  `ReadFromUDP` wake-ups can jitter by several milliseconds — this is
  exactly why calibration (§4) exists instead of a hardcoded threshold.
- **The protocol has no encryption, authentication, or replay protection.**
  Anyone who can inject UDP packets with correct spacing to the destination
  port can forge a message. PulseTP is a timing experiment, not a security
  protocol.
