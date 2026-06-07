#Requires -Version 5.1
<#
.SYNOPSIS
  Cloudflare Scanner — local build script for Windows (PowerShell).

.DESCRIPTION
  Builds the app from source and bundles the matching xray-core sidecar,
  producing release-identical archives under .\dist\.

.EXAMPLE
  .\scripts\build.ps1                      # build for the current host platform
  .\scripts\build.ps1 all                  # build every supported platform
  .\scripts\build.ps1 windows-amd64        # build one specific platform
  .\scripts\build.ps1 windows-amd64 linux-amd64

.NOTES
  Supported platform keys:
    windows-amd64  windows-arm64  linux-amd64  linux-arm64
    termux-arm64   darwin-amd64   darwin-arm64

  Environment overrides:
    $env:VERSION       version string baked into the binary (default: git describe or "dev")
    $env:XRAY_VERSION  xray-core release tag to bundle (default: v1.8.24)
    $env:NO_XRAY=1     skip downloading xray (build the binary only)
    $env:NO_ARCHIVE=1  leave loose files in dist\<platform>\, skip .zip/.tar.gz
    $env:GO_VERSION    Go version to auto-install if Go is missing (default: 1.26.2)
#>

param([Parameter(ValueFromRemainingArguments = $true)] [string[]] $Targets)

$ErrorActionPreference = 'Stop'

# ── Config ──────────────────────────────────────────────────────────────────
$XrayVersion = if ($env:XRAY_VERSION) { $env:XRAY_VERSION } else { 'v1.8.24' }
$GoVersion   = if ($env:GO_VERSION)   { $env:GO_VERSION }   else { '1.26.2' }
$App         = 'Cloudflare-Scanner'

$RepoRoot = Split-Path -Parent $PSScriptRoot
$Dist     = Join-Path $RepoRoot 'dist'
Set-Location $RepoRoot

# ── Logging ─────────────────────────────────────────────────────────────────
function Log  ($m) { Write-Host "==> $m" -ForegroundColor Cyan }
function Ok   ($m) { Write-Host "  ok $m" -ForegroundColor Green }
function Warn ($m) { Write-Host "  !  $m" -ForegroundColor Yellow }
function Die  ($m) { Write-Error $m; exit 1 }

# ── Platform matrix (mirrors .github/workflows/release.yml) ──────────────────
$Matrix = @(
  @{ key='windows-amd64'; goos='windows'; goarch='amd64'; ext='.exe'; xray_in='xray.exe'; xray_zip='Xray-windows-64.zip' }
  @{ key='windows-arm64'; goos='windows'; goarch='arm64'; ext='.exe'; xray_in='xray.exe'; xray_zip='Xray-windows-arm64-v8a.zip' }
  @{ key='linux-amd64';   goos='linux';   goarch='amd64'; ext='';     xray_in='xray';     xray_zip='Xray-linux-64.zip' }
  @{ key='linux-arm64';   goos='linux';   goarch='arm64'; ext='';     xray_in='xray';     xray_zip='Xray-linux-arm64-v8a.zip' }
  @{ key='termux-arm64';  goos='linux';   goarch='arm64'; ext='';     xray_in='xray';     xray_zip='Xray-android-arm64-v8a.zip' }
  @{ key='darwin-amd64';  goos='darwin';  goarch='amd64'; ext='';     xray_in='xray';     xray_zip='Xray-macos-64.zip' }
  @{ key='darwin-arm64';  goos='darwin';  goarch='arm64'; ext='';     xray_in='xray';     xray_zip='Xray-macos-arm64-v8a.zip' }
)
function Row-For ($key) { $Matrix | Where-Object { $_.key -eq $key } | Select-Object -First 1 }

# ── Detect host platform key ────────────────────────────────────────────────
function Detect-Host {
  $a = (Get-CimInstance Win32_Processor -Property Architecture).Architecture
  if ($a -eq 12) { 'windows-arm64' } else { 'windows-amd64' }
}

# ── Ensure a Go toolchain is available ──────────────────────────────────────
$script:Go = $null
$NeedGo = (Select-String -Path 'go.mod' -Pattern '^go (\d+\.\d+)').Matches[0].Groups[1].Value

function Version-GE ($a, $b) {
  try { return [version]$a -ge [version]$b } catch { return $true }
}

function Ensure-Go {
  $go = Get-Command go -ErrorAction SilentlyContinue
  if ($go) {
    $have = (& go version) -replace '.*go(\d+\.\d+(\.\d+)?).*', '$1'
    if (Version-GE $have $NeedGo) { $script:Go = $go.Source; Ok "Go $have (>= $NeedGo required)"; return }
    Warn "Go $have is older than required $NeedGo — installing a local copy"
  } else {
    Warn "Go not found — installing a local copy (Go $GoVersion)"
  }

  $arch = if ((Get-CimInstance Win32_Processor -Property Architecture).Architecture -eq 12) { 'arm64' } else { 'amd64' }
  $cache = Join-Path $RepoRoot '.gobuild'
  New-Item -ItemType Directory -Force -Path $cache | Out-Null
  $zip = "go$GoVersion.windows-$arch.zip"
  $url = "https://go.dev/dl/$zip"
  Log "Downloading $url"
  Invoke-WebRequest $url -OutFile (Join-Path $cache $zip) -UseBasicParsing
  if (Test-Path (Join-Path $cache 'go')) { Remove-Item -Recurse -Force (Join-Path $cache 'go') }
  Expand-Archive -LiteralPath (Join-Path $cache $zip) -DestinationPath $cache -Force
  $script:Go = Join-Path $cache 'go\bin\go.exe'
  if (-not (Test-Path $script:Go)) { Die 'Go install failed' }
  Ok "Go $GoVersion installed to $cache\go"
}

