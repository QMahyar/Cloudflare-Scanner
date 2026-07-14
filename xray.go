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
	"sync"
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

// buildBatchXrayConfig writes ONE xray config probing every endpoint in the batch:
// a SOCKS inbound (in-%d, noauth, udp) + one outbound (out-%d, from makeOutbound) +
// a routing rule per endpoint, plus trailing direct/block outbounds. dirName is the
// subdir under os.TempDir() (e.g. "_xray_work/wgbatch_<port>"). Returns the config
// path and per-endpoint SOCKS ports aligned to endpoints. Caller owns dir cleanup.
//
// This is the single implementation shared by the WARP noise-fallback
// (GenerateConfigBatch) and clean-IP Phase-2 (BuildXrayJSONBatch) paths — only the
// per-endpoint outbound and the dir name differ between them.
func buildBatchXrayConfig(endpoints []string, basePort int, dirName string,
	makeOutbound func(ep, outTag string) (OutboundConfig, error)) (configPath string, ports []int, err error) {

	if len(endpoints) == 0 {
		return "", nil, fmt.Errorf("no endpoints in batch")
	}

	configDir := filepath.Join(os.TempDir(), dirName)
	if err := os.MkdirAll(configDir, 0700); err != nil {
		return "", nil, fmt.Errorf("cannot create work dir: %w", err)
	}

	logFile := filepath.Join(configDir, "xray.log")
	_ = os.WriteFile(logFile, []byte{}, 0600)

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

		ob, err := makeOutbound(ep, outTag)
		if err != nil {
			return "", nil, err
		}

		cfg.Inbounds = append(cfg.Inbounds, InboundConfig{
			Tag:      inTag,
			Port:     port,
			Listen:   "127.0.0.1",
			Protocol: "socks",
			Settings: socksSettings,
		})
		cfg.Outbounds = append(cfg.Outbounds, ob)
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
	if err := os.WriteFile(configPath, configJSON, 0600); err != nil {
		return "", nil, fmt.Errorf("write config: %w", err)
	}

	return configPath, ports, nil
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

	dirName := filepath.Join("_xray_work", fmt.Sprintf("wgbatch_%d", basePort))
	return buildBatchXrayConfig(endpoints, basePort, dirName,
		func(ep, outTag string) (OutboundConfig, error) {
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
			return OutboundConfig{
				Tag:      outTag,
				Protocol: "wireguard",
				Settings: wgJSON,
			}, nil
		})
}

// probeResult is the minimal result shape the shared batch lifecycle returns.
// Callers map this to their own result types (CleanIPResult, ScanResult).
type probeResult struct {	Endpoint string
	Latency  time.Duration
	Success  bool
	Error    string
}

// probeFunc is the per-endpoint probe function the caller supplies.
type probeFunc func(ctx context.Context, endpoint string, socksPort int) probeResult

// BatchProbe runs the shared xray batch lifecycle: start process, wait for
// port readiness, probe every endpoint concurrently, kill process, clean up.
// The caller supplies a config (already written to disk) and a probe function
// that tests one endpoint against its assigned SOCKS port. Returns results
// aligned to the endpoints slice. The caller owns the config dir cleanup.
func BatchProbe(ctx context.Context, xrayPath, configPath string, endpoints []string, basePort int, timeout time.Duration, probe probeFunc) []probeResult {
	results := make([]probeResult, len(endpoints))
	for i, ep := range endpoints {
		results[i] = probeResult{Endpoint: ep, Error: "not run"}
	}

	select {
	case <-ctx.Done():
		for i := range results {
			results[i] = probeResult{Endpoint: endpoints[i], Error: "cancelled"}
		}
		return results
	default:
	}

	cmd := exec.Command(xrayPath, "run", "-c", configPath)
	cmd.Dir = filepath.Dir(configPath)
	stderrPath := filepath.Join(filepath.Dir(configPath), "stderr.log")
	if f, ferr := os.Create(stderrPath); ferr == nil {
		cmd.Stderr = f
		defer f.Close()
	}

	if err := cmd.Start(); err != nil {
		for i := range results {
			results[i] = probeResult{Endpoint: endpoints[i], Error: fmt.Sprintf("start xray: %v", err)}
		}
		return results
	}
	defer func() {
		if cmd.Process != nil {
			cmd.Process.Kill()
			cmd.Wait()
		}
	}()

	// Wait for the highest inbound port — xray binds in order.
	lastAddr := fmt.Sprintf("127.0.0.1:%d", basePort+len(endpoints)-1)
	startupDeadline := time.Now().Add(6 * time.Second)
	started := false
	for time.Now().Before(startupDeadline) {
		select {
		case <-ctx.Done():
			for i := range results {
				results[i] = probeResult{Endpoint: endpoints[i], Error: "cancelled"}
			}
			return results
		default:
		}
		conn, derr := net.DialTimeout("tcp", lastAddr, 300*time.Millisecond)
		if derr == nil {
			conn.Close()
			started = true
			break
		}
		select {
		case <-ctx.Done():
			for i := range results {
				results[i] = probeResult{Endpoint: endpoints[i], Error: "cancelled"}
			}
			return results
		case <-time.After(80 * time.Millisecond):
		}
	}
	if !started {
		for i := range results {
			results[i] = probeResult{Endpoint: endpoints[i], Error: "xray startup timeout"}
		}
		return results
	}

	// Probe every endpoint concurrently.
	var wg sync.WaitGroup
	for i := range endpoints {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			select {
			case <-ctx.Done():
				results[idx] = probeResult{Endpoint: endpoints[idx], Error: "cancelled"}
				return
			default:
			}
			results[idx] = probe(ctx, endpoints[idx], basePort+idx)
		}(i)
	}
	wg.Wait()

	return results
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
