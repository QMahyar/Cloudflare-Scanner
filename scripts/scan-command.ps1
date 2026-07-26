# Cloudflare Scanner command manager. Installed as Cloudflare-Scanner, cloudflare-scanner, and scan.
param([Parameter(Position = 0)][string]$Command = 'start')

$ErrorActionPreference = 'Stop'
$Repo = 'QMahyar/Cloudflare-Scanner'
$InstallDir = Join-Path $env:LOCALAPPDATA 'CloudflareScanner'
$App = Join-Path $InstallDir 'scanner-app.exe'

function Stop-Scanner {
    Get-CimInstance Win32_Process -Filter "Name = 'scanner-app.exe'" -ErrorAction SilentlyContinue |
        Where-Object { $_.ExecutablePath -eq $App } |
        ForEach-Object { Stop-Process -Id $_.ProcessId -Force -ErrorAction SilentlyContinue }
}

function Show-Help {
@'
Cloudflare Scanner

Usage: Cloudflare-Scanner [command]
       scan [command]

Commands:
  start       Launch the app (default)
  restart     Stop the installed app and launch it again
  update      Install the latest GitHub release
  uninstall   Remove the app and command aliases
  help        Show this help
'@ | Write-Host
}

switch ($Command.ToLowerInvariant()) {
    { $_ -in '', 'start', 'launch', 'run' } {
        if (!(Test-Path -LiteralPath $App)) { throw "Cloudflare Scanner is not installed. Run the installer first." }
        Start-Process -FilePath $App
    }
    'restart' {
        Stop-Scanner
        if (!(Test-Path -LiteralPath $App)) { throw "Cloudflare Scanner is not installed. Run the installer first." }
        Start-Process -FilePath $App
    }
    'update' {
        Stop-Scanner
        [Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12
        Invoke-Expression (Invoke-RestMethod "https://raw.githubusercontent.com/$Repo/master/scripts/install-windows.ps1")
    }
    { $_ -in 'uninstall', 'remove' } {
        Stop-Scanner
        $userPath = [Environment]::GetEnvironmentVariable('PATH', 'User') -split ';' | Where-Object { $_ -and $_ -ne $InstallDir }
        [Environment]::SetEnvironmentVariable('PATH', ($userPath -join ';'), 'User')
        Remove-Item -LiteralPath $InstallDir -Recurse -Force -ErrorAction SilentlyContinue
        Write-Host 'Cloudflare Scanner uninstalled. Open a new terminal to refresh PATH.'
    }
    { $_ -in 'help', '-h', '--help' } { Show-Help }
    default { Write-Error "Unknown command: $Command"; Show-Help; exit 2 }
}
