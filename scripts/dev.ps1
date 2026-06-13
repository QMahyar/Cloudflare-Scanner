#Requires -Version 5.1
<#
.SYNOPSIS
  Cloudflare Scanner - fast LOCAL dev build + run loop (Windows / PowerShell).

.DESCRIPTION
  Optimised for the inner dev loop, NOT release packaging. For release-identical
  archives use scripts\build.ps1 instead. dev.ps1:

    * rebuilds the Svelte UI only when frontend\src changed (make-style staleness
      check against ui\dist) - skip the slow npm step on Go-only changes
    * builds the Go binary into builds\<platform>\
    * drops the matching xray-core sidecar next to it, cached under
      builds\.cache\ so it is downloaded once and reused
    * optionally launches the app (-Run)

  Everything lands in builds\ (git-ignored). The binary finds xray next to
  itself, so builds\<platform>\ is a complete, runnable folder.

.PARAMETER Platform
  Target platform key (default: host). One of:
    windows-amd64  windows-arm64  linux-amd64  linux-arm64  darwin-amd64  darwin-arm64
  Cross-built targets are produced but cannot be -Run on a Windows host.

.PARAMETER SkipUI   Never run npm (Go-only rebuild), even if the UI is stale.
.PARAMETER ForceUI  Always rebuild the UI, even if it looks up to date.
.PARAMETER Run      Launch the freshly built app after building (host platform only).
.PARAMETER Clean    Wipe builds\<platform>\ before building.

.EXAMPLE
  .\scripts\dev.ps1                 # build host platform into builds\windows-amd64\
  .\scripts\dev.ps1 -Run            # build, then launch
  .\scripts\dev.ps1 -SkipUI -Run    # Go-only rebuild + run (fastest loop)
  .\scripts\dev.ps1 -ForceUI        # force a UI rebuild
  .\scripts\dev.ps1 -Platform linux-amd64   # cross-build (no -Run)

.NOTES
  Env overrides: $env:XRAY_VERSION (default v1.8.24), $env:VERSION.
#>
[CmdletBinding()]
param(
  [string]$Platform = '',
  [switch]$SkipUI,
  [switch]$ForceUI,
  [switch]$Run,
  [switch]$Clean
)

$ErrorActionPreference = 'Stop'

$XrayVersion = if ($env:XRAY_VERSION) { $env:XRAY_VERSION } else { 'v1.8.24' }
$App = 'Cloudflare-Scanner'

# Resolve repo root from this script's own location, independent of CWD.
$ScriptDir = $PSScriptRoot
if (-not $ScriptDir) { $ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path }
$RepoRoot = Split-Path -Parent $ScriptDir
$Builds   = Join-Path $RepoRoot 'builds'
$Cache    = Join-Path $Builds '.cache\xray'
Set-Location $RepoRoot

function Log  ($m) { Write-Host "==> $m" -ForegroundColor Cyan }
function Ok   ($m) { Write-Host "  ok $m" -ForegroundColor Green }
function Warn ($m) { Write-Host "  !  $m" -ForegroundColor Yellow }
function Die  ($m) { Write-Error $m; exit 1 }

# Platform matrix mirrors .github\workflows\release.yml (xray zip names + the
# binary name inside each archive).
$Matrix = @(
  @{ key='windows-amd64'; goos='windows'; goarch='amd64'; ext='.exe'; xray_in='xray.exe'; xray_zip='Xray-windows-64.zip' }
  @{ key='windows-arm64'; goos='windows'; goarch='arm64'; ext='.exe'; xray_in='xray.exe'; xray_zip='Xray-windows-arm64-v8a.zip' }
  @{ key='linux-amd64';   goos='linux';   goarch='amd64'; ext='';     xray_in='xray';     xray_zip='Xray-linux-64.zip' }
  @{ key='linux-arm64';   goos='linux';   goarch='arm64'; ext='';     xray_in='xray';     xray_zip='Xray-linux-arm64-v8a.zip' }
  @{ key='darwin-amd64';  goos='darwin';  goarch='amd64'; ext='';     xray_in='xray';     xray_zip='Xray-macos-64.zip' }
  @{ key='darwin-arm64';  goos='darwin';  goarch='arm64'; ext='';     xray_in='xray';     xray_zip='Xray-macos-arm64-v8a.zip' }
)
function Row-For ($key) { $Matrix | Where-Object { $_.key -eq $key } | Select-Object -First 1 }
function Detect-Host {
  $a = (Get-CimInstance Win32_Processor -Property Architecture).Architecture
  if ($a -eq 12) { 'windows-arm64' } else { 'windows-amd64' }
}

if (-not $Platform) { $Platform = Detect-Host }
$row = Row-For $Platform
if (-not $row) { Die "unknown platform '$Platform'. Known: $(($Matrix.key) -join ', ')" }

if (-not (Get-Command go -ErrorAction SilentlyContinue)) {
  Die 'Go not found on PATH. Install Go, or use scripts\build.ps1 (which auto-installs a local copy).'
}

# Version stamp: VERSION file + short sha (lightweight; build.ps1 owns the
# release-grade scheme). $env:VERSION overrides.
$Version = $env:VERSION
if (-not $Version) {
  $verFile = Join-Path $RepoRoot 'VERSION'
  $base = if (Test-Path $verFile) { (Get-Content $verFile -Raw).Trim() } else { 'dev' }
  $Version = 'v' + ($base -replace '^v', '')
  $sha = (& git rev-parse --short HEAD 2>$null)
  if ($sha) { $Version = "$Version-dev.g$sha" }
}

