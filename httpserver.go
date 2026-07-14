package main

import (
	"crypto/rand"
	"embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// ui/dist is git-ignored; run `cd frontend && npm run build` before `go build`.
//
//go:embed all:ui/dist
var uiFS embed.FS

// distFS is the embedded Vite build output (ui/dist) rooted so that
// "index.html" and "assets/..." resolve directly. Initialized in startServer.
var distFS fs.FS

const jobTTL = 10 * time.Minute

// maxScanCount caps the requested endpoint/IP count so a huge value can't drive
// the generators into multi-GB allocations (clean scan) or excessive work. The
// WARP generator is additionally bounded by its finite address pool.
const maxScanCount = 100000
const maxEndpointConcurrency = 2048
const maxCleanPhase1Probes = 4096
const maxCleanPhase2Probes = 256
const maxOutCount = 10000
const maxReplacerOutputs = 50000

// maxScanPorts caps the clean-scan port list. The port count multiplies the IP
// count into the endpoint slice ((v4+v6)*len(ports)), so an unbounded list — the
// one clean-scan input that wasn't clamped — could blow up allocation. 64 is far
// above the official CF CDN port set while keeping maxScanPorts*maxScanCount sane.
const maxScanPorts = 64
const csrfCookieName = "cfscanner_token"
const csrfHeaderName = "X-CSRF-Token"

var (
	scanJobs   = map[string]*ScanJob{}
	scanJobsMu sync.Mutex
	jobCounter int

	cleanJobs       = map[string]*CleanIPJob{}
	cleanJobsMu     sync.Mutex
	cleanJobCounter int

	// warpSocksPortBase hands out non-overlapping SOCKS port windows for the WARP
	// noise-fallback batches. Process-global (not per-job) so two WARP noise scans
	// running at once can't allocate the same window.
	warpSocksPortBase atomic.Int32
)

func scheduleScanJobCleanup(id string) {
	time.AfterFunc(jobTTL, func() {
		scanJobsMu.Lock()
		delete(scanJobs, id)
		scanJobsMu.Unlock()
	})
}

func scheduleCleanJobCleanup(id string) {
	time.AfterFunc(jobTTL, func() {
		cleanJobsMu.Lock()
		delete(cleanJobs, id)
		cleanJobsMu.Unlock()
	})
}

// stopAllJobs cancels every in-flight scan/clean job so their goroutines unwind
// and each xray BatchProbe's deferred Kill()/Wait() can run before the process
// exits. os.Exit skips defers, so on shutdown we must trigger the
// cancellation-driven cleanup explicitly or leak xray children + work dirs.
// Idempotent: job.stop() is sync.Once-guarded.
func stopAllJobs() {
	scanJobsMu.Lock()
	for _, j := range scanJobs {
		j.stop()
	}
	scanJobsMu.Unlock()

	cleanJobsMu.Lock()
	for _, j := range cleanJobs {
		j.stop()
	}
	cleanJobsMu.Unlock()
}

// failEntry is the shared failure row shape for scan/clean results responses.
type failEntry struct {
	Endpoint string `json:"endpoint"`
	Error    string `json:"error"`
}

func jsonError(w http.ResponseWriter, msg string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

// sanitizeScanPorts validates (1..65535), dedupes, and caps a requested port list
// to maxScanPorts, defaulting to {443} when nothing valid remains. Bounding the
// port count bounds the (v4+v6)*len(ports) endpoint allocation downstream.
func sanitizeScanPorts(ports []int) []int {
	out := make([]int, 0, len(ports))
	seen := make(map[int]bool, len(ports))
	for _, p := range ports {
		if p < 1 || p > 65535 || seen[p] {
			continue
		}
		seen[p] = true
		out = append(out, p)
		if len(out) >= maxScanPorts {
			break
		}
	}
	if len(out) == 0 {
		return []int{443}
	}
	return out
}

// validateEndpointHostPort enforces a strict host:port for values written into a
// .conf file: a numeric in-range port and a host with no control characters or
// whitespace. net.SplitHostPort alone accepts embedded newlines in the host, which
// would let a crafted endpoint inject extra lines into the generated WireGuard config.
func validateEndpointHostPort(endpoint string) error {
	host, portStr, err := net.SplitHostPort(endpoint)
	if err != nil {
		return fmt.Errorf("expected host:port: %w", err)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil || port < 1 || port > 65535 {
		return fmt.Errorf("port must be 1-65535")
	}
	if host == "" {
		return fmt.Errorf("host required")
	}
	for _, r := range host {
		if r < 0x20 || r == 0x7f || r == ' ' {
			return fmt.Errorf("host contains an invalid character")
		}
	}
	return nil
}

func clampInt(v, min, max int) int {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

func newCSRFToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// isLoopbackHost reports whether an HTTP Host header targets this loopback-only
// server. It rejects any other hostname so a malicious page that rebinds its own
// DNS name to 127.0.0.1 can't reach the API (the browser still sends the
// attacker's hostname in Host, never "127.0.0.1"). An empty Host is allowed: the
// rebinding vector is a browser, which always sends one.
func isLoopbackHost(hostHeader string) bool {
	if hostHeader == "" {
		return true
	}
	host := hostHeader
	if h, _, err := net.SplitHostPort(hostHeader); err == nil {
		host = h
	}
	host = strings.TrimPrefix(strings.TrimSuffix(host, "]"), "[")
	if host == "localhost" {
		return true
	}
	if ip := net.ParseIP(host); ip != nil && ip.IsLoopback() {
		return true
	}
	return false
}

func csrfMiddleware(next http.Handler, token string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !isLoopbackHost(r.Host) {
			jsonError(w, "forbidden", http.StatusForbidden)
			return
		}

		if r.URL.Path == "/" || r.URL.Path == "/index.html" {
			http.SetCookie(w, &http.Cookie{
				Name:     csrfCookieName,
				Value:    token,
				Path:     "/",
				SameSite: http.SameSiteStrictMode,
			})
		}

		needsToken := strings.HasPrefix(r.URL.Path, "/api/") && r.Method != http.MethodGet
		if r.URL.Path == "/api/select-output-dir" || r.URL.Path == "/api/update-check" {
			needsToken = true
		}
		if !needsToken {
			next.ServeHTTP(w, r)
			return
		}

		cookie, err := r.Cookie(csrfCookieName)
		if err != nil || cookie.Value == "" || cookie.Value != token || r.Header.Get(csrfHeaderName) != token {
			jsonError(w, "forbidden", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func startServer(xrayPath string) (*http.Server, int, error) {
	sub, err := fs.Sub(uiFS, "ui/dist")
	if err != nil {
		return nil, 0, fmt.Errorf("embed ui/dist: %w", err)
	}
	distFS = sub

	token, err := newCSRFToken()
	if err != nil {
		return nil, 0, fmt.Errorf("csrf token: %w", err)
	}

	mux := http.NewServeMux()

	mux.HandleFunc("/", handleIndex)
	mux.Handle("GET /assets/", http.FileServerFS(distFS))
	mux.HandleFunc("POST /api/scan", handleScanStart(xrayPath))
	mux.HandleFunc("GET /api/status/{id}", handleScanStatus)
	mux.HandleFunc("GET /api/scan-events/{id}", handleScanEvents)
	mux.HandleFunc("GET /api/results/{id}", handleScanResults)
	mux.HandleFunc("POST /api/stop/{id}", handleScanStop)
	mux.HandleFunc("POST /api/apply-endpoint", handleApplyEndpoint)
	mux.HandleFunc("GET /api/select-output-dir", handleSelectOutputDir)
	mux.HandleFunc("POST /api/clean-scan", handleCleanScanStart(xrayPath))
	mux.HandleFunc("GET /api/clean-status/{id}", handleCleanScanStatus)
	mux.HandleFunc("GET /api/clean-events/{id}", handleCleanScanEvents)
	mux.HandleFunc("GET /api/clean-results/{id}", handleCleanScanResults)
	mux.HandleFunc("POST /api/clean-stop/{id}", handleCleanScanStop)
	mux.HandleFunc("POST /api/clean-export", handleCleanExport)
	mux.HandleFunc("POST /api/replacer/fetch", handleReplacerFetch)
	mux.HandleFunc("POST /api/replacer/parse", handleReplacerParse)
	mux.HandleFunc("POST /api/replacer/apply", handleReplacerApply)
	mux.HandleFunc("GET /api/version", handleVersion)
	mux.HandleFunc("GET /api/update-check", handleUpdateCheck)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, 0, fmt.Errorf("listen: %w", err)
	}
	srv := &http.Server{
		Handler:           csrfMiddleware(mux, token),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	go func() {
		// srv.Shutdown makes Serve return http.ErrServerClosed; a closed listener
		// yields net.ErrClosed. Neither is a real error on graceful shutdown.
		if err := srv.Serve(listener); err != nil &&
			!errors.Is(err, net.ErrClosed) &&
			!errors.Is(err, http.ErrServerClosed) {
			fmt.Fprintf(os.Stderr, "server error: %v\n", err)
		}
	}()
	return srv, listener.Addr().(*net.TCPAddr).Port, nil
}

func handleIndex(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Referrer-Policy", "no-referrer")
	// Vite emits external module scripts, so script-src no longer needs
	// 'unsafe-inline'. style-src keeps it for Svelte's dynamic style= bindings
	// and inline styles; connect-src 'self' covers fetch/SSE to the API.
	w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; connect-src 'self'")
	data, err := fs.ReadFile(distFS, "index.html")
	if err != nil {
		http.Error(w, "UI unavailable", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(data)
}

// streamSSE drives a Server-Sent Events response from periodic snapshots of a
// job. It calls snapshot() immediately, then every 250ms, emitting a `data:`
// frame only when the JSON changes, and returns once snapshot reports done or
// the client disconnects. This is server-pushed polling: the scan goroutines
// stay untouched, but the client holds one connection instead of hammering the
// status endpoint a few times a second. Results stay on their own poll.
func streamSSE(w http.ResponseWriter, r *http.Request, snapshot func() (map[string]interface{}, bool)) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		jsonError(w, "streaming unsupported", 500)
		return
	}
	// The server-wide WriteTimeout (set in startServer) is a deadline on the
	// whole response, which is correct for normal JSON handlers but fatal for an
	// SSE stream that legitimately stays open for the entire scan (a noise scan
	// can run minutes). Once it fires mid-stream the connection is torn down and
	// the browser logs ERR_INCOMPLETE_CHUNKED_ENCODING. Clear the deadline for
	// this one response so the stream lives as long as the scan does; the
	// ctx.Done() select below still tears it down when the client disconnects.
	if rc := http.NewResponseController(w); rc != nil {
		_ = rc.SetWriteDeadline(time.Time{})
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no") // disable proxy buffering if any

	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	ctx := r.Context()

	var last string
	for {
		data, done := snapshot()
		b, _ := json.Marshal(data)
		if s := string(b); s != last {
			last = s
			fmt.Fprintf(w, "data: %s\n\n", s)
			flusher.Flush()
		}
		if done {
			return
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
