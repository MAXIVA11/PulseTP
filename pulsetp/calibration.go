package pulsetp

import "time"

// Calibration is the result of measuring the preamble: the actual short and
// long gap timings observed over the network, and the threshold/tolerance
// derived from them.
type Calibration struct {
	AvgShort  time.Duration
	AvgLong   time.Duration
	Threshold time.Duration
	Tolerance time.Duration
}

// Calibrator consumes inter-pulse gaps measured during the known preamble
// pattern and derives a decision threshold tailored to actual network
// jitter, rather than trusting a hardcoded value.
type Calibrator struct {
	pattern []int
	idx     int
	short   []time.Duration
	long    []time.Duration
}

// NewCalibrator creates a calibrator expecting the given known bit pattern.
func NewCalibrator(pattern []int) *Calibrator {
	return &Calibrator{pattern: pattern}
}

// Done reports whether every preamble gap has been observed.
func (c *Calibrator) Done() bool {
	return c.idx >= len(c.pattern)
}

// Remaining returns how many preamble gaps are still expected.
func (c *Calibrator) Remaining() int {
	return len(c.pattern) - c.idx
}

// Add records one observed gap against the next expected preamble bit.
func (c *Calibrator) Add(gap time.Duration) {
	if c.Done() {
		return
	}
	if c.pattern[c.idx] == 0 {
		c.short = append(c.short, gap)
	} else {
		c.long = append(c.long, gap)
	}
	c.idx++
}

// Result computes the calibrated threshold and tolerance window from
// observed samples. The threshold sits at the midpoint between the average
// short and long gap; the tolerance is a quarter of the spread between
// them, giving a jitter margin proportional to how far apart the two
// symbols actually are on this network.
func (c *Calibrator) Result(fallback Config) Calibration {
	avgShort := average(c.short, fallback.ShortGap)
	avgLong := average(c.long, fallback.LongGap)

	threshold := (avgShort + avgLong) / 2
	tolerance := (avgLong - avgShort) / 4
	if tolerance < 0 {
		tolerance = -tolerance
	}
	if tolerance < time.Millisecond {
		tolerance = time.Millisecond
	}

	return Calibration{
		AvgShort:  avgShort,
		AvgLong:   avgLong,
		Threshold: threshold,
		Tolerance: tolerance,
	}
}

func average(samples []time.Duration, fallback time.Duration) time.Duration {
	if len(samples) == 0 {
		return fallback
	}
	var sum time.Duration
	for _, s := range samples {
		sum += s
	}
	return sum / time.Duration(len(samples))
}

// Classify decides the bit value for an observed gap against a calibrated
// threshold. Gaps within the tolerance window of the threshold are still
// classified (there must always be an answer) but reported as low
// confidence so callers such as the CLI can flag them visually.
func Classify(gap, threshold, tolerance time.Duration) (bit int, confident bool) {
	if gap >= threshold {
		bit = 1
	}
	diff := gap - threshold
	if diff < 0 {
		diff = -diff
	}
	confident = diff >= tolerance
	return bit, confident
}

// majorityVote resolves repeated per-gap classifications of the same
// logical bit into a single value: whichever of 0/1 appears more often
// wins. Callers are expected to pass an odd-length slice (Config.Repeat)
// so there's always a clear winner; an even-length tie falls back to 0.
func majorityVote(votes []int) int {
	ones := 0
	for _, v := range votes {
		if v == 1 {
			ones++
		}
	}
	if ones*2 > len(votes) {
		return 1
	}
	return 0
}
