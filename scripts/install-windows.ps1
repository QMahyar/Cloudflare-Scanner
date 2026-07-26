#Requires -Version 5.1
# Cloudflare Scanner — one-liner installer for Windows
# Run in PowerShell:
#   [Net.ServicePointManager]::SecurityProtocol=[Net.SecurityProtocolType]::Tls12; irm https://raw.githubusercontent.com/QMahyar/Cloudflare-Scanner/master/scripts/install-windows.ps1 | iex

$ErrorActionPreference = 'Stop'
if ($PSVersionTable.PSEdition -eq 'Core' -and -not $IsWindows) {
    Write-Error 'This installer is for Windows PowerShell/pwsh only.'
    exit 1
}

$Repo = 'QMahyar/Cloudflare-Scanner'
$InstallDir = Join-Path $env:LOCALAPPDATA 'CloudflareScanner'
$App = Join-Path $InstallDir 'scanner-app.exe'

$arch = (Get-CimInstance -ClassName Win32_Processor -Property Architecture).Architecture
$Platform = if ($arch -eq 12) { 'windows-arm64' } else { 'windows-amd64' }

Write-Host 'Fetching latest release...'
try {
    $release = Invoke-RestMethod "https://api.github.com/repos/$Repo/releases/latest"
    $Tag = $release.tag_name
} catch {
    Write-Error "Could not fetch release info: $_"
    exit 1
}

Write-Host "Installing Cloudflare Scanner $Tag ($Platform)..."
$Zip = Join-Path $env:TEMP "Cloudflare-Scanner-${Tag}-${Platform}.zip"
$Url = "https://github.com/$Repo/releases/download/$Tag/Cloudflare-Scanner-${Tag}-${Platform}.zip"
Invoke-WebRequest $Url -OutFile $Zip -UseBasicParsing
New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null
Expand-Archive -LiteralPath $Zip -DestinationPath $InstallDir -Force
Remove-Item $Zip -Force

# Avoid a command/wrapper name collision: scanner-app.exe is the payload; all
# public commands are tiny .cmd launchers in the same PATH directory.
Copy-Item (Join-Path $InstallDir 'Cloudflare-Scanner.exe') $App -Force
Remove-Item (Join-Path $InstallDir 'Cloudflare-Scanner.exe') -Force
Invoke-WebRequest "https://raw.githubusercontent.com/$Repo/master/scripts/scan-command.ps1" -OutFile (Join-Path $InstallDir 'scan-command.ps1') -UseBasicParsing

foreach ($name in 'Cloudflare-Scanner', 'cloudflare-scanner', 'scan') {
    @"
@echo off
powershell.exe -NoProfile -ExecutionPolicy Bypass -File "$InstallDir\scan-command.ps1" %*
"@ | Set-Content -LiteralPath (Join-Path $InstallDir "$name.cmd") -Encoding ASCII
}

$UserPath = [Environment]::GetEnvironmentVariable('PATH', 'User') -split ';' | Where-Object { $_ }
if ($InstallDir -notin $UserPath) {
    [Environment]::SetEnvironmentVariable('PATH', (($UserPath + $InstallDir) -join ';'), 'User')
    Write-Host "Added $InstallDir to your PATH."
}

Write-Host ''
Write-Host 'Done! Open a new terminal, then run: Cloudflare-Scanner'
Write-Host 'Aliases: cloudflare-scanner, scan'
Write-Host 'Commands: Cloudflare-Scanner help | update | restart | uninstall'
