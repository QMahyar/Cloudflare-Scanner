package main

import (
	"encoding/base64"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// hexPattern is compiled once instead of on every Validate() call.
var hexPattern = regexp.MustCompile(`^[0-9a-fA-F]+$`)

type NoiseConfig struct {
	Type   string
	Packet string
	Delay  string
	Count  int
}

func DefaultNoise() NoiseConfig {
	return NoiseConfig{
		Type:   "rand",
		Packet: "50-100",
		Delay:  "1-5",
		Count:  5,
	}
}

func (n NoiseConfig) Validate() error {
	switch n.Type {
	case "rand":
		if !isValidRange(n.Packet) {
			return fmt.Errorf("invalid packet range: %s", n.Packet)
		}
	case "base64":
		if n.Packet == "" {
			return fmt.Errorf("base64 packet cannot be empty")
		}
		_, err := base64.StdEncoding.DecodeString(n.Packet)
		if err != nil {
			return fmt.Errorf("invalid base64: %w", err)
		}
	case "hex":
		if !hexPattern.MatchString(n.Packet) || len(n.Packet) == 0 {
			return fmt.Errorf("invalid hex: %s", n.Packet)
		}
	case "str":
		if len(n.Packet) == 0 {
			return fmt.Errorf("string packet cannot be empty")
		}
	case "":
		return nil
	default:
		return fmt.Errorf("unknown noise type: %s", n.Type)
	}

	if n.Type != "" {
		if !isValidRange(n.Delay) {
			return fmt.Errorf("invalid delay range: %s", n.Delay)
		}
		if n.Count < 1 || n.Count > 50 {
			return fmt.Errorf("noise count must be 1-50, got %d", n.Count)
		}
	}

	return nil
}

func isValidRange(value string) bool {
	if value == "" {
		return false
	}
	if strings.Contains(value, "-") {
		parts := strings.SplitN(value, "-", 2)
		if len(parts) != 2 {
			return false
		}
		min, err1 := strconv.Atoi(strings.TrimSpace(parts[0]))
		max, err2 := strconv.Atoi(strings.TrimSpace(parts[1]))
		if err1 != nil || err2 != nil || min <= 0 || max < min {
			return false
		}
		return true
	}
	n, err := strconv.Atoi(value)
	return err == nil && n > 0
}
