[CmdletBinding()]
param(
    [Alias("Cleanup")]
    [switch]$RemoveBackupOnSuccess
)

Set-StrictMode -Version 2.0
$ErrorActionPreference = "Stop"

$root = [System.IO.Path]::GetFullPath((Split-Path -Parent $PSScriptRoot))
$backupRoot = [System.IO.Path]::GetFullPath((Join-Path $root "backups"))
$backupScript = Join-Path $PSScriptRoot "backup-data.ps1"
$verifyScript = Join-Path $PSScriptRoot "verify-backup.ps1"
$stamp = (Get-Date).ToUniversalTime().ToString("yyyyMMdd-HHmmss-fff'Z'")
$bundleName = "gvideo-drill-$stamp-$([guid]::NewGuid().ToString('N'))"
$backupPath = [System.IO.Path]::GetFullPath((Join-Path $backupRoot $bundleName))

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

function Assert-IsCreatedDrillBackup([string]$Path) {
    $resolvedPath = [System.IO.Path]::GetFullPath($Path).TrimEnd("\", "/")
    $resolvedBackupRoot = $backupRoot.TrimEnd("\", "/")
    $expectedPath = [System.IO.Path]::GetFullPath((Join-Path $backupRoot $bundleName)).TrimEnd("\", "/")
    if (-not $resolvedPath.Equals($expectedPath, [System.StringComparison]::OrdinalIgnoreCase)) {
        throw "Refusing to operate on an unexpected backup path: $resolvedPath"
    }

    $parent = [System.IO.Directory]::GetParent($resolvedPath)
    if (-not $parent -or -not $parent.FullName.TrimEnd("\", "/").Equals($resolvedBackupRoot, [System.StringComparison]::OrdinalIgnoreCase)) {
        throw "The drill backup must be an immediate child of the workspace backups directory."
    }
    if (-not [System.IO.Path]::GetFileName($resolvedPath).Equals($bundleName, [System.StringComparison]::Ordinal)) {
        throw "The drill backup directory name does not match this run."
    }
}

function Remove-CreatedDrillBackup([string]$Path) {
    Assert-IsCreatedDrillBackup $Path
    Assert-NoReparsePointInPath $Path "Drill backup cleanup"

    $allowedFiles = @("database.db", "media.tar.gz", "manifest.json")
    $items = @(Get-ChildItem -LiteralPath $Path -Force)
    foreach ($item in $items) {
        if ($item.PSIsContainer -or $allowedFiles -notcontains $item.Name) {
            throw "Refusing to clean a drill backup containing an unexpected entry: $($item.FullName)"
        }
        if (($item.Attributes -band [System.IO.FileAttributes]::ReparsePoint) -ne 0) {
            throw "Refusing to clean a drill backup containing a reparse point: $($item.FullName)"
        }
    }

    foreach ($fileName in $allowedFiles) {
        $filePath = Join-Path $Path $fileName
        if (-not (Test-Path -LiteralPath $filePath -PathType Leaf)) {
            throw "Refusing to clean an incomplete drill backup: $filePath"
        }
    }

    foreach ($fileName in $allowedFiles) {
        Remove-Item -LiteralPath (Join-Path $Path $fileName) -Force
    }
    Remove-Item -LiteralPath $Path -Force
}

foreach ($scriptPath in @($backupScript, $verifyScript)) {
    if (-not (Test-Path -LiteralPath $scriptPath -PathType Leaf)) {
        throw "Required script not found: $scriptPath"
    }
    Assert-NoReparsePointInPath $scriptPath "Required script"
}

Assert-IsCreatedDrillBackup $backupPath
Assert-NoReparsePointInPath $backupRoot "Backup root"

try {
    Write-Host "Creating isolated drill backup: $backupPath"
    & $backupScript -Destination $backupPath
    if (-not $?) {
        throw "Backup creation script reported failure."
    }
    if (-not (Test-Path -LiteralPath (Join-Path $backupPath "manifest.json") -PathType Leaf)) {
        throw "Backup creation completed without a manifest."
    }

    Write-Host "Verifying isolated restore from: $backupPath"
    & $verifyScript -Backup $backupPath
    if (-not $?) {
        throw "Backup verification script reported failure."
    }

    if ($RemoveBackupOnSuccess) {
        Remove-CreatedDrillBackup $backupPath
        Write-Host "Backup and restore drill passed; this run's backup was removed: $backupPath"
    } else {
        Write-Host "Backup and restore drill passed; audit backup retained: $backupPath"
    }
} catch {
    [Console]::Error.WriteLine("Backup and restore drill failed: {0}", $_.Exception.Message)
    if (Test-Path -LiteralPath $backupPath) {
        [Console]::Error.WriteLine("This run's backup was preserved for diagnosis: {0}", $backupPath)
    }
    throw
}
