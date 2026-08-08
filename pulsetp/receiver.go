package pulsetp

import (
	"context"
	"fmt"
	"net"
	"time"
)

// Listener receives a PulseTP pulse train on a UDP port and decodes it into
// bytes, emitting live Events as pulses arrive.
type Listener struct {
	Config Config
	conn   *net.UDPConn
}

// Listen opens a UDP socket on port and returns a Listener ready to
// decode an incoming pulse train.
func Listen(cfg Config, port int) (*Listener, error) {
	conn, err := net.ListenUDP("udp", &net.UDPAddr{Port: port})
	if err != nil {
		return nil, fmt.Errorf("listen on port %d: %w", port, err)
	}
	return &Listener{Config: cfg, conn: conn}, nil
}

// Addr returns the address the listener is bound to.
func (l *Listener) Addr() net.Addr {
	return l.conn.LocalAddr()
}

// Close closes the underlying socket.
func (l *Listener) Close() error {
	return l.conn.Close()
}

type rawPacket struct {
	data []byte
	at   time.Time
}

// Run decodes one full message: it blocks reading pulses until a long
// silence (Config.EndSilence) signals the message ended, or ctx is
// cancelled. It returns a channel of live Events; the channel is closed
// when the message is complete, the context is cancelled, or the socket
// errors.
func (l *Listener) Run(ctx context.Context) <-chan Event {
	events := make(chan Event, 64)
	go l.run(ctx, events)
	return events
}

func (l *Listener) run(ctx context.Context, events chan<- Event) {
	defer close(events)

	raw := make(chan rawPacket, 64)
	readErrs := make(chan error, 1)
	go func() {
		buf := make([]byte, 1500)
		for {
			n, _, err := l.conn.ReadFromUDP(buf)
			if err != nil {
				readErrs <- err
				return
			}
			at := time.Now()
			cp := make([]byte, n)
			copy(cp, buf[:n])
			select {
			case raw <- rawPacket{data: cp, at: at}:
			case <-ctx.Done():
				return
			}
		}
	}()

	stopWatchingCtx := make(chan struct{})
	defer close(stopWatchingCtx)
	go func() {
		select {
		case <-ctx.Done():
			l.conn.Close()
		case <-stopWatchingCtx:
		}
	}()

	cal := NewCalibrator(l.Config.PreambleBits)
	calibrated := false
	threshold := l.Config.Threshold
	var tolerance time.Duration

	var lastAt time.Time
	haveLast := false
	pulseIndex := 0

	repeat := l.Config.Repeat
	if repeat < 1 {
		repeat = 1
	}

	var voteBuf []int
	var bitBuf []int
	var message []byte

	silence := time.NewTimer(24 * time.Hour)
	silence.Stop()
	defer silence.Stop()

	resetSilence := func() {
		if !silence.Stop() {
			select {
			case <-silence.C:
			default:
			}
		}
		silence.Reset(l.Config.EndSilence)
	}

	for {
		select {
		case <-ctx.Done():
			return

		case err := <-readErrs:
			if !haveLast {
				// Socket closed/errored before anything arrived.
				events <- Event{Kind: EventError, Err: fmt.Errorf("pulsetp: listen error: %w", err)}
			}
			return

		case pkt := <-raw:
			p, err := UnmarshalPulse(pkt.data)
			if err != nil {
				events <- Event{Kind: EventError, Err: err}
				continue
			}

			resetSilence()

			if !haveLast {
				haveLast = true
				lastAt = pkt.at
				events <- Event{Kind: EventPulse, Pulse: PulseInfo{
					Seq: p.Seq, Index: pulseIndex, Arrival: pkt.at,
					Phase: PhasePreamble, Threshold: threshold,
				}}
				pulseIndex++
				continue
			}

			gap := pkt.at.Sub(lastAt)
			lastAt = pkt.at

			if !calibrated {
				cal.Add(gap)
				events <- Event{Kind: EventPulse, Pulse: PulseInfo{
					Seq: p.Seq, Index: pulseIndex, Arrival: pkt.at,
					Gap: gap, HasGap: true, Phase: PhasePreamble, Threshold: threshold,
				}}
				pulseIndex++

				if cal.Done() {
					result := cal.Result(l.Config)
					threshold = result.Threshold
					tolerance = result.Tolerance
					calibrated = true
					events <- Event{Kind: EventCalibrated, Calibration: result}
				}
				continue
			}

			bit, confident := Classify(gap, threshold, tolerance)
			events <- Event{Kind: EventPulse, Pulse: PulseInfo{
				Seq: p.Seq, Index: pulseIndex, Arrival: pkt.at,
				Gap: gap, HasGap: true, Bit: bit, HasBit: true,
				Confident: confident, Phase: PhaseData, Threshold: threshold,
			}}
			pulseIndex++

			voteBuf = append(voteBuf, bit)
			if len(voteBuf) < repeat {
				continue
			}
			decoded := majorityVote(voteBuf)
			voteBuf = voteBuf[:0]
			bitBuf = append(bitBuf, decoded)

			if len(bitBuf) == 8 {
				b := BitsToByte(bitBuf)
				bitBuf = bitBuf[:0]
				message = append(message, b)
				out := make([]byte, len(message))
				copy(out, message)
				events <- Event{Kind: EventByte, Byte: b, Message: out}
			}

		case <-silence.C:
			if haveLast {
				out := make([]byte, len(message))
				copy(out, message)
				events <- Event{Kind: EventMessage, Message: out}
			}
			return
		}
	}
}
