package pulsetp

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"
)

func TestLoopback(t *testing.T) {
	cfg := DefaultConfig()
	// Speed the test up: keep the ratio but shrink absolute durations.
	cfg.ShortGap = 8 * time.Millisecond
	cfg.LongGap = 32 * time.Millisecond
	cfg.Threshold = 20 * time.Millisecond
	cfg.EndSilence = 300 * time.Millisecond

	l, err := Listen(cfg, 0)
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer l.Close()

	addr := fmt.Sprintf("127.0.0.1:%d", l.Addr().(*net.UDPAddr).Port)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	events := l.Run(ctx)

	want := []byte("hi")
	sendErr := make(chan error, 1)
	go func() {
		sendErr <- Send(ctx, addr, want, cfg, nil)
	}()

	var got []byte
	var calibration Calibration
	sawCalibration := false
	for ev := range events {
		switch ev.Kind {
		case EventError:
			t.Fatalf("receiver error: %v", ev.Err)
		case EventCalibrated:
			calibration = ev.Calibration
			sawCalibration = true
		case EventMessage:
			got = ev.Message
		}
	}

	if err := <-sendErr; err != nil {
		t.Fatalf("send: %v", err)
	}
	if !sawCalibration {
		t.Fatalf("never calibrated")
	}
	if string(got) != string(want) {
		t.Fatalf("got %q, want %q (calibration: %+v)", got, want, calibration)
	}
}

func TestLoopbackWithRepeat(t *testing.T) {
	cfg := DefaultConfig()
	cfg.ShortGap = 8 * time.Millisecond
	cfg.LongGap = 32 * time.Millisecond
	cfg.Threshold = 20 * time.Millisecond
	cfg.EndSilence = 300 * time.Millisecond
	cfg.Repeat = 3

	l, err := Listen(cfg, 0)
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer l.Close()

	addr := fmt.Sprintf("127.0.0.1:%d", l.Addr().(*net.UDPAddr).Port)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	events := l.Run(ctx)

	want := []byte("hi")
	sendErr := make(chan error, 1)
	go func() {
		sendErr <- Send(ctx, addr, want, cfg, nil)
	}()

	var got []byte
	var pulseCount int
	for ev := range events {
		switch ev.Kind {
		case EventError:
			t.Fatalf("receiver error: %v", ev.Err)
		case EventPulse:
			pulseCount++
		case EventMessage:
			got = ev.Message
		}
	}

	if err := <-sendErr; err != nil {
		t.Fatalf("send: %v", err)
	}
	if string(got) != string(want) {
		t.Fatalf("got %q, want %q", got, want)
	}

	// preamble (8) + leading pulse (1) + data bits * Repeat
	wantPulses := 1 + len(cfg.PreambleBits) + len(want)*8*cfg.Repeat
	if pulseCount != wantPulses {
		t.Fatalf("got %d pulses, want %d (repeat=%d not applied?)", pulseCount, wantPulses, cfg.Repeat)
	}
}
