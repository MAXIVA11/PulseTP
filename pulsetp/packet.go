package pulsetp

import (
	"encoding/binary"
	"fmt"
)

const (
	magicByte0 = 'P'
	magicByte1 = 'T'
	version    = 1

	// pulseSize is the wire size of a pulse packet: 2 magic bytes, 1
	// version byte, 4 bytes sequence number, 8 bytes send timestamp.
	// Deliberately tiny — the payload here is never what carries meaning.
	pulseSize = 2 + 1 + 4 + 8
)

// Pulse is the minimal "near-empty" packet PulseTP transmits. It carries no
// message content — only a sequence number and a send timestamp used for
// diagnostics and loss detection. The actual signal is the gap in arrival
// time between one pulse and the next.
type Pulse struct {
	Seq    uint32
	SentAt int64 // unix nanoseconds
}

// Marshal encodes the pulse into its wire format.
func (p Pulse) Marshal() []byte {
	buf := make([]byte, pulseSize)
	buf[0] = magicByte0
	buf[1] = magicByte1
	buf[2] = version
	binary.BigEndian.PutUint32(buf[3:7], p.Seq)
	binary.BigEndian.PutUint64(buf[7:15], uint64(p.SentAt))
	return buf
}

// UnmarshalPulse decodes a pulse from its wire format, rejecting anything
// that isn't a recognizable PulseTP packet.
func UnmarshalPulse(b []byte) (Pulse, error) {
	if len(b) < pulseSize {
		return Pulse{}, fmt.Errorf("pulsetp: packet too short (%d bytes)", len(b))
	}
	if b[0] != magicByte0 || b[1] != magicByte1 {
		return Pulse{}, fmt.Errorf("pulsetp: not a pulse packet (bad magic)")
	}
	if b[2] != version {
		return Pulse{}, fmt.Errorf("pulsetp: unsupported protocol version %d", b[2])
	}
	return Pulse{
		Seq:    binary.BigEndian.Uint32(b[3:7]),
		SentAt: int64(binary.BigEndian.Uint64(b[7:15])),
	}, nil
}
