package main

import (
	"archive/zip"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const xrayVersion = "v1.8.24"

func DownloadXray(destDir string) (string, error) {
	arch := runtime.GOARCH
	goos := runtime.GOOS

	archStr := map[string]string{
		"amd64": "64",
		"386":   "32",
		"arm64": "arm64-v8a",
	}[arch]

	if archStr == "" {
		return "", fmt.Errorf("unsupported arch: %s", arch)
	}

	filename := fmt.Sprintf("Xray-%s-%s.zip", goos, archStr)
	if goos == "windows" {
		filename = fmt.Sprintf("Xray-windows-%s.zip", archStr)
	}

	url := fmt.Sprintf("https://github.com/XTLS/Xray-core/releases/download/%s/%s", xrayVersion, filename)

	fmt.Printf("Downloading %s ...\n", url)

	zipPath := filepath.Join(destDir, filename)

	resp, err := http.Get(url)
	if err != nil {
		return "", fmt.Errorf("download: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	out, err := os.Create(zipPath)
	if err != nil {
		return "", fmt.Errorf("create file: %w", err)
	}

	if _, err := io.Copy(out, resp.Body); err != nil {
		out.Close()
		return "", fmt.Errorf("save: %w", err)
	}
	out.Close()

	binName := "xray"
	if goos == "windows" {
		binName = "xray.exe"
	}

	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		return "", fmt.Errorf("open zip: %w", err)
	}
	defer zr.Close()

	for _, f := range zr.File {
		if strings.HasSuffix(f.Name, binName) {
			rc, err := f.Open()
			if err != nil {
				zr.Close()
				return "", fmt.Errorf("open entry: %w", err)
			}

			dest := filepath.Join(destDir, binName)
			dst, err := os.Create(dest)
			if err != nil {
				rc.Close()
				zr.Close()
				return "", fmt.Errorf("create binary: %w", err)
			}

			if _, err := io.Copy(dst, rc); err != nil {
				dst.Close()
				rc.Close()
				zr.Close()
				return "", fmt.Errorf("extract: %w", err)
			}

			dst.Close()
			rc.Close()
			zr.Close()
			os.Remove(zipPath)
			return dest, nil
		}
	}

	return "", fmt.Errorf("%s not found in zip", binName)
}
