[CmdletBinding()]
param(
    [switch]$SkipTests
)

$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest

$ScriptDirectory = Split-Path -Parent $MyInvocation.MyCommand.Path
$RepositoryRoot = (Resolve-Path (Join-Path $ScriptDirectory "..")).Path
$script:FrontendBuilt = $false

function Write-Step([string]$Message) {
    Write-Host "`n==> $Message" -ForegroundColor Cyan
}

function Assert-Command([string]$Name, [string]$HelpMessage) {
    if (-not (Get-Command $Name -ErrorAction SilentlyContinue)) {
        throw "$Name was not found. $HelpMessage"
    }
}

function Invoke-Checked([scriptblock]$Command) {
    & $Command
    if ($LASTEXITCODE -ne 0) {
        throw "Command failed with exit code $LASTEXITCODE."
    }
}

function Build-Frontend {
    if ($script:FrontendBuilt) {
        return
    }
    Write-Step "Installing and building the frontend"
    Push-Location (Join-Path $RepositoryRoot "frontend")
    try {
        Invoke-Checked { npm ci }
        Invoke-Checked { npm run build }
    }
    finally {
        Pop-Location
    }
    $script:FrontendBuilt = $true
}

function Build-ProductionExecutable([string]$OutputPath) {
    Build-Frontend
    Push-Location $RepositoryRoot
    try {
        Write-Step "Generating theme-aware application icons"
        Invoke-Checked { go run ./tools/theme-icon }
        New-Item -ItemType Directory -Force -Path (Split-Path -Parent $OutputPath) | Out-Null
        Invoke-Checked { go build "-tags=production" -trimpath "-ldflags=-w -s" -o $OutputPath . }
    }
    finally {
        Pop-Location
    }
}

if ($env:OS -ne "Windows_NT") {
    throw "This script must be run on Windows."
}

Assert-Command "go" "Install Go 1.25 or newer from https://go.dev/dl/."
Assert-Command "node" "Install Node.js 20 or newer from https://nodejs.org/."
Assert-Command "npm" "npm is included with Node.js."

$GoVersionText = ((& go env GOVERSION).Trim() -replace '^go', '')
$NodeVersionText = ((& node --version).Trim() -replace '^v', '')
$NpmVersionText = (& npm --version).Trim()
$RequiredGoText = ((Select-String -Path (Join-Path $RepositoryRoot "go.mod") -Pattern '^go\s+(.+)$').Matches[0].Groups[1].Value).Trim()

if ([version]$GoVersionText -lt [version]$RequiredGoText) {
    throw "Go $RequiredGoText or newer is required; found $GoVersionText."
}
if ([version]$NodeVersionText -lt [version]"20.0.0") {
    throw "Node.js 20 or newer is required; found $NodeVersionText."
}

Write-Step "Toolchain: Go $GoVersionText, Node $NodeVersionText, npm $NpmVersionText"

if (-not $SkipTests) {
    Write-Step "Running backend tests and frontend checks"
    Build-Frontend
    Push-Location (Join-Path $RepositoryRoot "frontend")
    try {
        Invoke-Checked { npm run check }
    }
    finally {
        Pop-Location
    }
    Push-Location $RepositoryRoot
    try {
        Invoke-Checked { go test ./... }
    }
    finally {
        Pop-Location
    }
}
else {
    Write-Step "Skipping verification"
}

Write-Step "Building the Windows executable"
$OutputPath = Join-Path $RepositoryRoot "bin\vibedock.exe"
Build-ProductionExecutable $OutputPath
if (-not (Test-Path -PathType Leaf $OutputPath)) {
    throw "The build completed without producing $OutputPath."
}

Write-Step "Built $OutputPath ($env:PROCESSOR_ARCHITECTURE)"
Write-Host "WebView2 is included with current Windows 10/11 installations and is required to run VibeDock."
