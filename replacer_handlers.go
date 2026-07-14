package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
)

type replacerFetchRequest struct {
	URL string `json:"url"`
}

type replacerConfigEntry struct {
	Fingerprint   string `json:"fingerprint"`
	Protocol      string `json:"protocol"`
	UUID          string `json:"uuid"`
	Address       string `json:"address"`
	Port          int    `json:"port"`
	Encryption    string `json:"encryption"`
	Security      string `json:"security"`
	SNI           string `json:"sni"`
	FingerprintFP string `json:"fingerprint_fp"`
	Network       string `json:"network"`
	Host          string `json:"host"`
	Path          string `json:"path"`
	PacketEnc     string `json:"packet_enc"`
	Remark        string `json:"remark"`
	Flow          string `json:"flow,omitempty"`
	PublicKey     string `json:"pbk,omitempty"`
	ShortId       string `json:"sid,omitempty"`
	SpiderX       string `json:"spx,omitempty"`
	AllowInsecure bool   `json:"allow_insecure,omitempty"`
	ALPN          string `json:"alpn,omitempty"`
	HeaderType    string `json:"header_type,omitempty"`
	Mode          string `json:"mode,omitempty"`
	ServiceName   string `json:"service_name,omitempty"`
	MaxEarlyData  int    `json:"max_early_data,omitempty"`
	EarlyDataHdr  string `json:"early_data_header,omitempty"`
}

// proxyConfigToEntry / toProxyConfig are the single source of truth for the
// ProxyConfig <-> replacerConfigEntry mapping used by the replacer handlers.
func proxyConfigToEntry(c *ProxyConfig) replacerConfigEntry {
	return replacerConfigEntry{
		Fingerprint:   ConfigFingerprint(c),
		Protocol:      c.Protocol,
		UUID:          c.UUID,
		Address:       c.Address,
		Port:          c.Port,
		Encryption:    c.Encryption,
		Security:      c.Security,
		SNI:           c.SNI,
		FingerprintFP: c.Fingerprint,
		Network:       c.Network,
		Host:          c.Host,
		Path:          c.Path,
		PacketEnc:     c.PacketEncoding,
		Remark:        c.Remark,
		Flow:          c.Flow,
		PublicKey:     c.PublicKey,
		ShortId:       c.ShortId,
		SpiderX:       c.SpiderX,
		AllowInsecure: c.AllowInsecure,
		ALPN:          c.ALPN,
		HeaderType:    c.HeaderType,
		Mode:          c.Mode,
		ServiceName:   c.ServiceName,
		MaxEarlyData:  c.MaxEarlyData,
		EarlyDataHdr:  c.EarlyDataHeaderName,
	}
}

func (e replacerConfigEntry) toProxyConfig() *ProxyConfig {
	return &ProxyConfig{
		Protocol:            e.Protocol,
		UUID:                e.UUID,
		Address:             e.Address,
		Port:                e.Port,
		Encryption:          e.Encryption,
		Security:            e.Security,
		SNI:                 e.SNI,
		Fingerprint:         e.FingerprintFP,
		Network:             e.Network,
		Host:                e.Host,
		Path:                e.Path,
		PacketEncoding:      e.PacketEnc,
		Remark:              e.Remark,
		Flow:                e.Flow,
		PublicKey:           e.PublicKey,
		ShortId:             e.ShortId,
		SpiderX:             e.SpiderX,
		AllowInsecure:       e.AllowInsecure,
		ALPN:                e.ALPN,
		HeaderType:          e.HeaderType,
		Mode:                e.Mode,
		ServiceName:         e.ServiceName,
		MaxEarlyData:        e.MaxEarlyData,
		EarlyDataHeaderName: e.EarlyDataHdr,
	}
}

func handleReplacerFetch(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var req replacerFetchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, err.Error(), 400)
		return
	}

	if req.URL == "" {
		jsonError(w, "url required", 400)
		return
	}

	content, err := FetchSubscription(req.URL)
	if err != nil {
		jsonError(w, fmt.Sprintf("fetch: %v", err), 400)
		return
	}

	configs, err := ParseSubscription(content)
	if err != nil {
		jsonError(w, fmt.Sprintf("parse: %v", err), 400)
		return
	}

	if len(configs) == 0 {
		jsonError(w, "no valid configs found in subscription", 400)
		return
	}

	unique := DeduplicateConfigs(configs)

	entries := make([]replacerConfigEntry, 0, len(unique))
	for _, c := range unique {
		entries = append(entries, proxyConfigToEntry(c))
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"configs": entries,
		"total":   len(configs),
		"unique":  len(unique),
	})
}

type replacerParseRequest struct {
	Raw string `json:"raw"`
}

func handleReplacerParse(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 10<<20)
	var req replacerParseRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, err.Error(), 400)
		return
	}

	if req.Raw == "" {
		jsonError(w, "raw text required", 400)
		return
	}

	configs := ParseRawConfigs(req.Raw)

	if len(configs) == 0 {
		jsonError(w, "no valid configs found in text", 400)
		return
	}

	unique := DeduplicateConfigs(configs)

	entries := make([]replacerConfigEntry, 0, len(unique))
	for _, c := range unique {
		entries = append(entries, proxyConfigToEntry(c))
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"configs": entries,
		"total":   len(configs),
		"unique":  len(unique),
	})
}

type replacerApplyRequest struct {
	Configs      []replacerConfigEntry `json:"configs"`
	Endpoints    []string              `json:"endpoints"`
	NameTemplate string                `json:"name_template"`
}

func handleReplacerApply(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 10<<20)
	var req replacerApplyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, err.Error(), 400)
		return
	}

	if len(req.Configs) == 0 {
		jsonError(w, "no configs provided", 400)
		return
	}
	if len(req.Endpoints) == 0 {
		jsonError(w, "no endpoints provided", 400)
		return
	}

	if len(req.Configs)*len(req.Endpoints) > maxReplacerOutputs {
		jsonError(w, fmt.Sprintf("too many outputs requested (max %d)", maxReplacerOutputs), 400)
		return
	}

	configs := make([]*ProxyConfig, 0, len(req.Configs))
	for _, e := range req.Configs {
		cfg := e.toProxyConfig()
		if cfg.Protocol != "vless" && cfg.Protocol != "trojan" && cfg.Protocol != "vmess" {
			jsonError(w, "unsupported config protocol", 400)
			return
		}
		if cfg.UUID == "" {
			jsonError(w, "config UUID/password required", 400)
			return
		}
		configs = append(configs, cfg)
	}

	endpoints := make([]string, 0, len(req.Endpoints))
	for _, ep := range req.Endpoints {
		ep = strings.TrimSpace(ep)
		if ep == "" {
			continue
		}
		host, portStr, err := net.SplitHostPort(ep)
		port, perr := strconv.Atoi(portStr)
		if err != nil || perr != nil || host == "" || port < 1 || port > 65535 {
			jsonError(w, fmt.Sprintf("invalid endpoint: %s", ep), 400)
			return
		}
		endpoints = append(endpoints, ep)
	}
	if len(endpoints) == 0 {
		jsonError(w, "no valid endpoints provided", 400)
		return
	}

	urls := GenerateReplacedConfigsNamed(configs, endpoints, req.NameTemplate)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"urls":         urls,
		"count":        len(urls),
		"subscription": base64.StdEncoding.EncodeToString([]byte(strings.Join(urls, "\n"))),
	})
}
