// Package pulsetp implements the PulseTP rhythm-based protocol: a UDP
// transport where meaning lives in the timing between packets rather than
// in packet contents.
package pulsetp

import "time"

// Config holds the tunable timing parameters for a PulseTP session. Sender
// and receiver must agree on the broad shape (short/long nominal gaps and
// the preamble pattern) but the receiver calibrates its own decision
// threshold from the preamble rather than trusting a hardcoded value, so
// small clock/network differences don't break decoding.
type Config struct {
	// ShortGap is the nominal inter-pulse delay used to transmit a 0 bit.
	ShortGap time.Duration
	// LongGap is the nominal inter-pulse delay used to transmit a 1 bit.
	LongGap time.Duration
	// Threshold is the fallback 0/1 decision boundary used only until the
	// receiver finishes calibrating against the preamble.
	Threshold time.Duration
	// EndSilence is how long the receiver waits without a new pulse before
	// declaring the message complete.
	EndSilence time.Duration
	// PreambleBits is the known bit pattern sent first so the receiver can
	// measure real short/long gap timing and calibrate its threshold.
	PreambleBits []int
	// Repeat is how many consecutive gaps encode each data bit (the
	// preamble is unaffected). The sender transmits the same bit value
	// Repeat times in a row; the receiver takes a majority vote across
	// those gaps instead of trusting any single one, tolerating an
	// occasional jitter-induced misclassification. Must be odd so a vote
	// never ties; 1 (the default) disables repetition entirely.
	Repeat int
}

// DefaultConfig returns PulseTP's default timing parameters, tuned for a
// friendly zero-config experience on localhost and typical LANs.
func DefaultConfig() Config {
	return Config{
		ShortGap:     40 * time.Millisecond,
		LongGap:      160 * time.Millisecond,
		Threshold:    100 * time.Millisecond,
		EndSilence:   2 * time.Second,
		PreambleBits: []int{0, 1, 0, 1, 0, 1, 0, 1},
		Repeat:       1,
	}
}
