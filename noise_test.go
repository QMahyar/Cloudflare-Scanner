package main

import (
	"strings"
	"testing"
)

func TestNoiseValidate(t *testing.T) {
	// base64("hello") = aGVsbG8=
	const goodB64 = "aGVsbG8="

	cases := []struct {
		name    string
		cfg     NoiseConfig
		wantErr string // substring; empty means expect nil
	}{
		// empty type skips all validation
		{"empty-type", NoiseConfig{Type: ""}, ""},
		{"empty-type-ignores-bad-count", NoiseConfig{Type: "", Count: 0, Delay: "nope"}, ""},

		// rand
		{"rand-ok", NoiseConfig{Type: "rand", Packet: "50-100", Delay: "1-5", Count: 5}, ""},
		{"rand-single-packet", NoiseConfig{Type: "rand", Packet: "50", Delay: "3", Count: 1}, ""},
		{"rand-bad-packet", NoiseConfig{Type: "rand", Packet: "abc", Delay: "1-5", Count: 5}, "invalid packet range"},
		{"rand-empty-packet", NoiseConfig{Type: "rand", Packet: "", Delay: "1-5", Count: 5}, "invalid packet range"},
		{"rand-inverted-range", NoiseConfig{Type: "rand", Packet: "100-50", Delay: "1-5", Count: 5}, "invalid packet range"},
		{"rand-zero-min", NoiseConfig{Type: "rand", Packet: "0-10", Delay: "1-5", Count: 5}, "invalid packet range"},

		// base64
		{"base64-ok", NoiseConfig{Type: "base64", Packet: goodB64, Delay: "1-5", Count: 5}, ""},
		{"base64-empty", NoiseConfig{Type: "base64", Packet: "", Delay: "1-5", Count: 5}, "base64 packet cannot be empty"},
		{"base64-invalid", NoiseConfig{Type: "base64", Packet: "!!!", Delay: "1-5", Count: 5}, "invalid base64"},

		// hex
		{"hex-ok", NoiseConfig{Type: "hex", Packet: "deadbeef", Delay: "1-5", Count: 5}, ""},
		{"hex-mixed-case", NoiseConfig{Type: "hex", Packet: "AbCdEf", Delay: "1-5", Count: 10}, ""},
		{"hex-empty", NoiseConfig{Type: "hex", Packet: "", Delay: "1-5", Count: 5}, "invalid hex"},
		{"hex-bad", NoiseConfig{Type: "hex", Packet: "xyz", Delay: "1-5", Count: 5}, "invalid hex"},

		// str
		{"str-ok", NoiseConfig{Type: "str", Packet: "hello", Delay: "1-5", Count: 5}, ""},
		{"str-empty", NoiseConfig{Type: "str", Packet: "", Delay: "1-5", Count: 5}, "string packet cannot be empty"},

		// unknown type
		{"unknown-type", NoiseConfig{Type: "bogus", Packet: "x", Delay: "1-5", Count: 5}, "unknown noise type"},

		// delay / count (checked for any non-empty type)
		{"bad-delay", NoiseConfig{Type: "rand", Packet: "10", Delay: "nope", Count: 5}, "invalid delay range"},
		{"empty-delay", NoiseConfig{Type: "rand", Packet: "10", Delay: "", Count: 5}, "invalid delay range"},
		{"count-0", NoiseConfig{Type: "rand", Packet: "10", Delay: "1-5", Count: 0}, "noise count must be 1-50"},
		{"count-1", NoiseConfig{Type: "rand", Packet: "10", Delay: "1-5", Count: 1}, ""},
		{"count-50", NoiseConfig{Type: "rand", Packet: "10", Delay: "1-5", Count: 50}, ""},
		{"count-51", NoiseConfig{Type: "rand", Packet: "10", Delay: "1-5", Count: 51}, "noise count must be 1-50"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := c.cfg.Validate()
			if c.wantErr == "" {
				if err != nil {
					t.Fatalf("Validate() unexpected error: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("Validate() = nil, want error containing %q", c.wantErr)
			}
			if !strings.Contains(err.Error(), c.wantErr) {
				t.Errorf("error %q does not contain %q", err.Error(), c.wantErr)
			}
		})
	}
}

func TestDefaultNoise(t *testing.T) {
	n := DefaultNoise()
	if err := n.Validate(); err != nil {
		t.Errorf("DefaultNoise() failed Validate: %v", err)
	}
	if n.Type != "rand" || n.Count != 5 {
		t.Errorf("DefaultNoise() = %+v, unexpected defaults", n)
	}
}
