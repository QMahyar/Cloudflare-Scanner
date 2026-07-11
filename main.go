package main

import (
	_ "embed"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
)

//go:embed VERSION
var versionFile string

// Version is injected at build time via -ldflags "-X 'main.Version=vX.Y.Z'".
// When unset (a plain `go build` with no ldflags), AppVersion falls back to the
// embedded VERSION file — the single source of truth for the version number.
var Version = ""

// AppVersion resolves the version string shown in the banner, the About tab,
// and the update check.
func AppVersion() string {
	if v := strings.TrimSpace(Version); v != "" {
		return v
	}
	if v := strings.TrimSpace(versionFile); v != "" {
		return "v" + strings.TrimPrefix(v, "v")
	}
	return "dev"
}

func main() {
	exePath, _ := os.Executable()
	workDir := filepath.Dir(exePath)

	xrayName := "xray"
	if runtime.GOOS == "windows" {
		xrayName = "xray.exe"
	}
	xrayPath := filepath.Join(workDir, xrayName)

	if _, err := os.Stat(xrayPath); os.IsNotExist(err) {
		fmt.Printf("%s not found. Place it next to this executable.\n", xrayName)
		fmt.Println("Download from: https://github.com/XTLS/Xray-core/releases")
		os.Exit(1)
	}

	// xray exists — confirm it actually runs. A broken binary otherwise fails
	// every clean-IP Phase-2 check silently; warn loudly but don't exit, since
	// the WARP scanner's native handshake path works without xray.
	if warn := VerifyXrayRunnable(xrayPath); warn != "" {
		fmt.Println()
		fmt.Println(warn)
		fmt.Println()
	}

	port, err := startServer(xrayPath)
	if err != nil {
		fmt.Printf("Failed to start server: %v\n", err)
		os.Exit(1)
	}

	url := fmt.Sprintf("http://127.0.0.1:%d", port)

	fmt.Println()
	fmt.Println("  ╔══════════════════════════════════════════════════════╗")
	fmt.Printf("  ║         Cloudflare Scanner %-25s║\n", AppVersion())
	fmt.Println("  ║    Open your browser to the URL below               ║")
	fmt.Println("  ╚══════════════════════════════════════════════════════╝")
	fmt.Println()
	fmt.Printf("  ➜  %s\n", url)
	fmt.Println()
	fmt.Println("  Endpoint Scanner  — Scan Warp WireGuard endpoints")
	fmt.Println("  IP Scanner        — Scan Cloudflare IPs (TCP + xray)")
	fmt.Println("  IP Replacer       — Fetch/paste configs, replace IP:port")
	fmt.Println()
	fmt.Println("  Close this window to stop the server.")
	fmt.Println()

	openBrowser(url)

	waitForShutdown()
}

// waitForShutdown blocks until an interrupt/terminate signal arrives, then
// removes the xray work dirs left behind by in-flight scans and exits cleanly.
func waitForShutdown() {
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	<-sig

	fmt.Println("\n  Shutting down — cleaning up xray work dirs...")
	os.RemoveAll(filepath.Join(os.TempDir(), "_xray_work"))
	os.RemoveAll(filepath.Join(os.TempDir(), "_xray_clean"))
	os.Exit(0)
}

func isTermux() bool {
	_, ok := os.LookupEnv("PREFIX")
	return ok
}

func openBrowserCmd(argv0 string, args []string) bool {
	proc, err := os.StartProcess(argv0, args, &os.ProcAttr{
		Files: []*os.File{os.Stdin, os.Stdout, os.Stderr},
	})
	if err != nil {
		fmt.Printf("  (browser open skipped: %v)\n", err)
		return false
	}
	proc.Release()
	return true
}

func openBrowser(url string) {
	if runtime.GOOS == "linux" && isTermux() {
		// ponytail: pass URL as argv directly — no shell, no injection
		if openBrowserCmd("termux-open-url", []string{"termux-open-url", url}) {
			return
		}
		if openBrowserCmd("am", []string{"am", "start", "--user", "0", "-a", "android.intent.action.VIEW", "-d", url}) {
			return
		}
		fmt.Println("  Could not auto-open browser on Termux.")
		fmt.Println("  Open the URL manually in your browser.")
		return
	}

	switch runtime.GOOS {
	case "windows":
		cmdPath := filepath.Join(os.Getenv("SystemRoot"), "System32", "cmd.exe")
		openBrowserCmd(cmdPath, []string{cmdPath, "/c", "start", "", url})
	case "darwin":
		openBrowserCmd("/usr/bin/open", []string{"/usr/bin/open", url})
	default:
		openBrowserCmd("/usr/bin/xdg-open", []string{"/usr/bin/xdg-open", url})
	}
}
