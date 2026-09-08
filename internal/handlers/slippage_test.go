package handlers

import "testing"

// The delta is what makes reputation mean something: without it, an agent that
// promises 0.1% and delivers 0.1% is indistinguishable from one that promises
// 0.1% and delivers 2%. Both win, and both look identical afterwards.
func TestSlippageDeltaSign(t *testing.T) {
	cases := []struct {
		name      string
		projected float64
		actual    float64
		wantSign  int
	}{
		{"delivered worse than promised", 0.001, 0.02, +1},
		{"delivered exactly as promised", 0.005, 0.005, 0},
		{"delivered better than promised", 0.01, 0.002, -1},
	}

	for _, c := range cases {
		delta := c.actual - c.projected
		got := 0
		if delta > 0 {
			got = 1
		} else if delta < 0 {
			got = -1
		}
		if got != c.wantSign {
			t.Errorf("%s: delta %.4f has sign %d, want %d", c.name, delta, got, c.wantSign)
		}
	}
}

// Mirrors the SQL running mean, which is computed against the pre-increment
// count so a concurrent settlement cannot read a stale average.
func TestRunningMean(t *testing.T) {
	mean, n := 0.0, int64(0)
	for _, v := range []float64{0.02, 0.01} {
		mean = (mean*float64(n) + v) / float64(n+1)
		n++
	}
	if want := 0.015; mean != want {
		t.Fatalf("running mean = %v, want %v", mean, want)
	}
}
