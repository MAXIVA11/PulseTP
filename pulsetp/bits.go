package pulsetp

// BytesToBits expands bytes into individual bits, most significant bit
// first, in transmission order.
func BytesToBits(data []byte) []int {
	bits := make([]int, 0, len(data)*8)
	for _, b := range data {
		for i := 7; i >= 0; i-- {
			bits = append(bits, int((b>>uint(i))&1))
		}
	}
	return bits
}

// BitsToByte packs up to 8 bits, most significant bit first, into a byte.
// Fewer than 8 bits are treated as left-padded with the missing high bits.
func BitsToByte(bits []int) byte {
	var b byte
	for _, bit := range bits {
		b = b<<1 | byte(bit&1)
	}
	return b
}
