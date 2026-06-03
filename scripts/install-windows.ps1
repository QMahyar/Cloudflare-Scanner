#Requires -Version 5.1
# Cloudflare Scanner — one-liner installer for Windows
# Run in PowerShell (as Administrator for PATH update):
#   irm https://raw.githubusercontent.com/QMahyar/Cloudflare-Scanner/master/scripts/install-windows.ps1 | iex

$ErrorActionPreference = 'Stop'

$Repo      = "QMahyar/Cloudflare-Scanner"
$InstallDir = "$env:LOCALAPPDATA\CloudflareScanner"

# ── detect architecture ──────────────────────────────────────────────────────
$arch = (Get-CimInstance -ClassName Win32_Processor -Property Architecture).Architecture
# 9 = x86-64, 12 = ARM64
$Platform = if ($arch -eq 12) { "windows-arm64" } else { "windows-amd64" }

# ── fetch latest release ─────────────────────────────────────────────────────
Write-Host "Fetching latest release..."
try {
    $release = Invoke-RestMethod "https://api.github.com/repos/$Repo/releases/latest"
    $Tag = $release.tag_name
} catch {
    Write-Error "Could not fetch release info: $_"
    exit 1
}

Write-Host "Installing Cloudflare Scanner $Tag ($Platform)..."

# ── download & extract ───────────────────────────────────────────────────────
$Zip = Join-Path $env:TEMP "Cloudflare-Scanner-${Tag}-${Platform}.zip"
$Url = "https://github.com/$Repo/releases/download/$Tag/Cloudflare-Scanner-${Tag}-${Platform}.zip"

Write-Host "Downloading from $Url ..."
Invoke-WebRequest $Url -OutFile $Zip -UseBasicParsing

New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null
Expand-Archive -LiteralPath $Zip -DestinationPath $InstallDir -Force
Remove-Item $Zip

# ── add to user PATH ─────────────────────────────────────────────────────────
$UserPath = [Environment]::GetEnvironmentVariable("PATH", "User") -split ";" | Where-Object { $_ }
if ($InstallDir -notin $UserPath) {
    $NewPath = ($UserPath + $InstallDir) -join ";"
    [Environment]::SetEnvironmentVariable("PATH", $NewPath, "User")
    Write-Host "Added $InstallDir to your PATH."
}

Write-Host ""
Write-Host "Done! Restart your terminal, then run:  Cloudflare-Scanner"
Write-Host "Or launch directly:  $InstallDir\Cloudflare-Scanner.exe"
Write-Host ""
Write-Host "Update:    re-run this installer"
Write-Host "Uninstall: Remove-Item -Recurse $InstallDir"
