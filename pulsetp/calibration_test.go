package pulsetp

import "testing"

func TestMajorityVote(t *testing.T) {
	cases := []struct {
		votes []int
		want  int
	}{
		{[]int{1, 1, 1}, 1},
		{[]int{0, 0, 0}, 0},
		{[]int{1, 0, 1}, 1}, // one dissenting 0 is outvoted
		{[]int{0, 1, 0}, 0}, // one dissenting 1 is outvoted
		{[]int{1, 1, 1, 1, 0}, 1},
		{[]int{0, 0, 0, 0, 1}, 0},
		{[]int{1}, 1},
		{[]int{0}, 0},
	}
	for _, c := range cases {
		if got := majorityVote(c.votes); got != c.want {
			t.Errorf("majorityVote(%v) = %d, want %d", c.votes, got, c.want)
		}
	}
}
