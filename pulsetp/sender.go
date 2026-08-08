package pulsetp

import (
	"context"
	"fmt"
	"net"
	"time"
)

// SentPulse describes one transmitted pulse, reported to callers that want
// to render the outgoing rhythm live.
type SentPulse struct {
	Index  int // 0 for the leading pulse, which carries no bit
	Seq    uint32
	Bit    int
	HasBit bool
	Gap    time.Duration
	Phase  Phase
	SentAt time.Time
}

// Send transmits message as a PulseTP pulse train to addr: a fixed-rhythm
// preamble first, so the receiver can calibrate, followed by the message
// bits, encoded purely as the delay before the next pulse. onPulse, if
// non-nil, is called synchronously after each pulse is written to the
// socket.
func Send(ctx context.Context, addr string, message []byte, cfg Config, onPulse func(SentPulse)) error {
	raddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		return fmt.Errorf("resolve %q: %w", addr, err)
	}
	conn, err := net.DialUDP("udp", nil, raddr)
	if err != nil {
		return fmt.Errorf("dial %q: %w", addr, err)
	}
	defer conn.Close()

	repeat := cfg.Repeat
	if repeat < 1 {
		repeat = 1
	}

	bits := make([]int, 0, len(cfg.PreambleBits)+len(message)*8*repeat)
	bits = append(bits, cfg.PreambleBits...)
	for _, bit := range BytesToBits(message) {
		for i := 0; i < repeat; i++ {
			bits = append(bits, bit)
		}
	}

	var seq uint32
	write := func() (time.Time, error) {
		p := Pulse{Seq: seq, SentAt: time.Now().UnixNano()}
		now := time.Now()
		_, err := conn.Write(p.Marshal())
		seq++
		return now, err
	}

	sentAt, err := write()
	if err != nil {
		return fmt.Errorf("send leading pulse: %w", err)
	}
	if onPulse != nil {
		onPulse(SentPulse{Index: 0, Seq: 0, Phase: phaseFor(0, len(cfg.PreambleBits)), SentAt: sentAt})
	}

	for i, bit := range bits {
		gap := cfg.ShortGap
		if bit == 1 {
			gap = cfg.LongGap
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(gap):
		}

		sentAt, err := write()
		if err != nil {
			return fmt.Errorf("send pulse %d: %w", seq, err)
		}
		if onPulse != nil {
			onPulse(SentPulse{
				Index:  i + 1,
				Seq:    seq - 1,
				Bit:    bit,
				HasBit: true,
				Gap:    gap,
				Phase:  phaseFor(i, len(cfg.PreambleBits)),
				SentAt: sentAt,
			})
		}
	}

	return nil
}

func phaseFor(bitIndex, preambleLen int) Phase {
	if bitIndex < preambleLen {
		return PhasePreamble
	}
	return PhaseData
}
