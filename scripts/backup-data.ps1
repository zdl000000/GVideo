param(
    [string]$Destination = ""
)

$ErrorActionPreference = "Stop"
$root = Split-Path -Parent $PSScriptRoot
$stamp = Get-Date -Format "yyyyMMdd-HHmmss"
if (-not $Destination) {
    $Destination = Join-Path $root "backups\gvideo-$stamp.db"
}
$destinationPath = [System.IO.Path]::GetFullPath($Destination)
$workspacePath = [System.IO.Path]::GetFullPath($root)
if (-not $destinationPath.StartsWith($workspacePath, [System.StringComparison]::OrdinalIgnoreCase)) {
    throw "Backup destination must stay inside the GVideo workspace."
}
$containerPath = "/app/data/backups/gvideo-$stamp.db"

Push-Location $root
try {
    New-Item -ItemType Directory -Force -Path (Split-Path -Parent $destinationPath) | Out-Null
    docker compose exec -T backend gvideo data-backup $containerPath
    if ($LASTEXITCODE -ne 0) { throw "Database backup failed." }
    docker compose cp "backend:$containerPath" $destinationPath
    if ($LASTEXITCODE -ne 0) { throw "Copying the database backup failed." }
    Write-Host "Backup saved to $destinationPath"
} finally { Pop-Location }
