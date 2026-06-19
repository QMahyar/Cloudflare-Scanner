package main

import (
	"testing"
	"time"
)

func TestLossPercent(t *testing.T) {
	cases := []struct {
		passes, attempts int
		want             float64
	}{
		{0, 0, 0},   // never measured -> unknown, not 100
		{4, 4, 0},   // perfect
		{0, 4, 100}, // total loss
		{3, 4, 25},
		{1, 2, 50},
		{5, 4, 0},  // passes > attempts clamps to 0 loss
		{-1, 4, 100}, // negative passes clamps to 0
	}
	for _, c := range cases {
		if got := lossPercent(c.passes, c.attempts); got != c.want {
			t.Errorf("lossPercent(%d,%d) = %v, want %v", c.passes, c.attempts, got, c.want)
		}
	}
}

func TestQualityScore(t *testing.T) {
	ms := func(n int) time.Duration { return time.Duration(n) * time.Millisecond }

	// No measurement -> 0 (never a free 100).
	if got := qualityScore(0, 0, 0); got != 0 {
		t.Errorf("qualityScore(0,..) = %d, want 0", got)
	}

	// Score must stay within 0..100 across the input space.
	for _, lat := range []int{1, 20, 100, 250, 400, 1500} {
		for _, jit := range []int{0, 10, 50, 200} {
			for _, loss := range []float64{0, 10, 50, 100} {
				s := qualityScore(ms(lat), ms(jit), loss)
				if s < 0 || s > 100 {
					t.Errorf("qualityScore(%dms,%dms,%.0f%%) = %d out of range", lat, jit, loss, s)
				}
			}
		}
	}

	// A fast, stable, lossless IP must outrank a slow / lossy one.
	good := qualityScore(ms(20), ms(3), 0)
	bad := qualityScore(ms(300), ms(80), 50)
	if good <= bad {
		t.Errorf("expected good(%d) > bad(%d)", good, bad)
	}
	if good < 90 {
		t.Errorf("a 20ms/3ms/0%% IP should score high, got %d", good)
	}

	// Loss must dominate: same latency, more loss => strictly lower score.
	a := qualityScore(ms(50), ms(5), 0)
	b := qualityScore(ms(50), ms(5), 40)
	if b >= a {
		t.Errorf("higher loss should lower score: a=%d b=%d", a, b)
	}
}
