# PulseTP Protocol Specification

PulseTP is a UDP-based protocol that encodes information in the *timing*
between packets, not their contents. This document specifies the wire
format, timing rules, and receiver state machine precisely enough to
reimplement the protocol from scratch.

The payload is an arbitrary byte sequence: the CLI's `send`/`listen`
commands use it for both text messages and raw files (`--file`/`--output`).
Nothing in §3 or §4 cares what the bytes mean, only how many gaps it takes
to send them.

## 1. Transport

PulseTP runs directly over UDP. UDP is chosen specifically because it does
not buffer, coalesce, retransmit, or reorder packets the way a TCP stack
does, all of which would corrupt inter-packet timing before it ever
reached the application. PulseTP has no notion of a connection: a sender
simply fires pulses at a destination `host:port`, and a listener decodes
whatever arrives at a bound port.

## 2. The pulse packet

Every packet on the wire is a **pulse**: a fixed 15-byte struct with no
message content.

| Offset | Size | Field    | Description                                   |
|-------:|-----:|----------|------------------------------------------------|
| 0      | 2    | Magic    | ASCII `"PT"`, identifies a PulseTP pulse       |
| 2      | 1    | Version  | Protocol version, currently `1`                |
| 3      | 4    | Seq      | Sequence number, big-endian `uint32`, starts at 0 |
| 7      | 8    | SentAt   | Send timestamp, big-endian `int64`, Unix nanoseconds |

`Seq` and `SentAt` are diagnostic only: they let a receiver detect dropped
packets and measure clock skew, but **no bit of the message is ever encoded
in packet contents.** The entire signal is the wall-clock gap between one
pulse's arrival and the next.

## 3. Gap encoding

Let `gap` be the time elapsed between the arrival of pulse `n-1` and pulse
`n`. A sender transmits two nominal gap durations:

- **short gap** (default `40ms`), encodes bit `0`
- **long gap** (default `160ms`), encodes bit `1`

