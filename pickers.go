package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

var errFolderSelectionCancelled = errors.New("folder selection cancelled")

func handleSelectOutputDir(w http.ResponseWriter, r *http.Request) {
	dir, err := selectOutputDir()
	if err != nil {
		if errors.Is(err, errFolderSelectionCancelled) {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{"cancelled": true})
			return
		}
		jsonError(w, fmt.Sprintf("folder picker failed: %v", err), 500)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"path": dir})
}

func selectOutputDir() (string, error) {
	switch runtime.GOOS {
	case "windows":
		return selectOutputDirWindows()
	case "darwin":
		return selectOutputDirDarwin()
	default:
		return selectOutputDirLinux()
	}
}

func selectOutputDirWindows() (string, error) {
	script := `Add-Type -AssemblyName System.Windows.Forms; $dlg = New-Object System.Windows.Forms.FolderBrowserDialog; $dlg.Description = "Select output folder"; $dlg.ShowNewFolderButton = $true; if ($dlg.ShowDialog() -eq [System.Windows.Forms.DialogResult]::OK) { [Console]::Out.Write($dlg.SelectedPath) } else { exit 2 }`
	// Resolve powershell.exe by absolute path rather than via PATH lookup, so a
	// stray powershell.exe in an untrusted working/PATH directory can't be run.
	// Mirrors the absolute cmd.exe path used in openBrowser (main.go).
	psPath := filepath.Join(os.Getenv("SystemRoot"), "System32", "WindowsPowerShell", "v1.0", "powershell.exe")
	if _, err := os.Stat(psPath); err != nil {
		psPath = "powershell" // fall back to PATH if SystemRoot layout is unusual
	}
	cmd := exec.Command(psPath, "-NoProfile", "-NonInteractive", "-Command", script)
	out, err := cmd.CombinedOutput()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 2 {
			return "", errFolderSelectionCancelled
		}
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			msg = err.Error()
		}
		return "", errors.New(msg)
	}
	path := strings.TrimSpace(string(out))
	if path == "" {
		return "", errFolderSelectionCancelled
	}
	return filepath.Clean(path), nil
}

func selectOutputDirDarwin() (string, error) {
	cmd := exec.Command("/usr/bin/osascript",
		"-e", `set selectedFolder to choose folder with prompt "Select output folder"`,
		"-e", `POSIX path of selectedFolder`,
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		msg := strings.ToLower(strings.TrimSpace(string(out)))
		if strings.Contains(msg, "user canceled") || strings.Contains(msg, "cancelled") {
			return "", errFolderSelectionCancelled
		}
		if msg == "" {
			msg = err.Error()
		}
		return "", errors.New(msg)
	}
	path := strings.TrimSpace(string(out))
	if path == "" {
		return "", errFolderSelectionCancelled
	}
	return filepath.Clean(path), nil
}

func selectOutputDirLinux() (string, error) {
	type pickerCmd struct {
		name string
		args []string
	}
	candidates := []pickerCmd{
		{name: "zenity", args: []string{"--file-selection", "--directory", "--title=Select output folder"}},
		{name: "kdialog", args: []string{"--getexistingdirectory", ".", "--title", "Select output folder"}},
	}
	for _, c := range candidates {
		cmd := exec.Command(c.name, c.args...)
		out, err := cmd.CombinedOutput()
		if err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
				return "", errFolderSelectionCancelled
			}
			continue
		}
		path := strings.TrimSpace(string(out))
		if path == "" {
			return "", errFolderSelectionCancelled
		}
		return filepath.Clean(path), nil
	}
	return "", errors.New("no supported folder picker found (install zenity or kdialog)")
}
