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
		{5, 4, 0},    // passes > attempts clamps to 0 loss
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

func TestMedianDuration(t *testing.T) {
	ms := func(n int) time.Duration { return time.Duration(n) * time.Millisecond }

	cases := []struct {
		name string
		in   []time.Duration
		want time.Duration
	}{
		{"empty", nil, 0},
		{"single", []time.Duration{ms(10)}, ms(10)},
		{"odd", []time.Duration{ms(30), ms(10), ms(20)}, ms(20)},
		// even: average of the two middle values after sort
		{"even", []time.Duration{ms(40), ms(10), ms(20), ms(30)}, ms(25)},
		// does not mutate the caller's slice
		{"unsorted-odd", []time.Duration{ms(5), ms(1), ms(9)}, ms(5)},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			orig := append([]time.Duration(nil), c.in...)
			got := medianDuration(c.in)
			if got != c.want {
				t.Errorf("medianDuration(%v) = %v, want %v", c.in, got, c.want)
			}
			for i := range orig {
				if c.in[i] != orig[i] {
					t.Errorf("input mutated at %d: got %v want %v", i, c.in[i], orig[i])
				}
			}
		})
	}
}

func TestBestDuration(t *testing.T) {
	ms := func(n int) time.Duration { return time.Duration(n) * time.Millisecond }

	if got := bestDuration(nil); got != 0 {
		t.Errorf("bestDuration(nil) = %v, want 0", got)
	}
	if got := bestDuration([]time.Duration{}); got != 0 {
		t.Errorf("bestDuration(empty) = %v, want 0", got)
	}
	in := []time.Duration{ms(50), ms(10), ms(30)}
	if got := bestDuration(in); got != ms(10) {
		t.Errorf("bestDuration = %v, want 10ms", got)
	}
	// single element
	if got := bestDuration([]time.Duration{ms(7)}); got != ms(7) {
		t.Errorf("bestDuration(single) = %v, want 7ms", got)
	}
}

func TestJitterDuration(t *testing.T) {
	ms := func(n int) time.Duration { return time.Duration(n) * time.Millisecond }

	if got := jitterDuration(nil); got != 0 {
		t.Errorf("jitterDuration(nil) = %v, want 0", got)
	}
	if got := jitterDuration([]time.Duration{ms(5)}); got != 0 {
		t.Errorf("jitterDuration(len=1) = %v, want 0", got)
	}
	// max - min after sort
	in := []time.Duration{ms(40), ms(10), ms(25)}
	orig := append([]time.Duration(nil), in...)
	if got := jitterDuration(in); got != ms(30) {
		t.Errorf("jitterDuration = %v, want 30ms", got)
	}
	for i := range orig {
		if in[i] != orig[i] {
			t.Errorf("input mutated at %d", i)
		}
	}
}

func TestSortScanResults(t *testing.T) {
	ms := func(n int) time.Duration { return time.Duration(n) * time.Millisecond }
	results := []ScanResult{
		{Endpoint: "fail-fast", Success: false, Latency: ms(5)},
		{Endpoint: "ok-slow", Success: true, Latency: ms(50)},
		{Endpoint: "fail-slow", Success: false, Latency: ms(100)},
		{Endpoint: "ok-fast", Success: true, Latency: ms(10)},
	}
	sortScanResults(results)

	wantOrder := []string{"ok-fast", "ok-slow", "fail-fast", "fail-slow"}
	for i, want := range wantOrder {
		if results[i].Endpoint != want {
			t.Errorf("pos %d: got %q, want %q", i, results[i].Endpoint, want)
		}
	}
	// successes before failures
	for i := 0; i < 2; i++ {
		if !results[i].Success {
			t.Errorf("pos %d should be success", i)
		}
	}
	for i := 2; i < 4; i++ {
		if results[i].Success {
			t.Errorf("pos %d should be failure", i)
		}
	}
}

func TestSortCleanIPResults(t *testing.T) {
	ms := func(n int) time.Duration { return time.Duration(n) * time.Millisecond }
	results := []CleanIPResult{
		{Endpoint: "1.1.1.1:443", Success: false, Latency: ms(5)},
		{Endpoint: "1.0.0.1:443", Success: true, Latency: ms(40)},
		{Endpoint: "8.8.8.8:443", Success: false, Latency: ms(90)},
		{Endpoint: "8.8.4.4:443", Success: true, Latency: ms(15)},
	}
	sortCleanIPResults(results)

	wantOrder := []string{"8.8.4.4:443", "1.0.0.1:443", "1.1.1.1:443", "8.8.8.8:443"}
	for i, want := range wantOrder {
		if results[i].Endpoint != want {
			t.Errorf("pos %d: got %q, want %q", i, results[i].Endpoint, want)
		}
	}
}
