[CmdletBinding()]
param(
    [string]$CloudflaredPath = "",
    [ValidateRange(15, 180)]
    [int]$StartupTimeoutSeconds = 90
)

Set-StrictMode -Version 2.0
$ErrorActionPreference = "Stop"
$ProgressPreference = "SilentlyContinue"
$root = [IO.Path]::GetFullPath((Split-Path -Parent $PSScriptRoot))
$tmp = Join-Path $root "tmp"
$statePath = Join-Path $tmp "public-test.json"
$stdoutPath = Join-Path $tmp "cloudflared.stdout.log"
$stderrPath = Join-Path $tmp "cloudflared.stderr.log"
$localURL = "http://127.0.0.1:8088"
$startedTunnel = $false
$tunnelProcess = $null
$publicSettingsRequested = $false

function Invoke-Checked([string[]]$Arguments, [string]$FailureMessage) {
    & $Arguments[0] $Arguments[1..($Arguments.Count - 1)]
    if ($LASTEXITCODE -ne 0) { throw $FailureMessage }
}

function Wait-ForURL([string]$URL, [int]$TimeoutSeconds) {
    $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
    do {
        try {
            $response = Invoke-WebRequest -UseBasicParsing -Uri $URL -TimeoutSec 15
            if ($response.StatusCode -eq 200) { return }
        } catch {
            Start-Sleep -Seconds 2
        }
    } while ((Get-Date) -lt $deadline)
    throw "Timed out waiting for $URL."
}

function Restore-LocalComposeSettings {
    $env:FRONTEND_URL = $localURL
    $env:COOKIE_SECURE = "false"
    Invoke-Checked @("docker", "compose", "up", "-d", "--force-recreate", "backend", "frontend") "Failed to restore local HTTP settings."
    Wait-ForURL ($localURL + "/healthz") $StartupTimeoutSeconds
}

function Resolve-Cloudflared([string]$RequestedPath) {
    if (-not [string]::IsNullOrWhiteSpace($RequestedPath)) {
        $resolved = [IO.Path]::GetFullPath($RequestedPath)
        if (-not (Test-Path -LiteralPath $resolved -PathType Leaf)) {
            throw "cloudflared was not found at $resolved."
        }
        return $resolved
    }

    $command = Get-Command cloudflared -ErrorAction SilentlyContinue
    if ($command) { return $command.Source }

    $knownPaths = @(
        (Join-Path ${env:ProgramFiles(x86)} "cloudflared\cloudflared.exe"),
        (Join-Path $env:ProgramFiles "cloudflared\cloudflared.exe"),
        (Join-Path $env:LOCALAPPDATA "Microsoft\WinGet\Links\cloudflared.exe")
    )
    foreach ($path in $knownPaths) {
        if ($path -and (Test-Path -LiteralPath $path -PathType Leaf)) { return $path }
    }
    throw "cloudflared was not found. Install Cloudflare.cloudflared with winget first."
}

function Find-PublicURL {
    foreach ($path in @($stderrPath, $stdoutPath)) {
        if (-not (Test-Path -LiteralPath $path -PathType Leaf)) { continue }
        $content = Get-Content -LiteralPath $path -Raw -ErrorAction SilentlyContinue
        if ([string]::IsNullOrWhiteSpace($content)) { continue }
        $match = [regex]::Match($content, 'https://[a-z0-9-]+\.trycloudflare\.com')
        if ($match.Success) { return $match.Value }
    }
    return ""
}

New-Item -ItemType Directory -Path $tmp -Force | Out-Null
$cloudflared = Resolve-Cloudflared $CloudflaredPath

