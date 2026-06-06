package main

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

type WarpConfig struct {
	PrivateKey string
	Addresses  []string
	PublicKey  string
	Reserved   []int
	MTU        int
}

func ParseWarpConfig(path string) (*WarpConfig, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("cannot open config: %w", err)
	}
	defer file.Close()

	cfg := &WarpConfig{
		Reserved: []int{0, 0, 0},
		MTU:      1280,
	}

	scanner := bufio.NewScanner(file)
	var section string

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = strings.ToLower(line)
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.ToLower(strings.TrimSpace(parts[0]))
		val := strings.TrimSpace(parts[1])

		switch section {
		case "[interface]":
			switch key {
			case "privatekey":
				cfg.PrivateKey = val
			case "address":
				for _, a := range strings.Split(val, ",") {
					a = strings.TrimSpace(a)
					if a != "" {
						if !strings.Contains(a, "/") && strings.Contains(a, ":") {
							a += "/128"
						}
						cfg.Addresses = append(cfg.Addresses, a)
					}
				}
			case "reserved":
				cfg.Reserved = cfg.Reserved[:0]
				for _, b := range strings.Split(val, ",") {
					n, _ := strconv.Atoi(strings.TrimSpace(b))
					cfg.Reserved = append(cfg.Reserved, n)
				}
			case "s1":
				if len(cfg.Reserved) > 0 && cfg.Reserved[0] == 0 {
					n, _ := strconv.Atoi(val)
					cfg.Reserved[0] = n
				}
			case "s2":
				if len(cfg.Reserved) > 1 && cfg.Reserved[1] == 0 {
					n, _ := strconv.Atoi(val)
					cfg.Reserved[1] = n
				}
			case "s3":
				if len(cfg.Reserved) > 2 && cfg.Reserved[2] == 0 {
					n, _ := strconv.Atoi(val)
					cfg.Reserved[2] = n
				}
			case "mtu":
				n, _ := strconv.Atoi(val)
				if n > 0 {
					cfg.MTU = n
				}
			}
		case "[peer]":
			switch key {
			case "publickey":
				cfg.PublicKey = val
			case "endpoint":
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	if cfg.PrivateKey == "" {
		return nil, fmt.Errorf("config missing [Interface] PrivateKey")
	}
	if cfg.PublicKey == "" {
		return nil, fmt.Errorf("config missing [Peer] PublicKey")
	}
	if len(cfg.Addresses) == 0 {
		return nil, fmt.Errorf("config missing [Interface] Address")
	}

	for len(cfg.Reserved) < 3 {
		cfg.Reserved = append(cfg.Reserved, 0)
	}
	cfg.Reserved = cfg.Reserved[:3]

	return cfg, nil
}