A receiver decides each bit by comparing the observed gap to a calibrated
**threshold** `X` (default hint `100ms`, but see §4, the real threshold
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

sent at the sender's nominal short/long gap durations, exactly like a
modem or radio handshake tone. The receiver knows this pattern in advance,
so it can measure the *actual* gap durations that arrived for each known 0
and each known 1, independent of whatever jitter, latency, or clock drift
the real network introduces.

From those eight measured gaps the receiver computes:

- `avgShort`: mean of the four gaps known to encode `0`
- `avgLong`: mean of the four gaps known to encode `1`
- `threshold = (avgShort + avgLong) / 2`
- `tolerance = |avgLong - avgShort| / 4`, floored at `1ms`

`threshold` replaces the hardcoded hint for the rest of the session.
`tolerance` defines a window around the threshold inside which a decoded
bit is flagged **low-confidence** (still decoded, a decision is always
made, but visually distinguishable to the operator, e.g. in the CLI's
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

This mirrors how a person tapping out Morse code signals "I'm done":
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
and stops, including mid-preamble, which simply yields an empty message.

## 7. Configuration parameters

| Parameter      | Default | Meaning                                          |
|----------------|--------:|---------------------------------------------------|
| `ShortGap`     | 40ms    | Nominal gap the sender uses for bit `0`           |
| `LongGap`      | 160ms   | Nominal gap the sender uses for bit `1`           |
| `Threshold`    | 100ms   | Fallback 0/1 boundary, used only before calibration completes |
| `EndSilence`   | 2s      | Silence duration that marks end-of-message         |
| `PreambleBits` | `01010101` | Known calibration pattern sent before real data |
| `Repeat`       | 1       | How many consecutive gaps encode each data bit (§10) |

Sender and receiver must agree on `ShortGap`/`LongGap`/`PreambleBits`/`Repeat`
for calibration and decoding to converge cleanly; `Threshold` only matters if
the message is shorter than the preamble (i.e., never, since the preamble
always completes first) and is otherwise cosmetic.

## 8. Design notes and known limitations

- **No recovery from an actually dropped UDP packet.** If a pulse never
  arrives, the gap measured across it is roughly double what it should be,
  which reads as a spurious `1` and, worse, permanently shifts bit
  alignment for the rest of the message (one fewer pulse arrived than the
  receiver is counting on). §10's repetition coding does not fix this
  case; nothing currently does.
- **Jitter-induced misclassification is fixed by §10.** A single gap that
  lands on the wrong side of the threshold purely from OS/network timing
  noise (not an actual dropped packet) used to silently corrupt one bit.
  `--repeat` now catches this via majority vote.
- **Timing precision is bounded by the OS scheduler and Go's runtime
  timer**, not by PulseTP itself. On a loaded machine, `time.After` and
  `ReadFromUDP` wake-ups can jitter by several milliseconds; this is
  exactly why calibration (§4) exists instead of a hardcoded threshold.
- **The core protocol has no encryption, authentication, or replay
  protection.** The preamble, calibration math, and bit threshold are all
  public, so anyone who can observe the packets (not just the intended
  receiver) can decode the message exactly the same way, just from
  arrival timestamps, no access to PulseTP's code required. Anyone who
  can inject UDP packets with correct spacing to the destination port can
  also forge a message. See §9 for the optional application-layer fix for
  confidentiality.

## 9. Optional payload encryption

`send --key` / `listen --key` (or the `PULSETP_KEY` environment variable)
add confidentiality on top of the core protocol, without changing the wire
format at all: the encrypted bytes are just another opaque payload, exactly
like a plaintext message or a file.

- **Key derivation**: `scrypt(passphrase, salt, N=32768, r=8, p=1, 32)`
  produces a 256-bit key from the passphrase. A fresh random 16-byte salt
  is generated per message.
- **Encryption**: AES-256-GCM with a fresh random 12-byte nonce per
  message. The payload that actually goes over the wire is
  `salt || nonce || ciphertext`, GCM's authentication tag is part of the
  ciphertext.
- **Failure mode**: a wrong passphrase, or any corruption/truncation in
  transit, fails GCM authentication and returns an error. It never
  silently produces garbage plaintext.

What this does and doesn't buy you:

- It hides the *content* of the message from anyone without the
  passphrase, an eavesdropper still recovers a byte-identical ciphertext
  from the timing (per §8's threat model above), but can't read it.
- It does **not** hide that a transmission happened, its timing pattern,
  or its length. Traffic analysis (someone is talking, roughly how much,
  for how long) is still fully exposed, same as any protocol without
  padding or cover traffic.
- It has no replay protection at the application layer: a captured pulse
  train, re-sent later by an eavesdropper with correct timing, decrypts
  successfully again. GCM authenticates the ciphertext against the key,
  not the freshness of the session.

## 10. Optional repetition coding (majority vote)

`send --repeat N` / `listen --repeat N` (both sides must agree on `N`, an
odd number, default 1 meaning disabled) trade throughput for tolerance to
jitter-induced misclassification, without changing the wire format: it's
still one pulse per gap, just more of them.

- **Sender**: each *data* bit (the preamble is unaffected, it's still
  always exactly 8 pulses) is transmitted as `N` consecutive gaps of the
  same duration, `ShortGap` repeated `N` times for a `0`, `LongGap`
  repeated `N` times for a `1`. Total data pulses become
  `len(payload) * 8 * N` instead of `len(payload) * 8`.
- **Receiver**: classifies every gap individually exactly as in §3 (each
  one still gets its own `EventPulse` with its own bit/confidence, so a
  live view shows the raw, possibly-noisy stream), then buffers `N`
  classifications per logical bit and takes a majority vote
  (`ones*2 > N`) to decide the actual bit that goes into the message.
  Because `N` is odd, a vote never ties.
- **What it fixes**: an occasional single gap landing on the wrong side of
  the threshold purely from timing noise, e.g. `N=3` survives any one bad
  sample per bit; `N=5` survives any two.
- **What it doesn't fix**: an actually dropped UDP packet. Losing a pulse
  merges two gaps into one observed gap and permanently shifts alignment
  for everything after it (§8), no amount of repetition recovers from a
  pulse that never arrived, because the receiver has no way to know one
  is missing from a repeat group versus not having started one yet.
- **Cost**: linear in `N`. `--repeat 5` takes 5x as long to transmit the
  same message as `--repeat 1`; combined with encryption's fixed 44-byte
  overhead, short messages get slow fast. `send` always prints the
  resulting pulse count and estimated duration up front.
