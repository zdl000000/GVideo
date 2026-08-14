[CmdletBinding()]
param()

Set-StrictMode -Version 2.0
$ErrorActionPreference = "Stop"
$root = [IO.Path]::GetFullPath((Split-Path -Parent $PSScriptRoot))
$statePath = Join-Path $root "tmp\public-test.json"
$localURL = "http://127.0.0.1:8088"

function Invoke-Checked([string[]]$Arguments, [string]$FailureMessage) {
    & $Arguments[0] $Arguments[1..($Arguments.Count - 1)]
    if ($LASTEXITCODE -ne 0) { throw $FailureMessage }
}

$state = $null
if (Test-Path -LiteralPath $statePath -PathType Leaf) {
    $state = Get-Content -LiteralPath $statePath -Raw | ConvertFrom-Json
    $process = Get-Process -Id ([int]$state.process_id) -ErrorAction SilentlyContinue
    if ($process -and $process.ProcessName -eq "cloudflared") {
        Stop-Process -Id $process.Id -Force
        Wait-Process -Id $process.Id -Timeout 15 -ErrorAction SilentlyContinue
        Write-Host "Stopped cloudflared process $($process.Id)."
    }
} else {
    Write-Host "No public-test state file was found; no tunnel process was stopped."
}

Push-Location $root
try {
    $env:FRONTEND_URL = $localURL
    $env:COOKIE_SECURE = "false"
    Invoke-Checked @("docker", "compose", "up", "-d", "--force-recreate", "backend", "frontend") "Failed to restore local HTTP settings."
} finally {
    Pop-Location
}

Remove-Item -LiteralPath $statePath -Force -ErrorAction SilentlyContinue
Write-Host "GVideo local mode restored at $localURL" -ForegroundColor Green
