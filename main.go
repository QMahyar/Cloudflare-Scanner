package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

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

	port, err := startServer(xrayPath)
	if err != nil {
		fmt.Printf("Failed to start server: %v\n", err)
		os.Exit(1)
	}

	url := fmt.Sprintf("http://127.0.0.1:%d", port)

	fmt.Println()
	fmt.Println("  ╔══════════════════════════════════════════════════════╗")
	fmt.Println("  ║            Cloudflare Scanner v1.3                  ║")
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

	select {}
}

func isTermux() bool {
	_, ok := os.LookupEnv("PREFIX")
	return ok
}

func openBrowser(url string) {
	var argv0 string
	var args []string
	if runtime.GOOS == "linux" && isTermux() {
		argv0 = "termux-open-url"
		args = []string{argv0, url}
	} else {
		switch runtime.GOOS {
		case "windows":
			argv0 = "rundll32"
			args = []string{argv0, "url.dll,FileProtocolHandler", url}
		case "darwin":
			argv0 = "open"
			args = []string{argv0, url}
		default:
			argv0 = "xdg-open"
			args = []string{argv0, url}
		}
	}
	proc, err := os.StartProcess(argv0, args, &os.ProcAttr{
		Files: []*os.File{os.Stdin, os.Stdout, os.Stderr},
	})
	if err == nil {
		proc.Release()
	}
}