Log "dev build  -  $Platform, version $Version, xray $XrayVersion"

# ── 1. UI: rebuild only when frontend sources are newer than the bundle ──────
function Test-UIStale {
  $distIndex = Join-Path $RepoRoot 'ui\dist\index.html'
  if (-not (Test-Path $distIndex)) { return $true }
  $distTime = (Get-Item $distIndex).LastWriteTimeUtc
  $srcs = @(
    'frontend\src', 'frontend\index.html', 'frontend\package.json',
    'frontend\package-lock.json', 'frontend\vite.config.js', 'frontend\svelte.config.js'
  )
  $newest = $null
  foreach ($s in $srcs) {
    $p = Join-Path $RepoRoot $s
    if (Test-Path $p) {
      $m = (Get-ChildItem $p -Recurse -File -ErrorAction SilentlyContinue |
            Measure-Object -Property LastWriteTimeUtc -Maximum).Maximum
      if ($m -and (-not $newest -or $m -gt $newest)) { $newest = $m }
    }
  }
  return ($newest -and $newest -gt $distTime)
}

$buildUI = if ($SkipUI) { $false } elseif ($ForceUI) { $true } else { Test-UIStale }
if ($buildUI) {
  Log 'Building UI (npm run build)'
  Push-Location (Join-Path $RepoRoot 'frontend')
  try {
    if (-not (Test-Path 'node_modules')) {
      Warn 'node_modules missing - running npm install'
      & npm install
      if ($LASTEXITCODE -ne 0) { Die 'npm install failed' }
    }
    & npm run build
    if ($LASTEXITCODE -ne 0) { Die 'npm run build failed' }
  } finally { Pop-Location }
  Ok 'UI bundle -> ui\dist'
} else {
  Ok 'UI up to date - skipped (use -ForceUI to force a rebuild)'
}

# ── 2. Go binary -> builds\<platform>\ ───────────────────────────────────────
$outdir = Join-Path $Builds $Platform
if ($Clean -and (Test-Path $outdir)) { Remove-Item -Recurse -Force $outdir }
New-Item -ItemType Directory -Force -Path $outdir | Out-Null
$binname = "$App$($row.ext)"
$binpath = Join-Path $outdir $binname

Log "Building binary (GOOS=$($row.goos) GOARCH=$($row.goarch))"
$env:GOOS = $row.goos; $env:GOARCH = $row.goarch; $env:CGO_ENABLED = '0'
try {
  & go build -trimpath "-ldflags=-s -w -X main.Version=$Version" -o $binpath .
  if ($LASTEXITCODE -ne 0) { Die 'go build failed' }
} finally {
  Remove-Item Env:\GOOS, Env:\GOARCH, Env:\CGO_ENABLED -ErrorAction SilentlyContinue
}
Ok "binary -> builds\$Platform\$binname"

# ── 3. xray sidecar: cached under builds\.cache\, copied next to the binary ───
$cacheDir   = Join-Path $Cache $Platform
$cachedXray = Join-Path $cacheDir $row.xray_in
if (-not (Test-Path $cachedXray)) {
  New-Item -ItemType Directory -Force -Path $cacheDir | Out-Null
  $url = "https://github.com/XTLS/Xray-core/releases/download/$XrayVersion/$($row.xray_zip)"
  $tmp = Join-Path ([System.IO.Path]::GetTempPath()) ([System.IO.Path]::GetRandomFileName())
  New-Item -ItemType Directory -Force -Path $tmp | Out-Null
  Log "Fetching xray-core $XrayVersion ($($row.xray_zip))"
  try {
    Invoke-WebRequest $url -OutFile (Join-Path $tmp 'xray.zip') -UseBasicParsing
    Add-Type -AssemblyName System.IO.Compression.FileSystem
    $zip = [System.IO.Compression.ZipFile]::OpenRead((Join-Path $tmp 'xray.zip'))
    try {
      $entry = $zip.Entries | Where-Object { $_.Name -eq $row.xray_in } | Select-Object -First 1
      if (-not $entry) { Die "xray binary '$($row.xray_in)' not found in archive" }
      [System.IO.Compression.ZipFileExtensions]::ExtractToFile($entry, $cachedXray, $true)
    } finally { $zip.Dispose() }
  } finally { Remove-Item -Recurse -Force $tmp -ErrorAction SilentlyContinue }
  Ok "xray cached -> builds\.cache\xray\$Platform\$($row.xray_in)"
} else {
  Ok 'xray from cache (delete builds\.cache to re-download)'
}
Copy-Item $cachedXray (Join-Path $outdir $row.xray_in) -Force
Ok "xray -> builds\$Platform\$($row.xray_in)"

Log "Ready: builds\$Platform"
Get-ChildItem $outdir | ForEach-Object { Write-Host "    $($_.Name)" }

# ── 4. Optional run ──────────────────────────────────────────────────────────
if ($Run) {
  if ($row.goos -ne 'windows') {
    Warn "skip -Run: a $Platform binary cannot launch on this Windows host"
    return
  }
  Log "Launching $binname  (Ctrl+C to stop)"
  Push-Location $outdir
  try { & $binpath } finally { Pop-Location }
}
