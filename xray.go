package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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
	Count  int    `json:"count,omitempty"`
}

type XrayManager struct {
	XrayPath string
	Config   *WarpConfig
	Noise    NoiseConfig
}

// GenerateConfigBatch builds a SINGLE xray config that probes a whole batch of
// WARP endpoints at once: one SOCKS inbound per endpoint, one WireGuard outbound
// per endpoint (identical credentials/noise, only the peer Endpoint differs), and
// a routing rule wiring each inbound to its outbound. The noise/AmneziaWG fallback
// otherwise spawns one xray PROCESS per endpoint; collapsing a batch into one
// process cuts that spawn cost by the batch factor while keeping each endpoint's
// probe independent. Returns the config path and per-endpoint SOCKS ports (aligned
// to the endpoints slice).
func (xm *XrayManager) GenerateConfigBatch(endpoints []string, basePort int) (configPath string, ports []int, err error) {
	if len(endpoints) == 0 {
		return "", nil, fmt.Errorf("no endpoints in batch")
	}

	tag := fmt.Sprintf("wgbatch_%d", basePort)
	configDir := filepath.Join(os.TempDir(), "_xray_work", tag)
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return "", nil, fmt.Errorf("cannot create work dir: %w", err)
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
				Count:  xm.Noise.Count,
			},
		}
	}

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
		Inbounds:  make([]InboundConfig, 0, len(endpoints)),
		Outbounds: make([]OutboundConfig, 0, len(endpoints)+2),
		Routing: &RoutingConfig{
			DomainStrategy: "AsIs",
			Rules:          make([]RoutingRule, 0, len(endpoints)),
		},
	}

	ports = make([]int, len(endpoints))
	for i, ep := range endpoints {
		port := basePort + i
		ports[i] = port
		inTag := fmt.Sprintf("in-%d", i)
		outTag := fmt.Sprintf("out-%d", i)

		wgSettings := WireGuardSettings{
			SecretKey: xm.Config.PrivateKey,
			Address:   xm.Config.Addresses,
			Peers: []WireGuardPeer{
				{
					PublicKey: xm.Config.PublicKey,
					Endpoint:  ep,
				},
			},
			Reserved:   xm.Config.Reserved,
			Noises:     noiseEntries,
			MTU:        xm.Config.MTU,
			KernelMode: false,
		}
		wgJSON, _ := json.Marshal(wgSettings)

		cfg.Inbounds = append(cfg.Inbounds, InboundConfig{
			Tag:      inTag,
			Port:     port,
			Listen:   "127.0.0.1",
			Protocol: "socks",
			Settings: socksSettings,
		})
		cfg.Outbounds = append(cfg.Outbounds, OutboundConfig{
			Tag:      outTag,
			Protocol: "wireguard",
			Settings: wgJSON,
		})
		cfg.Routing.Rules = append(cfg.Routing.Rules, RoutingRule{
			Type:        "field",
			InboundTag:  []string{inTag},
			OutboundTag: outTag,
		})
	}

	cfg.Outbounds = append(cfg.Outbounds,
		OutboundConfig{Tag: "direct", Protocol: "freedom"},
		OutboundConfig{Tag: "block", Protocol: "blackhole"},
	)

	configJSON, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return "", nil, fmt.Errorf("marshal config: %w", err)
	}

	configPath = filepath.Join(configDir, "config.json")
	if err := os.WriteFile(configPath, configJSON, 0644); err != nil {
		return "", nil, fmt.Errorf("write config: %w", err)
	}

	return configPath, ports, nil
}

func (xm *XrayManager) StartXray(configPath string) (*exec.Cmd, error) {
	cmd := exec.Command(xm.XrayPath, "run", "-c", configPath)
	cmd.Dir = filepath.Dir(configPath)

	stderrPath := filepath.Join(filepath.Dir(configPath), "stderr.log")
	f, err := os.Create(stderrPath)
	if err != nil {
		return nil, fmt.Errorf("create stderr log: %w", err)
	}
	cmd.Stderr = f

	if err := cmd.Start(); err != nil {
		f.Close()
		return nil, fmt.Errorf("start xray: %w", err)
	}

	// Close our handle; the OS keeps the file open via the child process.
	f.Close()
	return cmd, nil
}

func (xm *XrayManager) WaitForPortCtx(ctx context.Context, port int, timeout time.Duration) bool {
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return false
		default:
		}
		conn, err := net.DialTimeout("tcp", addr, 300*time.Millisecond)
		if err == nil {
			conn.Close()
			return true
		}
		select {
		case <-ctx.Done():
			return false
		case <-time.After(80 * time.Millisecond):
		}
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

// VerifyXrayRunnable confirms the xray binary actually executes — not just that
// the file exists (which is all main.go's os.Stat proves). A present-but-broken
// xray (wrong arch, AV-quarantined, missing libs) otherwise makes every clean-IP
// Phase-2 validation fail with the same opaque "startup timeout", so we probe it
// once at boot and surface a clear, actionable message. Returns "" when xray runs.
func VerifyXrayRunnable(xrayPath string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, xrayPath, "version").CombinedOutput()
	if ctx.Err() == context.DeadlineExceeded {
		return fmt.Sprintf("  ⚠ xray did not respond to `version` within 6s (%s) — Phase-2 validation may fail.", xrayPath)
	}
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			msg = err.Error()
		}
		return fmt.Sprintf("  ⚠ xray failed to run (%s): %s\n    Clean-IP Phase-2 validation needs a working xray. Re-download it from https://github.com/XTLS/Xray-core/releases", xrayPath, msg)
	}
	return ""
}
