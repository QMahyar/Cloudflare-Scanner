package main

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

type XrayConfig struct {
	Log       *LogConfig       `json:"log,omitempty"`
	Inbounds  []InboundConfig  `json:"inbounds"`
	Outbounds []OutboundConfig `json:"outbounds"`
	Routing   *RoutingConfig   `json:"routing,omitempty"`
}

type LogConfig struct {
	Access   string `json:"access"`
	Error    string `json:"error"`
	Loglevel string `json:"loglevel"`
}

type InboundConfig struct {
	Tag      string          `json:"tag"`
	Port     int             `json:"port"`
	Listen   string          `json:"listen"`
	Protocol string          `json:"protocol"`
	Settings json.RawMessage `json:"settings,omitempty"`
}

type OutboundConfig struct {
	Tag            string          `json:"tag"`
	Protocol       string          `json:"protocol"`
	Settings       json.RawMessage `json:"settings,omitempty"`
	StreamSettings json.RawMessage `json:"streamSettings,omitempty"`
	Mux            json.RawMessage `json:"mux,omitempty"`
}

type RoutingConfig struct {
	DomainStrategy string        `json:"domainStrategy"`
	Rules          []RoutingRule `json:"rules"`
}

type RoutingRule struct {
	Type        string   `json:"type"`
	InboundTag  []string `json:"inboundTag,omitempty"`
	OutboundTag string   `json:"outboundTag"`
}

type WireGuardSettings struct {
	SecretKey  string          `json:"secretKey"`
	Address    []string        `json:"address"`
	Peers      []WireGuardPeer `json:"peers"`
	Reserved   []int           `json:"reserved"`
	Noises     []NoiseEntry    `json:"noises,omitempty"`
	MTU        int             `json:"mtu,omitempty"`
	KernelMode bool            `json:"kernelMode,omitempty"`
}

type WireGuardPeer struct {
	PublicKey string `json:"publicKey"`
	Endpoint  string `json:"endpoint"`
}

type NoiseEntry struct {
	Type   string `json:"type"`
	Packet string `json:"packet"`
	Delay  string `json:"delay"`
}

type XrayManager struct {
	XrayPath string
	WorkDir  string
	Config   *WarpConfig
	Noise    NoiseConfig
}

func (xm *XrayManager) GenerateConfig(endpoint string, socksPort int) (configPath string, err error) {
	tag := fmt.Sprintf("wg_%d", socksPort)
	configDir := filepath.Join(xm.WorkDir, "_xray_work", tag)
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return "", fmt.Errorf("cannot create work dir: %w", err)
	}

	logFile := filepath.Join(configDir, "xray.log")
	_ = os.WriteFile(logFile, []byte{}, 0644)

	var noiseEntries []NoiseEntry
	if xm.Noise.Type != "" {
		noiseEntries = []NoiseEntry{
			{
				Type:   xm.Noise.Type,
				Packet: xm.Noise.Packet,
				Delay:  xm.Noise.Delay,
			},
		}
	}

	wgSettings := WireGuardSettings{
		SecretKey: xm.Config.PrivateKey,
		Address:   xm.Config.Addresses,
		Peers: []WireGuardPeer{
			{
				PublicKey: xm.Config.PublicKey,
				Endpoint:  endpoint,
			},
		},
		Reserved:   xm.Config.Reserved,
		Noises:     noiseEntries,
		MTU:        xm.Config.MTU,
		KernelMode: false,
	}

	wgJSON, _ := json.Marshal(wgSettings)

	socksSettings, _ := json.Marshal(map[string]interface{}{
		"auth": "noauth",
		"udp":  true,
	})

	cfg := XrayConfig{
		Log: &LogConfig{
			Access:   logFile,
			Error:    logFile,
			Loglevel: "warning",
		},
		Inbounds: []InboundConfig{
			{
				Tag:      "socks-in",
				Port:     socksPort,
				Listen:   "127.0.0.1",
				Protocol: "socks",
				Settings: socksSettings,
			},
		},
		Outbounds: []OutboundConfig{
			{
				Tag:      "warp",
				Protocol: "wireguard",
				Settings: wgJSON,
			},
			{
				Tag:      "direct",
				Protocol: "freedom",
			},
			{
				Tag:      "block",
				Protocol: "blackhole",
			},
		},
		Routing: &RoutingConfig{
			DomainStrategy: "AsIs",
			Rules: []RoutingRule{
				{
					Type:        "field",
					InboundTag:  []string{"socks-in"},
					OutboundTag: "warp",
				},
			},
		},
	}

	configJSON, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal config: %w", err)
	}

	configPath = filepath.Join(configDir, "config.json")
	if err := os.WriteFile(configPath, configJSON, 0644); err != nil {
		return "", fmt.Errorf("write config: %w", err)
	}

	return configPath, nil
}

func (xm *XrayManager) StartXray(configPath string) (*exec.Cmd, error) {
	cmd := exec.Command(xm.XrayPath, "run", "-c", configPath)
	cmd.Dir = filepath.Dir(configPath)

	stderrPath := filepath.Join(filepath.Dir(configPath), "stderr.log")
	f, err := os.Create(stderrPath)
	if err == nil {
		cmd.Stderr = f
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start xray: %w", err)
	}

	return cmd, nil
}

func (xm *XrayManager) WaitForPort(port int, timeout time.Duration) bool {
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", addr, 300*time.Millisecond)
		if err == nil {
			conn.Close()
			return true
		}
		time.Sleep(80 * time.Millisecond)
	}
	return false
}

func (xm *XrayManager) StopXray(cmd *exec.Cmd) {
	if cmd == nil || cmd.Process == nil {
		return
	}
	cmd.Process.Kill()
	cmd.Wait()
}
