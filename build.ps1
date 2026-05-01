<#
.SYNOPSIS
    Skreen Build Script
    Compiles the Agent, packages it into a Windows installer via NSIS,
    builds the Server, and stages everything for deployment.

.PARAMETER ServerHost
    The production server hostname (e.g. skreen-api.onrender.com).
    Defaults to localhost.

.PARAMETER ServerPort
    The production server port (e.g. 443).
    Defaults to 8080.
#>

param(
    [string]$ServerHost = "localhost",
    [string]$ServerPort = "8080"
)

$ErrorActionPreference = "Stop"

$RootPath       = $PSScriptRoot
$AgentPath      = Join-Path $RootPath "agent"
$ServerPath     = Join-Path $RootPath "server"
$InstallerPath  = Join-Path $RootPath "installer"
$NSISExe        = "C:\Program Files (x86)\NSIS\makensis.exe"

Write-Host ""
Write-Host "  ███████╗██╗  ██╗██████╗ ███████╗███████╗███╗   ██╗" -ForegroundColor White
Write-Host "  ██╔════╝██║ ██╔╝██╔══██╗██╔════╝██╔════╝████╗  ██║" -ForegroundColor White
Write-Host "  ███████╗█████╔╝ ██████╔╝█████╗  █████╗  ██╔██╗ ██║" -ForegroundColor White
Write-Host "  ╚════██║██╔═██╗ ██╔══██╗██╔══╝  ██╔══╝  ██║╚██╗██║" -ForegroundColor White
Write-Host "  ███████║██║  ██╗██║  ██╗███████╗███████╗██║ ╚████║" -ForegroundColor White
Write-Host "  ╚══════╝╚═╝  ╚═╝╚═╝  ╚═╝╚══════╝╚══════╝╚═╝  ╚═══╝" -ForegroundColor White
Write-Host "  Build Pipeline (Target: $ServerHost`:$ServerPort)" -ForegroundColor DarkGray
Write-Host ""

# ── 1. Build the Agent ────────────────────────────────────────────────────────
Write-Host "[1/4] Building Agent..." -ForegroundColor Cyan
Set-Location -Path $AgentPath

try {
    # -H=windowsgui : No console window
    # -s -w         : Strip debug symbols (smaller binary)
    # -X            : Inject production variables
    $LDFlags = "-H=windowsgui -s -w -X main.ServerHost=$ServerHost -X main.ServerPort=$ServerPort"
    go build -ldflags "$LDFlags" -o skreen-agent.exe ./cmd
    Write-Host "      Agent compiled: agent\skreen-agent.exe (Points to $ServerHost`:$ServerPort)" -ForegroundColor Green
} catch {
    Write-Host "      FAILED: $_" -ForegroundColor Red
    exit 1
}

# ── 2. Build NSIS Installer ───────────────────────────────────────────────────
Write-Host ""
Write-Host "[2/4] Building NSIS installer..." -ForegroundColor Cyan

if (-not (Test-Path $NSISExe)) {
    Write-Host "      NSIS not found at: $NSISExe" -ForegroundColor Red
    exit 1
}

Set-Location -Path $InstallerPath
try {
    & $NSISExe "skreen-agent.nsi"
    if ($LASTEXITCODE -ne 0) { throw "NSIS exited with code $LASTEXITCODE" }
    Write-Host "      Installer built: installer\skreen-agent-setup.exe" -ForegroundColor Green
} catch {
    Write-Host "      FAILED: $_" -ForegroundColor Red
    exit 1
}

# ── 3. Stage installer for Server ─────────────────────────────────────────────
Write-Host ""
Write-Host "[3/4] Staging installer to Server..." -ForegroundColor Cyan

$SourceInstaller = Join-Path $InstallerPath "skreen-agent-setup.exe"
$DestInstaller   = Join-Path $ServerPath    "skreen-agent-setup.exe"

try {
    Copy-Item -Path $SourceInstaller -Destination $DestInstaller -Force
    Write-Host "      Staged: server\skreen-agent-setup.exe" -ForegroundColor Green
} catch {
    Write-Host "      FAILED: $_" -ForegroundColor Red
    exit 1
}

# ── 4. Build the Server ───────────────────────────────────────────────────────
Write-Host ""
Write-Host "[4/4] Building Server..." -ForegroundColor Cyan
Set-Location -Path $ServerPath

try {
    go build -ldflags "-s -w" -o server.exe ./cmd
    Write-Host "      Server compiled: server\server.exe" -ForegroundColor Green
} catch {
    Write-Host "      FAILED: $_" -ForegroundColor Red
    exit 1
}

# ── Done ─────────────────────────────────────────────────────────────────────
Set-Location -Path $RootPath
Write-Host ""
Write-Host "  Build complete." -ForegroundColor White
Write-Host ""
Write-Host "  To deploy to Production:" -ForegroundColor DarkGray
Write-Host "    1. Run: .\build.ps1 -ServerHost your-api.onrender.com -ServerPort 443" -ForegroundColor White
Write-Host "    2. Push the code (including server/skreen-agent-setup.exe) to GitHub." -ForegroundColor White
Write-Host ""
