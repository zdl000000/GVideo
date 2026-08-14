[CmdletBinding()]
param(
    [string]$BaseURL = "http://127.0.0.1:8088",
    [string]$BackendHealthURL = "http://127.0.0.1:8080/healthz",
    [switch]$SkipBuildChecks,
    [switch]$SkipCapacityCheck,
    [switch]$IncludeBackupRestore,
    [switch]$KeepDrillBackup
)

Set-StrictMode -Version 2.0
$ErrorActionPreference = "Stop"
$ProgressPreference = "SilentlyContinue"
$root = [System.IO.Path]::GetFullPath((Split-Path -Parent $PSScriptRoot))
$frontend = Join-Path $root "frontend"
$powershellCommand = Get-Command powershell.exe -ErrorAction SilentlyContinue
if (-not $powershellCommand) {
    throw "Windows PowerShell 5.1 was not found on PATH."
}
$powershell = $powershellCommand.Source
$phase = 0

function Write-Phase([string]$Message) {
    $script:phase++
    Write-Host ("`n=== [{0:00}] {1} ===" -f $script:phase, $Message) -ForegroundColor Cyan
}

function Assert-LastExitCode([string]$Message) {
    if ($LASTEXITCODE -ne 0) {
        throw $Message
    }
}

function Assert-Script([string]$Path) {
    if (-not (Test-Path -LiteralPath $Path -PathType Leaf)) {
        throw "Required acceptance script was not found: $Path"
    }
}

function Invoke-PowerShellScript {
    param(
        [Parameter(Mandatory = $true)][string]$Path,
        [string[]]$Arguments = @(),
        [Parameter(Mandatory = $true)][string]$FailureMessage
    )
    & $powershell -NoProfile -ExecutionPolicy Bypass -File $Path @Arguments
    Assert-LastExitCode $FailureMessage
}

function Test-HealthEndpoint([string]$URL) {
    try {
        $response = Invoke-WebRequest -UseBasicParsing -Uri $URL -TimeoutSec 15
    } catch {
        throw "Health check failed for ${URL}: $($_.Exception.Message)"
    }
    if ($response.StatusCode -ne 200) {
        throw "Health check for $URL returned HTTP $($response.StatusCode)."
    }
    try {
        $payload = $response.Content | ConvertFrom-Json
    } catch {
        throw "Health check for $URL did not return JSON."
    }
    if ($payload.data.status -ne "ok") {
        throw "Health check for $URL did not report status ok."
    }
    Write-Host "Healthy: $URL"
}

$baseUri = $null
if (-not [Uri]::TryCreate($BaseURL, [UriKind]::Absolute, [ref]$baseUri) -or $baseUri.Scheme -notin @("http", "https")) {
    throw "BaseURL must be an absolute HTTP or HTTPS URL."
}
if ($baseUri.Query -or $baseUri.Fragment) {
    throw "BaseURL must not contain a query string or fragment."
}
if ($KeepDrillBackup -and -not $IncludeBackupRestore) {
    throw "KeepDrillBackup requires IncludeBackupRestore."
}

$normalizedBaseURL = $BaseURL.TrimEnd('/')
$checkScript = Join-Path $PSScriptRoot "check.ps1"
$capacityScript = Join-Path $PSScriptRoot "check-capacity.ps1"
$apiScript = Join-Path $PSScriptRoot "acceptance-api.ps1"
$backupScript = Join-Path $PSScriptRoot "backup-restore-drill.ps1"
foreach ($scriptPath in @($checkScript, $capacityScript, $apiScript, $backupScript)) {
    Assert-Script $scriptPath
}
if (-not (Test-Path -LiteralPath (Join-Path $frontend "package.json") -PathType Leaf)) {
    throw "Frontend package.json was not found: $frontend"
}

$startedAt = Get-Date
try {
    if (-not $SkipBuildChecks) {
        Write-Phase "Static checks, tests, typecheck, and production build"
        Invoke-PowerShellScript -Path $checkScript -FailureMessage "Project checks failed."
    }

    Write-Phase "Running service health checks"
    Test-HealthEndpoint $BackendHealthURL
    Test-HealthEndpoint ($normalizedBaseURL + "/healthz")

    if (-not $SkipCapacityCheck) {
        Write-Phase "Checking host and persistent-volume capacity"
        Invoke-PowerShellScript -Path $capacityScript -FailureMessage "Capacity check failed."
    }

    Write-Phase "Running the core API user journey"
    Invoke-PowerShellScript -Path $apiScript -Arguments @("-BaseURL", $normalizedBaseURL) -FailureMessage "API acceptance failed."

    Write-Phase "Running desktop and mobile browser acceptance"
    $previousE2EBaseURL = $env:E2E_BASE_URL
    try {
        $env:E2E_BASE_URL = $normalizedBaseURL
        Push-Location $frontend
        try {
            if ($baseUri.IsLoopback) {
                & npm run test:e2e
            } else {
                Write-Host "Remote browser target detected; using one worker and a 90-second test timeout."
                & npm run test:e2e -- --workers=1 --timeout=90000
            }
            Assert-LastExitCode "Browser acceptance failed."
        } finally {
            Pop-Location
        }
    } finally {
        $env:E2E_BASE_URL = $previousE2EBaseURL
    }

    if ($IncludeBackupRestore) {
        Write-Phase "Creating a backup and verifying an isolated restore"
        if ($KeepDrillBackup) {
            Invoke-PowerShellScript -Path $backupScript -FailureMessage "Backup and restore drill failed."
        } else {
            Invoke-PowerShellScript -Path $backupScript -Arguments @("-RemoveBackupOnSuccess") -FailureMessage "Backup and restore drill failed."
        }
    }

    $elapsed = (Get-Date) - $startedAt
    Write-Host ("`nAcceptance passed in {0:mm\:ss}." -f $elapsed) -ForegroundColor Green
} catch {
    $elapsed = (Get-Date) - $startedAt
    [Console]::Error.WriteLine("Acceptance failed after {0:mm\:ss}: {1}", $elapsed, $_.Exception.Message)
    exit 1
}

exit 0
