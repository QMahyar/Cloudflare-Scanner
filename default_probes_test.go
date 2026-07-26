package main

import (
	"runtime"
	"testing"
)

func TestDefaultPhase1Probes(t *testing.T) {
	n := defaultPhase1Probes()
	if n < 128 {
		t.Fatalf("too low: %d", n)
	}
	if n > 500 {
		t.Fatalf("too high: %d", n)
	}
	if runtime.GOOS == "windows" && n > 256 {
		t.Fatalf("windows cap exceeded: %d", n)
	}
}
