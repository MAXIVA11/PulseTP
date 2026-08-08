package pulsetp

import "time"

// Phase describes where a receiver is in decoding a message.
type Phase string

const (
	PhaseWaiting  Phase = "waiting"  // no pulses received yet
	PhasePreamble Phase = "preamble" // calibrating against the known preamble
	PhaseData     Phase = "data"     // decoding real message bits
)

// EventKind identifies what a receiver Event represents.
type EventKind int

const (
	// EventPulse fires for every received pulse (including the very first,
	// which carries no gap yet).
	EventPulse EventKind = iota
	// EventCalibrated fires once, right after the preamble finishes and a
	// threshold has been derived.
	EventCalibrated
	// EventByte fires whenever 8 decoded bits assemble into a new byte.
	EventByte
	// EventMessage fires once, when silence signals the message is complete.
	EventMessage
	// EventError fires on a malformed or unexpected packet; the receiver
	// keeps running.
	EventError
)

// PulseInfo describes a single received pulse and, if applicable, the bit
// decoded from the gap since the previous pulse.
type PulseInfo struct {
	Seq       uint32
	Index     int // pulse index within this session, 0-based
	Arrival   time.Time
	Gap       time.Duration // zero for the very first pulse
	HasGap    bool
	Bit       int
	HasBit    bool
	Confident bool
	Phase     Phase
	Threshold time.Duration // best current threshold estimate
}

// Event is emitted by a Receiver as a message is decoded live.
type Event struct {
	Kind        EventKind
	Pulse       PulseInfo
	Calibration Calibration
	Byte        byte
	Message     []byte
	Err         error
}
