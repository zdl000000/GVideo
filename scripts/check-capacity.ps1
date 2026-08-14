param(
    [ValidateRange(1, 99)]
    [int]$WarningFreePercent = 20,
    [ValidateRange(1, 99)]
    [int]$CriticalFreePercent = 10,
    [switch]$SkipHostDrive
)

$ErrorActionPreference = "Stop"
$root = Split-Path -Parent $PSScriptRoot

if ($CriticalFreePercent -ge $WarningFreePercent) {
    throw "CriticalFreePercent must be lower than WarningFreePercent."
}

function Test-FreePercent([string]$Name, [double]$FreePercent) {
    $rounded = [math]::Round($FreePercent, 1)
    Write-Host "$Name free space: $rounded%"
    if ($FreePercent -lt $CriticalFreePercent) {
        throw "$Name free space is below the critical threshold of $CriticalFreePercent%: $rounded%."
    }
    if ($FreePercent -lt $WarningFreePercent) {
        Write-Warning "$Name free space is below the warning threshold of $WarningFreePercent%: $rounded%."
    }
}

if (-not $SkipHostDrive) {
    $rootDriveName = ([System.IO.Path]::GetPathRoot($root)).TrimEnd("\", "/", ":")
    $rootDrive = Get-PSDrive -Name $rootDriveName -ErrorAction Stop
    $hostTotal = [double]$rootDrive.Used + [double]$rootDrive.Free
    if ($hostTotal -le 0) {
        throw "Could not determine free space for host drive $rootDriveName."
    }
    Test-FreePercent "Host drive $rootDriveName" ([double]$rootDrive.Free / $hostTotal * 100)
}

Push-Location $root
try {
    $compose = Join-Path $root "compose.yaml"
    $backendContainer = (@(docker compose -f $compose ps -q backend) -join "`n").Trim()
    if ($LASTEXITCODE -ne 0) {
        throw "Finding the backend container failed."
    }
    if (-not $backendContainer) {
        throw "The backend container is not running."
    }

    $dfOutput = @(docker compose -f $compose exec -T backend df -Pk /app/data /app/media 2>&1)
    if ($LASTEXITCODE -ne 0) {
        throw "Reading Docker volume capacity failed:`n$($dfOutput -join [Environment]::NewLine)"
    }

    $foundMounts = 0
    foreach ($line in $dfOutput) {
        if ($line -match '^\S+\s+\d+\s+\d+\s+(\d+)\s+(\d+)%\s+(/app/(?:data|media))\s*$') {
            $usedPercent = [double]$Matches[2]
            if ($usedPercent -eq 100) {
                $freePercent = 0
            } else {
                $freePercent = 100 - $usedPercent
            }
            Test-FreePercent "Docker mount $($Matches[3])" $freePercent
            $foundMounts++
        }
    }
    if ($foundMounts -eq 0) {
        throw "Docker volume capacity output did not contain /app/data or /app/media."
    }
} finally {
    Pop-Location
}
