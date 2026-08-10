$ErrorActionPreference = "Stop"
$root = Split-Path -Parent $PSScriptRoot
$stamp = Get-Date -Format "yyyyMMdd-HHmmss"
$source = Join-Path $root "backups\local-before-merge-$stamp.db"
$containerSource = "/app/data/import/local-$stamp.db"
$containerBackup = "/app/data/backups/docker-before-merge-$stamp.db"
$localBackup = Join-Path $root "backups\docker-before-merge-$stamp.db"

Push-Location $root
try {
    New-Item -ItemType Directory -Force -Path (Join-Path $root "backups") | Out-Null
    Push-Location (Join-Path $root "backend")
    try {
        $env:DATABASE_PATH = ".\data\gvideo.db"
        $env:MEDIA_DIR = ".\media"
        go run ./cmd/server data-backup $source
        if ($LASTEXITCODE -ne 0) { throw "Failed to create a consistent local database snapshot." }
    } finally { Pop-Location }
    docker compose up --build -d backend
    if ($LASTEXITCODE -ne 0) { throw "Failed to start the persistent backend." }
    docker compose exec -T backend gvideo data-backup $containerBackup
    if ($LASTEXITCODE -ne 0) { throw "Failed to back up the Docker database." }
    docker compose cp "backend:$containerBackup" $localBackup
    if ($LASTEXITCODE -ne 0) { throw "Failed to copy the Docker database backup." }
    docker compose exec -T backend mkdir -p /app/data/import
    if ($LASTEXITCODE -ne 0) { throw "Failed to prepare the database import directory." }
    docker compose cp $source "backend:$containerSource"
    if ($LASTEXITCODE -ne 0) { throw "Failed to copy the local database into the backend container." }
    docker compose exec -T backend gvideo data-merge $containerSource
    if ($LASTEXITCODE -ne 0) { throw "Merging the local database failed." }
    docker compose cp (Join-Path $root "backend\media\.") "backend:/app/media"
    if ($LASTEXITCODE -ne 0) { throw "Copying local media into the persistent media volume failed." }
    docker compose exec -T backend gvideo data-status
    Write-Host "Docker backup saved to $localBackup"
} finally { Pop-Location }
