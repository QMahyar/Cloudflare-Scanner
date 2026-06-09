package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// GitHubRepo is the "owner/name" slug used for the source link and the update
// check.
const GitHubRepo = "QMahyar/Cloudflare-Scanner"

// handleVersion reports the running build version and the source repo so the UI
// has a single source of truth for the GitHub link.
func handleVersion(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"version":  AppVersion(),
		"repo":     GitHubRepo,
		"repo_url": "https://github.com/" + GitHubRepo,
	})
}

// handleUpdateCheck queries the GitHub "latest release" API server-side (so the
// page's CSP can stay locked to 'self') and reports whether a newer tag exists.
func handleUpdateCheck(w http.ResponseWriter, r *http.Request) {
	client := &http.Client{Timeout: 10 * time.Second}
	api := "https://api.github.com/repos/" + GitHubRepo + "/releases/latest"
	req, err := http.NewRequest("GET", api, nil)
	if err != nil {
		jsonError(w, fmt.Sprintf("build request: %v", err), 500)
		return
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "Cloudflare-Scanner")

	resp, err := client.Do(req)
	if err != nil {
		jsonError(w, fmt.Sprintf("update check failed: %v", err), 502)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		jsonError(w, fmt.Sprintf("GitHub returned %d", resp.StatusCode), 502)
		return
	}

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	var rel struct {
		TagName string `json:"tag_name"`
		HTMLURL string `json:"html_url"`
		Name    string `json:"name"`
	}
	if err := json.Unmarshal(body, &rel); err != nil {
		jsonError(w, "parse GitHub response", 502)
		return
	}

	latest := strings.TrimSpace(rel.TagName)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"current":          AppVersion(),
		"latest":           latest,
		"url":              rel.HTMLURL,
		"name":             rel.Name,
		"update_available": isNewerVersion(AppVersion(), latest),
	})
}

// isNewerVersion reports whether latest > current via a dotted-numeric compare.
// A non-release current (e.g. "dev") is treated as older than any tagged release.
func isNewerVersion(current, latest string) bool {
	c := parseVer(current)
	l := parseVer(latest)
	if c == nil {
		return latest != ""
	}
	if l == nil {
		return false
	}
	for i := 0; i < len(c) || i < len(l); i++ {
		var cv, lv int
		if i < len(c) {
			cv = c[i]
		}
		if i < len(l) {
			lv = l[i]
		}
		if lv > cv {
			return true
		}
		if lv < cv {
			return false
		}
	}
	return false
}

// parseVer turns "v3.1.0" / "3.1.0-rc1" into [3,1,0]; returns nil for dev/unknown.
func parseVer(s string) []int {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "v")
	s = strings.TrimPrefix(s, "V")
	if s == "" || s == "dev" {
		return nil
	}
	if i := strings.IndexAny(s, "-+ "); i >= 0 {
		s = s[:i]
	}
	parts := strings.Split(s, ".")
	out := make([]int, 0, len(parts))
	for _, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil {
			return nil
		}
		out = append(out, n)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
