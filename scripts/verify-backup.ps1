param(
    [string]$Backup = ""
)

$ErrorActionPreference = "Stop"
$root = Split-Path -Parent $PSScriptRoot
$compose = Join-Path $root "compose.yaml"
$backupRoot = Join-Path $root "backups"
$temporaryRoot = [System.IO.Path]::GetFullPath((Join-Path $root "tmp"))
$maxManifestBytes = 1MB
$maxDatabaseBytes = 8GB
$maxArchiveBytes = 16GB
$maxArchiveEntries = 100000
$maxExpandedBytes = 32GB
$maxEntryBytes = 2GB
$maxPathLength = 1024
$maxPathDepth = 32
# Windows 自带 bsdtar：正确处理 C:\ 绝对路径；PATH 上的 GNU tar（Git）会把它当远程主机。
$nativeTar = Join-Path $env:SystemRoot "System32\tar.exe"

function Assert-LastExitCode([string]$Message) {
    if ($LASTEXITCODE -ne 0) {
        throw $Message
    }
}

function Assert-SafeManifestFile([string]$Name) {
    if (-not $Name -or [System.IO.Path]::GetFileName($Name) -ne $Name) {
        throw "Manifest contains an unsafe file name: $Name"
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

function Assert-FileSize([string]$Path, [int64]$MaximumBytes, [string]$Purpose) {
    $length = (Get-Item -LiteralPath $Path -Force).Length
    if ($length -gt $MaximumBytes) {
        throw "$Purpose exceeds the $MaximumBytes byte verification limit: $length bytes."
    }
}

function Convert-StatusOutput([string[]]$Lines) {
    $result = @{}
    foreach ($line in $Lines) {
        if ($line -match "^([^=]+)=(.*)$" -and $Matches[1] -notin @("database", "integrity", "media")) {
            $result[$Matches[1]] = [int64]$Matches[2]
        }
    }
    return $result
}

Assert-NoReparsePointInPath $backupRoot "Backup root"
if (-not $Backup) {
    $latest = Get-ChildItem -LiteralPath $backupRoot -Directory -ErrorAction SilentlyContinue |
        Where-Object { Test-Path -LiteralPath (Join-Path $_.FullName "manifest.json") } |
        Sort-Object LastWriteTimeUtc -Descending |
        Select-Object -First 1
    if (-not $latest) {
        throw "No complete backup bundle was found under $backupRoot."
    }
    $Backup = $latest.FullName
}

$backupPath = [System.IO.Path]::GetFullPath($Backup).TrimEnd("\", "/")
$backupPrefix = [System.IO.Path]::GetFullPath($backupRoot).TrimEnd("\", "/") + [System.IO.Path]::DirectorySeparatorChar
if (-not $backupPath.StartsWith($backupPrefix, [System.StringComparison]::OrdinalIgnoreCase)) {
    throw "Backup verification is limited to the workspace backups directory."
}

$manifestPath = Join-Path $backupPath "manifest.json"
if (-not (Test-Path -LiteralPath $manifestPath -PathType Leaf)) {
    throw "Backup manifest not found: $manifestPath"
}
Assert-NoReparsePointInPath $backupPath "Backup bundle"
Assert-NoReparsePointInPath $manifestPath "Backup manifest"
Assert-FileSize $manifestPath $maxManifestBytes "Backup manifest"
$manifest = Get-Content -Raw -Encoding UTF8 -LiteralPath $manifestPath | ConvertFrom-Json
if ($manifest.schema_version -ne 1) {
    throw "Unsupported backup manifest schema: $($manifest.schema_version)"
}

Assert-SafeManifestFile $manifest.database.file
Assert-SafeManifestFile $manifest.media.file
$databasePath = Join-Path $backupPath $manifest.database.file
$mediaPath = Join-Path $backupPath $manifest.media.file
foreach ($file in @($databasePath, $mediaPath)) {
    if (-not (Test-Path -LiteralPath $file -PathType Leaf)) {
        throw "Backup file not found: $file"
    }
    Assert-NoReparsePointInPath $file "Backup file"
}
Assert-FileSize $databasePath $maxDatabaseBytes "Database backup"
Assert-FileSize $mediaPath $maxArchiveBytes "Media archive"

$databaseHash = (Get-FileHash -Algorithm SHA256 -LiteralPath $databasePath).Hash.ToLowerInvariant()
$mediaHash = (Get-FileHash -Algorithm SHA256 -LiteralPath $mediaPath).Hash.ToLowerInvariant()
if ($databaseHash -ne $manifest.database.sha256) {
    throw "Database checksum mismatch."
}
if ($mediaHash -ne $manifest.media.sha256) {
    throw "Media checksum mismatch."
}
if ((Get-Item -LiteralPath $databasePath).Length -ne [int64]$manifest.database.bytes) {
    throw "Database size mismatch."
}
if ((Get-Item -LiteralPath $mediaPath).Length -ne [int64]$manifest.media.bytes) {
    throw "Media archive size mismatch."
}

$verificationPath = Join-Path $temporaryRoot ("restore-verification-" + [guid]::NewGuid().ToString("N"))
$verificationPath = [System.IO.Path]::GetFullPath($verificationPath)
$temporaryPrefix = $temporaryRoot.TrimEnd("\", "/") + [System.IO.Path]::DirectorySeparatorChar
if (-not $verificationPath.StartsWith($temporaryPrefix, [System.StringComparison]::OrdinalIgnoreCase)) {
    throw "Unsafe restore verification path."
}
$databaseDirectory = Join-Path $verificationPath "data"
$mediaDirectory = Join-Path $verificationPath "media"

Push-Location $root
try {
    Assert-NoReparsePointInPath $temporaryRoot "Temporary root"
    Assert-NoReparsePointInPath $verificationPath "Restore verification directory"
    $backendContainer = (docker compose -f $compose ps -q backend).Trim()
    Assert-LastExitCode "Finding the backend container failed."
    if (-not $backendContainer) {
        throw "The backend container is not running."
    }
    $backendImage = (docker inspect $backendContainer --format "{{.Config.Image}}").Trim()
    Assert-LastExitCode "Resolving the backend image failed."

    Assert-NoReparsePointInPath $backupPath "Backup bind mount"
    $sourceMount = "type=bind,source=$backupPath,target=/verify-source,readonly"
    $sourceStatus = @(docker run --rm --entrypoint gvideo --mount $sourceMount `
        -e DATABASE_PATH="/verify-source/$($manifest.database.file)" `
        -e MEDIA_WORKER_ENABLED=false `
        $backendImage data-verify 2>&1)
    if ($LASTEXITCODE -ne 0) {
        throw "Read-only database verification failed:`n$($sourceStatus -join [Environment]::NewLine)"
    }
    $sourceStats = Convert-StatusOutput $sourceStatus
    foreach ($property in $manifest.database.statistics.PSObject.Properties) {
        if ([int64]$sourceStats[$property.Name] -ne [int64]$property.Value) {
            throw "Source database statistic mismatch for $($property.Name)."
        }
    }

    New-Item -ItemType Directory -Path $databaseDirectory, $mediaDirectory -Force | Out-Null
    Assert-NoReparsePointInPath $verificationPath "Restore verification directory"
    Copy-Item -LiteralPath $databasePath -Destination (Join-Path $databaseDirectory "gvideo.db")

    $archiveEntries = @()
    $readAttempts = 3
    for ($readAttempt = 1; $readAttempt -le $readAttempts; $readAttempt++) {
        $archiveEntries = @(& $nativeTar -tvzf $mediaPath)
        if ($LASTEXITCODE -eq 0) { break }
        if ($readAttempt -lt $readAttempts) {
            Start-Sleep -Seconds (3 * $readAttempt)
        }
    }
    Assert-LastExitCode "Reading the media archive failed."
    $archivePaths = @(& $nativeTar -tzf $mediaPath)
    Assert-LastExitCode "Reading media archive paths failed."
    if ($archivePaths.Count -ne $archiveEntries.Count) {
        throw "Media archive path and detail listings do not match."
    }
    if ($archivePaths.Count -gt $maxArchiveEntries) {
        throw "Media archive exceeds the $maxArchiveEntries entry verification limit."
    }
    [int64]$expandedBytes = 0
    for ($index = 0; $index -lt $archivePaths.Count; $index++) {
        $entry = $archivePaths[$index]
        $normalized = $entry.Replace("\", "/")
        if ($normalized.StartsWith("/") -or $normalized -match "^[A-Za-z]:" -or $normalized -match "(^|/)\.\.(/|$)") {
            throw "Media archive contains an unsafe path: $entry"
        }
        if ($normalized.Length -gt $maxPathLength) {
            throw "Media archive path exceeds the $maxPathLength character limit: $entry"
        }
        $relativePath = $normalized -replace "^\./", ""
        $depth = @($relativePath.TrimEnd("/").Split("/", [System.StringSplitOptions]::RemoveEmptyEntries)).Count
        if ($depth -gt $maxPathDepth) {
            throw "Media archive path exceeds the $maxPathDepth level depth limit: $entry"
        }

        $detail = $archiveEntries[$index]
        if (-not $detail -or $detail[0] -notin @("-", "d")) {
            throw "Media archive contains an unsupported entry type: $detail"
        }
        if ($detail -notmatch '^\S+\s+\S+\s+\S+\s+\S+\s+(\d+)\s+') {
            throw "Media archive contains an unreadable entry detail: $detail"
        }
        [int64]$entryBytes = $Matches[1]
        if ($entryBytes -gt $maxEntryBytes) {
            throw "Media archive entry exceeds the $maxEntryBytes byte limit: $entry"
        }
        if ($expandedBytes -gt ($maxExpandedBytes - $entryBytes)) {
            throw "Media archive exceeds the $maxExpandedBytes byte expanded-size limit."
        }
        $expandedBytes += $entryBytes
    }
    & $nativeTar -xzf $mediaPath -C $mediaDirectory
    Assert-LastExitCode "Extracting the media archive failed."

    $mediaFileCount = @(Get-ChildItem -LiteralPath $mediaDirectory -Recurse -File).Count
    if ([int64]$mediaFileCount -ne [int64]$manifest.media.file_count) {
        throw "Media file count mismatch: expected $($manifest.media.file_count), got $mediaFileCount."
    }

    $databaseMount = "type=bind,source=$databaseDirectory,target=/verify-data"
    $mediaMount = "type=bind,source=$mediaDirectory,target=/verify-media,readonly"
    $mediaStatus = @(docker run --rm --entrypoint gvideo --mount "$databaseMount,readonly" --mount $mediaMount `
        -e DATABASE_PATH=/verify-data/gvideo.db `
        -e MEDIA_DIR=/verify-media `
        -e MEDIA_WORKER_ENABLED=false `
        $backendImage data-verify-media 2>&1)
    if ($LASTEXITCODE -ne 0) {
        throw "Verifying restored media references failed:`n$($mediaStatus -join [Environment]::NewLine)"
    }
    $verifiedMediaStats = Convert-StatusOutput $mediaStatus
    foreach ($property in $manifest.database.statistics.PSObject.Properties) {
        if ([int64]$verifiedMediaStats[$property.Name] -ne [int64]$property.Value) {
            throw "Media verification database statistic mismatch for $($property.Name)."
        }
    }

    $statusOutput = @(docker run --rm --entrypoint gvideo --mount $databaseMount --mount $mediaMount `
        -e DATABASE_PATH=/verify-data/gvideo.db `
        -e MEDIA_DIR=/verify-media `
        -e MEDIA_WORKER_ENABLED=false `
        $backendImage data-status 2>&1)
    if ($LASTEXITCODE -ne 0) {
        throw "Opening the restored database copy failed:`n$($statusOutput -join [Environment]::NewLine)"
    }
    $restoredStats = Convert-StatusOutput $statusOutput
    foreach ($property in $manifest.database.statistics.PSObject.Properties) {
        if ([int64]$restoredStats[$property.Name] -ne [int64]$property.Value) {
            throw "Database statistic mismatch for $($property.Name)."
        }
    }

    Write-Host "Backup verified: $backupPath"
    Write-Host "Database integrity, statistics, media checksum, paths, and file count are valid."
} finally {
    Pop-Location
    if (Test-Path -LiteralPath $verificationPath) {
        $resolvedVerificationPath = [System.IO.Path]::GetFullPath($verificationPath)
        if ($resolvedVerificationPath.StartsWith($temporaryPrefix, [System.StringComparison]::OrdinalIgnoreCase)) {
            Assert-NoReparsePointInPath $resolvedVerificationPath "Restore verification cleanup"
            Remove-Item -LiteralPath $resolvedVerificationPath -Recurse -Force
        }
    }
}
