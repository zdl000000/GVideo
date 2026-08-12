param(
    [string]$Destination = ""
)

$ErrorActionPreference = "Stop"
$root = Split-Path -Parent $PSScriptRoot
$compose = Join-Path $root "compose.yaml"
$stamp = (Get-Date).ToUniversalTime().ToString("yyyyMMdd-HHmmss-fff'Z'")
if (-not $Destination) {
    $Destination = Join-Path $root "backups\gvideo-$stamp"
} elseif (-not [System.IO.Path]::IsPathRooted($Destination)) {
    $Destination = Join-Path $root $Destination
}
$destinationPath = [System.IO.Path]::GetFullPath($Destination).TrimEnd("\", "/")
$workspacePath = [System.IO.Path]::GetFullPath($root)
$workspacePrefix = $workspacePath.TrimEnd("\", "/") + [System.IO.Path]::DirectorySeparatorChar
if ($destinationPath -ne $workspacePath -and -not $destinationPath.StartsWith($workspacePrefix, [System.StringComparison]::OrdinalIgnoreCase)) {
    throw "Backup destination must stay inside the GVideo workspace."
}
if (Test-Path -LiteralPath $destinationPath) {
    throw "Backup destination already exists: $destinationPath"
}

$databasePath = Join-Path $destinationPath "database.db"
$mediaPath = Join-Path $destinationPath "media.tar.gz"
$manifestPath = Join-Path $destinationPath "manifest.json"
$containerPath = "/tmp/gvideo-backup-$stamp.db"

function Assert-LastExitCode([string]$Message) {
    if ($LASTEXITCODE -ne 0) {
        throw $Message
    }
}

function Assert-NoReparsePointInPath([string]$Path, [string]$Purpose) {
    $cursor = [System.IO.Path]::GetFullPath($Path)
    while ($cursor) {
        if (Test-Path -LiteralPath $cursor) {
            $item = Get-Item -LiteralPath $cursor -Force
            if (($item.Attributes -band [System.IO.FileAttributes]::ReparsePoint) -ne 0) {
                throw "$Purpose cannot use a symbolic link, junction, or other reparse point: $cursor"
            }
        }
        $parent = [System.IO.Directory]::GetParent($cursor)
        if (-not $parent) { break }
        $cursor = $parent.FullName
    }
}

function Convert-StatusOutput([string[]]$Lines) {
    $result = [ordered]@{}
    foreach ($line in $Lines) {
        if ($line -match "^([^=]+)=(.*)$" -and $Matches[1] -notin @("database", "integrity", "media")) {
            $result[$Matches[1]] = [int64]$Matches[2]
        }
    }
    return $result
}

Push-Location $root
try {
    Assert-NoReparsePointInPath $destinationPath "Backup destination"
    New-Item -ItemType Directory -Path $destinationPath | Out-Null
    Assert-NoReparsePointInPath $destinationPath "Backup destination"

    $backendContainer = (docker compose -f $compose ps -q backend).Trim()
    Assert-LastExitCode "Finding the backend container failed."
    if (-not $backendContainer) {
        throw "The backend container is not running."
    }

    $mounts = docker inspect $backendContainer --format "{{json .Mounts}}" | ConvertFrom-Json
    Assert-LastExitCode "Inspecting backend mounts failed."
    $mediaMount = $mounts | Where-Object { $_.Destination -eq "/app/media" -and $_.Type -eq "volume" } | Select-Object -First 1
    if (-not $mediaMount.Name) {
        throw "The backend media volume could not be resolved."
    }
    $backendImage = (docker inspect $backendContainer --format "{{.Config.Image}}").Trim()
    Assert-LastExitCode "Resolving the backend image failed."

    $statusOutput = @(docker compose -f $compose exec -T backend gvideo data-status)
    Assert-LastExitCode "Reading database statistics failed."
    $databaseStats = Convert-StatusOutput $statusOutput

    try {
        docker compose -f $compose exec -T backend gvideo data-backup $containerPath
        Assert-LastExitCode "Database backup failed."
        docker compose -f $compose cp "backend:$containerPath" $databasePath
        Assert-LastExitCode "Copying the database backup failed."
    } finally {
        docker compose -f $compose exec -T backend rm -f -- $containerPath | Out-Null
    }

    $mediaVolumeMount = "type=volume,source=$($mediaMount.Name),target=/source,readonly"
    Assert-NoReparsePointInPath $destinationPath "Backup bind mount"
    $backupDirectoryMount = "type=bind,source=$destinationPath,target=/backup"
    docker run --rm --entrypoint tar --mount $mediaVolumeMount --mount $backupDirectoryMount $backendImage -czf /backup/media.tar.gz -C /source .
    Assert-LastExitCode "Archiving the media volume failed."

    $archiveEntries = @(tar -tvzf $mediaPath)
    Assert-LastExitCode "Reading the media archive failed."
    $mediaFileCount = @($archiveEntries | Where-Object { $_ -and $_[0] -eq "-" }).Count

    $databaseInfo = Get-Item -LiteralPath $databasePath
    $mediaInfo = Get-Item -LiteralPath $mediaPath
    $manifest = [ordered]@{
        schema_version = 1
        created_at_utc = (Get-Date).ToUniversalTime().ToString("o")
        consistency = "Near-consistent online snapshot: SQLite uses VACUUM INTO, then media is archived from a read-only mount. Run during a quiet write window."
        database = [ordered]@{
            file = $databaseInfo.Name
            bytes = $databaseInfo.Length
            sha256 = (Get-FileHash -Algorithm SHA256 -LiteralPath $databasePath).Hash.ToLowerInvariant()
            statistics = $databaseStats
        }
        media = [ordered]@{
            file = $mediaInfo.Name
            bytes = $mediaInfo.Length
            sha256 = (Get-FileHash -Algorithm SHA256 -LiteralPath $mediaPath).Hash.ToLowerInvariant()
            file_count = $mediaFileCount
            volume = $mediaMount.Name
        }
    }
    $manifest | ConvertTo-Json -Depth 6 | Set-Content -LiteralPath $manifestPath -Encoding UTF8
    Write-Host "Backup bundle saved to $destinationPath"
    Write-Host "Run .\scripts\verify-backup.ps1 -Backup $destinationPath to verify an isolated restore."
} finally { Pop-Location }