Push-Location $root
try {
    $publicURL = ""
    if (Test-Path -LiteralPath $statePath -PathType Leaf) {
        try {
            $state = Get-Content -LiteralPath $statePath -Raw | ConvertFrom-Json
            $savedProcess = Get-Process -Id ([int]$state.process_id) -ErrorAction SilentlyContinue
            if ($savedProcess -and $savedProcess.ProcessName -eq "cloudflared") {
                $tunnelProcess = $savedProcess
                $publicURL = [string]$state.public_url
            }
        } catch {
            $publicURL = ""
        }
    }

    if (-not $tunnelProcess) {
        Remove-Item -LiteralPath $stdoutPath, $stderrPath -Force -ErrorAction SilentlyContinue
        $tunnelProcess = Start-Process -FilePath $cloudflared `
            -ArgumentList @("tunnel", "--url", $localURL, "--no-autoupdate") `
            -RedirectStandardOutput $stdoutPath `
            -RedirectStandardError $stderrPath `
            -WindowStyle Hidden `
            -PassThru
        $startedTunnel = $true
    }

    if ([string]::IsNullOrWhiteSpace($publicURL)) {
        $deadline = (Get-Date).AddSeconds($StartupTimeoutSeconds)
        do {
            if ($tunnelProcess.HasExited) {
                throw "cloudflared exited before creating a public URL. Check $stderrPath."
            }
            $publicURL = Find-PublicURL
            if (-not [string]::IsNullOrWhiteSpace($publicURL)) { break }
            Start-Sleep -Seconds 2
        } while ((Get-Date) -lt $deadline)
    }
    if ([string]::IsNullOrWhiteSpace($publicURL)) {
        throw "Timed out waiting for a trycloudflare.com URL. Check $stderrPath."
    }

    $env:FRONTEND_URL = $publicURL
    $env:COOKIE_SECURE = "true"
    $publicSettingsRequested = $true
    $currentFrontendURL = docker inspect gvideo-backend-1 --format '{{range .Config.Env}}{{println .}}{{end}}' 2>$null |
        Where-Object { $_ -like "FRONTEND_URL=*" } |
        Select-Object -First 1
    $currentCookieSecure = docker inspect gvideo-backend-1 --format '{{range .Config.Env}}{{println .}}{{end}}' 2>$null |
        Where-Object { $_ -like "COOKIE_SECURE=*" } |
        Select-Object -First 1
    if ($currentFrontendURL -eq "FRONTEND_URL=$publicURL" -and $currentCookieSecure -eq "COOKIE_SECURE=true") {
        Invoke-Checked @("docker", "compose", "up", "-d", "backend", "frontend") "Failed to start GVideo."
    } else {
        Invoke-Checked @("docker", "compose", "up", "-d", "--force-recreate", "backend", "frontend") "Failed to enable HTTPS public-test settings."
    }
    Wait-ForURL ($localURL + "/healthz") $StartupTimeoutSeconds
    Wait-ForURL ($publicURL + "/healthz") $StartupTimeoutSeconds

    [ordered]@{
        process_id = $tunnelProcess.Id
        public_url = $publicURL
        local_url = $localURL
        started_at = (Get-Date).ToString("o")
        cloudflared_path = $cloudflared
    } | ConvertTo-Json | Set-Content -LiteralPath $statePath -Encoding utf8

    Write-Host "`nGVideo public test is ready:" -ForegroundColor Green
    Write-Host $publicURL -ForegroundColor Cyan
    Write-Host "Run .\scripts\acceptance.ps1 -BaseURL $publicURL -SkipBuildChecks -SkipCapacityCheck to repeat acceptance."
    Write-Host "The URL remains available while Docker Desktop and cloudflared process $($tunnelProcess.Id) are running."
} catch {
    $originalError = $_
    $rollbackError = $null
    if ($publicSettingsRequested) {
        try {
            Restore-LocalComposeSettings
            Write-Warning "Public test startup failed; local HTTP settings were restored."
        } catch {
            $rollbackError = $_
        }
    }
    if ($startedTunnel -and $tunnelProcess -and -not $tunnelProcess.HasExited) {
        Stop-Process -Id $tunnelProcess.Id -Force -ErrorAction SilentlyContinue
    }
    if ($rollbackError) {
        throw "Public test startup failed: $($originalError.Exception.Message) Local rollback also failed: $($rollbackError.Exception.Message)"
    }
    throw $originalError
} finally {
    Pop-Location
}