# ── Download + extract xray-core for one platform ───────────────────────────
function Fetch-Xray ($outdir, $xray_in, $xray_zip) {
  if ($env:NO_XRAY) { Warn "NO_XRAY set — skipping xray for $outdir"; return }
  $url = "https://github.com/XTLS/Xray-core/releases/download/$XrayVersion/$xray_zip"
  $tmp = Join-Path ([System.IO.Path]::GetTempPath()) ([System.IO.Path]::GetRandomFileName())
  New-Item -ItemType Directory -Force -Path $tmp | Out-Null
  Log "Fetching xray-core $XrayVersion ($xray_zip)"
  Invoke-WebRequest $url -OutFile (Join-Path $tmp 'xray.zip') -UseBasicParsing
  Add-Type -AssemblyName System.IO.Compression.FileSystem
  $zipObj = [System.IO.Compression.ZipFile]::OpenRead((Join-Path $tmp 'xray.zip'))
  try {
    $entry = $zipObj.Entries | Where-Object { $_.Name -eq $xray_in } | Select-Object -First 1
    if (-not $entry) { Die "xray binary '$xray_in' not found in archive" }
    [System.IO.Compression.ZipFileExtensions]::ExtractToFile($entry, (Join-Path $outdir $xray_in), $true)
  } finally { $zipObj.Dispose() }
  Remove-Item -Recurse -Force $tmp
  Ok "xray-core -> $outdir\$xray_in"
}

# ── Build one platform ──────────────────────────────────────────────────────
function Build-One ($key) {
  $r = Row-For $key
  if (-not $r) { Die "unknown platform: $key (run with no args for host, or 'all')" }

  $outdir = Join-Path $Dist $key
  if (Test-Path $outdir) { Remove-Item -Recurse -Force $outdir }
  New-Item -ItemType Directory -Force -Path $outdir | Out-Null
  $binname = "$App$($r.ext)"

  Log "Building $key  (GOOS=$($r.goos) GOARCH=$($r.goarch), version=$Version)"
  $env:GOOS = $r.goos; $env:GOARCH = $r.goarch; $env:CGO_ENABLED = '0'
  & $script:Go build -trimpath -ldflags="-s -w -X 'main.Version=$Version'" -o (Join-Path $outdir $binname) .
  if ($LASTEXITCODE -ne 0) { Die "go build failed for $key" }
  Remove-Item Env:\GOOS, Env:\GOARCH, Env:\CGO_ENABLED -ErrorAction SilentlyContinue
  Ok "binary -> $outdir\$binname"

  Fetch-Xray $outdir $r.xray_in $r.xray_zip

  if (-not $env:NO_ARCHIVE) {
    if ($r.goos -eq 'windows') {
      $archive = Join-Path $Dist "$App-$Version-$key.zip"
      $items = @(Join-Path $outdir $binname)
      if (-not $env:NO_XRAY) { $items += (Join-Path $outdir $r.xray_in) }
      if (Test-Path $archive) { Remove-Item -Force $archive }
      Compress-Archive -Path $items -DestinationPath $archive -Force
      Ok "archive -> dist\$App-$Version-$key.zip"
    } else {
      # tar ships with Windows 10+; produces a release-identical .tar.gz
      $archive = Join-Path $Dist "$App-$Version-$key.tar.gz"
      $items = @($binname)
      if (-not $env:NO_XRAY) { $items += $r.xray_in }
      if (Get-Command tar -ErrorAction SilentlyContinue) {
        tar -czf $archive -C $outdir @items
        Ok "archive -> dist\$App-$Version-$key.tar.gz"
      } else {
        Warn "tar not available — leaving loose files in $outdir"
      }
    }
  }
}

# ── Resolve version ─────────────────────────────────────────────────────────
$Version = $env:VERSION
if (-not $Version) {
  if (Get-Command git -ErrorAction SilentlyContinue) {
    $Version = (& git describe --tags --always --dirty 2>$null)
    if (-not $Version) { $Version = 'dev' }
  } else { $Version = 'dev' }
}

# ── Main ────────────────────────────────────────────────────────────────────
Log "Cloudflare Scanner build  -  version $Version, xray $XrayVersion"
Ensure-Go
& $script:Go vet ./...
if ($LASTEXITCODE -ne 0) { Die 'go vet failed' }
Ok 'go vet clean'

if (-not $Targets -or $Targets.Count -eq 0) {
  $list = @(Detect-Host)
  Log "No target given — building host platform: $($list[0])"
} elseif ($Targets[0] -eq 'all') {
  $list = $Matrix.key
} else {
  $list = $Targets
}

New-Item -ItemType Directory -Force -Path $Dist | Out-Null
foreach ($key in $list) { Build-One $key }

Log "Done. Artifacts in: $Dist"
Get-ChildItem $Dist | ForEach-Object { Write-Host "  $($_.Name)" }
